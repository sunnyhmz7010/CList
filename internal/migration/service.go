package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/jobs"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

var (
	ErrHashMismatch = errors.New("migration hash mismatch")
	ErrConflict     = errors.New("migration idempotency conflict")
)

type MigrationInput struct {
	PublicID        string     `json:"public_id"`
	TargetProfileID string     `json:"target_profile_id"`
	IdempotencyKey  string     `json:"-"`
	Actor           auth.Actor `json:"-"`
}

type payload struct {
	PublicID        string `json:"public_id"`
	TargetProfileID string `json:"target_profile_id"`
}

type Service struct {
	db       *sql.DB
	dir      string
	registry *storage.Registry
	files    *files.FileService
	repo     *repository.FileRepo
	jobs     *jobs.Store
	idem     *repository.IdempotencyRepo
}

func NewService(database *sql.DB, dir string, registry *storage.Registry, fileService *files.FileService, repo *repository.FileRepo) *Service {
	return &Service{db: database, dir: dir, registry: registry, files: fileService, repo: repo, jobs: jobs.NewStore(database), idem: repository.NewIdempotencyRepo(database)}
}

func (s *Service) Start(ctx context.Context, input MigrationInput) (jobs.Job, error) {
	if input.IdempotencyKey != "" {
		record, err := s.idem.Get(ctx, actorScope(input.Actor), input.IdempotencyKey, "migrations.start")
		if err == nil && record.State == repository.IdempotencyCompleted {
			return s.jobs.Get(ctx, record.ResourceID)
		}
		reserved, err := s.idem.Reserve(ctx, repository.IdempotencyRecord{Scope: actorScope(input.Actor), Key: input.IdempotencyKey, Operation: "migrations.start"})
		if err != nil {
			return jobs.Job{}, err
		}
		if !reserved {
			return jobs.Job{}, ErrConflict
		}
	}
	file, err := s.files.Get(ctx, input.PublicID, input.Actor)
	if err != nil {
		return jobs.Job{}, err
	}
	if input.TargetProfileID == "" || input.TargetProfileID == file.StorageProfileID {
		return jobs.Job{}, files.ErrInvalidInput
	}
	if input.Actor.Kind == auth.ActorToken {
		if _, ok := input.Actor.Scopes["manage"]; !ok {
			return jobs.Job{}, files.ErrForbidden
		}
	}
	if _, err := s.registry.Resolve(input.TargetProfileID); err != nil {
		return jobs.Job{}, err
	}
	job, err := s.jobs.Enqueue(ctx, "storage.migration", payload{PublicID: input.PublicID, TargetProfileID: input.TargetProfileID})
	if err != nil {
		return jobs.Job{}, err
	}
	if input.IdempotencyKey != "" {
		if err := s.idem.Complete(ctx, actorScope(input.Actor), input.IdempotencyKey, "migrations.start", job.ID, 202, nil); err != nil {
			return jobs.Job{}, err
		}
	}
	return job, nil
}

func (s *Service) Run(ctx context.Context, jobID string) error {
	job, err := s.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	var task payload
	if err := json.Unmarshal(job.Payload, &task); err != nil {
		return err
	}
	if err := s.jobs.Begin(ctx, jobID, time.Now().Add(2*time.Hour)); err != nil {
		return err
	}
	file, err := s.repo.Get(ctx, task.PublicID)
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	if file.State != repository.FileStateActive {
		return s.fail(ctx, jobID, files.ErrGone)
	}
	source, err := s.registry.Resolve(file.StorageProfileID)
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	target, err := s.registry.Resolve(task.TargetProfileID)
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return s.fail(ctx, jobID, err)
	}
	temporaryPath := filepath.Join(s.dir, jobID+".part")
	temporary, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	defer os.Remove(temporaryPath)
	reader, err := source.Open(ctx, file.StorageKey, nil)
	if err != nil {
		temporary.Close()
		return s.fail(ctx, jobID, err)
	}
	sourceHash, copyErr := copyAndHash(ctx, reader, temporary, file.SHA256)
	reader.Close()
	closeErr := temporary.Close()
	if copyErr != nil {
		return s.fail(ctx, jobID, copyErr)
	}
	if closeErr != nil {
		return s.fail(ctx, jobID, closeErr)
	}
	temporary, err = os.Open(temporaryPath)
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	object, err := target.Put(ctx, temporary, storage.ObjectMeta{Key: "migrations/" + jobID, FileName: file.FileName, MIMEType: file.MIMEType, Size: file.Size})
	temporary.Close()
	if err != nil {
		return s.fail(ctx, jobID, err)
	}
	if object.SHA256 != sourceHash || object.Size != file.Size {
		_ = target.Delete(ctx, object.Key)
		return s.fail(ctx, jobID, ErrHashMismatch)
	}
	if err := s.repo.SwitchStorage(ctx, file.PublicID, task.TargetProfileID, object.Key); err != nil {
		_ = target.Delete(ctx, object.Key)
		return s.fail(ctx, jobID, err)
	}
	if err := source.Delete(ctx, file.StorageKey); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		_ = s.repo.RecordError(ctx, file.PublicID, err)
		_ = s.jobs.Finish(ctx, jobID, jobs.CleanupPending, 1, object, err)
		return err
	}
	return s.jobs.Finish(ctx, jobID, jobs.Succeeded, 1, object, nil)
}

func (s *Service) GetJob(ctx context.Context, id string) (jobs.Job, error) {
	return s.jobs.Get(ctx, id)
}

func (s *Service) RunPending(ctx context.Context) error {
	if err := s.jobs.RecoverExpired(ctx, time.Now()); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM jobs WHERE kind='storage.migration' AND (state='queued' OR (state='retry_wait' AND (next_attempt_at IS NULL OR next_attempt_at<=?))) ORDER BY created_at,id LIMIT 8`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.Run(ctx, id); err != nil {
			continue
		}
	}
	return nil
}

func (s *Service) fail(ctx context.Context, jobID string, operationErr error) error {
	_ = s.jobs.Finish(ctx, jobID, jobs.Failed, 0, nil, operationErr)
	return operationErr
}

func copyAndHash(ctx context.Context, source io.Reader, destination io.Writer, expected string) (string, error) {
	hash := sha256.New()
	buffer := make([]byte, 256<<10)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		read, err := source.Read(buffer)
		if read > 0 {
			if _, writeErr := destination.Write(buffer[:read]); writeErr != nil {
				return "", writeErr
			}
			_, _ = hash.Write(buffer[:read])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if expected != "" && actual != expected {
		return actual, ErrHashMismatch
	}
	return actual, nil
}

func actorScope(actor auth.Actor) string { return string(actor.Kind) + ":" + actor.ID }
