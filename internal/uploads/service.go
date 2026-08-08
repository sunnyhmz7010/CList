package uploads

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrChunkHash      = errors.New("chunk hash mismatch")
	ErrFileHash       = errors.New("file hash mismatch")
	ErrMissingChunks  = errors.New("upload has missing chunks")
	ErrUploadConflict = errors.New("upload conflict")
	ErrChunkTooLarge  = errors.New("chunk is too large")
)

type InitInput struct {
	FileName         string
	MIMEType         string
	TotalSize        int64
	ChunkSize        int64
	TotalChunks      int
	SHA256           string
	FolderID         string
	StorageProfileID string
	IdempotencyKey   string
	Actor            auth.Actor
}

type Upload struct {
	ID               string `json:"id"`
	FileName         string `json:"file_name"`
	MIMEType         string `json:"mime_type"`
	TotalSize        int64  `json:"total_size"`
	ChunkSize        int64  `json:"chunk_size"`
	TotalChunks      int    `json:"total_chunks"`
	SHA256           string `json:"sha256"`
	FolderID         string `json:"folder_id,omitempty"`
	StorageProfileID string `json:"storage_profile_id"`
	State            string `json:"state"`
	MissingChunks    []int  `json:"missing_chunks"`
}

type Service struct {
	db       *sql.DB
	dir      string
	registry *storage.Registry
	files    *files.FileService
	idem     *repository.IdempotencyRepo
	maxChunk int64
}

func NewService(database *sql.DB, dir string, registry *storage.Registry, fileService *files.FileService) *Service {
	return &Service{
		db: database, dir: dir, registry: registry, files: fileService,
		idem: repository.NewIdempotencyRepo(database), maxChunk: 64 << 20,
	}
}

func (s *Service) Init(ctx context.Context, input InitInput) (Upload, error) {
	if input.TotalSize < 0 || input.ChunkSize <= 0 || input.ChunkSize > s.maxChunk || input.TotalChunks <= 0 || input.SHA256 == "" {
		return Upload{}, files.ErrInvalidInput
	}
	if input.StorageProfileID == "" {
		input.StorageProfileID = "local-default"
	}
	if input.IdempotencyKey != "" {
		existing, err := s.idem.Get(ctx, actorScope(input.Actor), input.IdempotencyKey, "uploads.init")
		if err == nil && existing.State == repository.IdempotencyCompleted {
			return s.Get(ctx, existing.ResourceID, input.Actor)
		}
		if err == nil {
			return Upload{}, ErrUploadConflict
		}
		reserved, err := s.idem.Reserve(ctx, repository.IdempotencyRecord{
			Scope: actorScope(input.Actor), Key: input.IdempotencyKey, Operation: "uploads.init",
		})
		if err != nil || !reserved {
			if err != nil {
				return Upload{}, err
			}
			return Upload{}, ErrUploadConflict
		}
	}
	id, err := randomID()
	if err != nil {
		return Upload{}, err
	}
	owner := ""
	if input.Actor.Kind == auth.ActorVault || input.Actor.Kind == auth.ActorGuest {
		owner = input.Actor.ID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO upload_sessions(
  id, owner_vault_id, storage_profile_id, folder_id, file_name, mime_type,
  total_size, chunk_size, total_chunks, sha256, state, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'uploading', ?, ?)`,
		id, nullable(owner), input.StorageProfileID, nullable(input.FolderID), input.FileName, input.MIMEType,
		input.TotalSize, input.ChunkSize, input.TotalChunks, input.SHA256, now, now)
	if err != nil {
		return Upload{}, err
	}
	if err := os.MkdirAll(filepath.Join(s.dir, id), 0o750); err != nil {
		return Upload{}, err
	}
	if input.IdempotencyKey != "" {
		if err := s.idem.Complete(ctx, actorScope(input.Actor), input.IdempotencyKey, "uploads.init", id, 201, nil); err != nil {
			return Upload{}, err
		}
	}
	return s.Get(ctx, id, input.Actor)
}

func (s *Service) PutChunk(ctx context.Context, uploadID string, index int, reader io.Reader, wantSHA256 string, actor auth.Actor) error {
	upload, owner, err := s.load(ctx, uploadID)
	if err != nil {
		return err
	}
	if err := authorize(actor, owner); err != nil {
		return err
	}
	if upload.State != "uploading" || index < 0 || index >= upload.TotalChunks {
		return ErrUploadConflict
	}
	directory := filepath.Join(s.dir, uploadID)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	finalPath := filepath.Join(directory, strconv.Itoa(index)+".chunk")
	temporary, err := os.CreateTemp(directory, ".chunk-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(reader, s.maxChunk+1))
	if err != nil {
		_ = temporary.Close()
		return err
	}
	if written > s.maxChunk {
		_ = temporary.Close()
		return ErrChunkTooLarge
	}
	gotSHA256 := hex.EncodeToString(hash.Sum(nil))
	if wantSHA256 == "" || gotSHA256 != wantSHA256 {
		_ = temporary.Close()
		return ErrChunkHash
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO upload_chunks(upload_id, chunk_index, size, sha256, path, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(upload_id, chunk_index) DO UPDATE SET
  size = excluded.size, sha256 = excluded.sha256, path = excluded.path, created_at = excluded.created_at`,
		uploadID, index, written, gotSHA256, finalPath, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Service) Get(ctx context.Context, uploadID string, actor auth.Actor) (Upload, error) {
	upload, owner, err := s.load(ctx, uploadID)
	if err != nil {
		return Upload{}, err
	}
	if err := authorize(actor, owner); err != nil {
		return Upload{}, err
	}
	upload.MissingChunks, err = s.MissingChunks(ctx, uploadID, actor)
	return upload, err
}

func (s *Service) MissingChunks(ctx context.Context, uploadID string, actor auth.Actor) ([]int, error) {
	upload, owner, err := s.load(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if err := authorize(actor, owner); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT chunk_index, path FROM upload_chunks WHERE upload_id = ?", uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	present := make(map[int]struct{}, upload.TotalChunks)
	for rows.Next() {
		var index int
		var path string
		if err := rows.Scan(&index, &path); err != nil {
			return nil, err
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			present[index] = struct{}{}
		}
	}
	missing := make([]int, 0)
	for index := 0; index < upload.TotalChunks; index++ {
		if _, ok := present[index]; !ok {
			missing = append(missing, index)
		}
	}
	return missing, rows.Err()
}

func (s *Service) Complete(ctx context.Context, uploadID string, actor auth.Actor, idempotencyKey string) (repository.File, error) {
	if idempotencyKey != "" {
		existing, err := s.idem.Get(ctx, actorScope(actor), idempotencyKey, "uploads.complete")
		if err == nil && existing.State == repository.IdempotencyCompleted {
			return s.files.Get(ctx, existing.ResourceID, actor)
		}
		if err == nil {
			return repository.File{}, ErrUploadConflict
		}
		reserved, err := s.idem.Reserve(ctx, repository.IdempotencyRecord{Scope: actorScope(actor), Key: idempotencyKey, Operation: "uploads.complete"})
		if err != nil || !reserved {
			if err != nil {
				return repository.File{}, err
			}
			return repository.File{}, ErrUploadConflict
		}
	}
	upload, owner, err := s.load(ctx, uploadID)
	if err != nil {
		return repository.File{}, err
	}
	if err := authorize(actor, owner); err != nil {
		return repository.File{}, err
	}
	missing, err := s.MissingChunks(ctx, uploadID, actor)
	if err != nil {
		return repository.File{}, err
	}
	if len(missing) != 0 {
		return repository.File{}, ErrMissingChunks
	}
	backend, err := s.registry.Resolve(upload.StorageProfileID)
	if err != nil {
		return repository.File{}, err
	}
	reader := &chunkSequence{dir: filepath.Join(s.dir, uploadID), total: upload.TotalChunks}
	defer reader.Close()
	object, err := backend.Put(ctx, reader, storage.ObjectMeta{
		Key: "objects/" + uploadID, FileName: upload.FileName, MIMEType: upload.MIMEType, Size: upload.TotalSize,
	})
	if err != nil {
		return repository.File{}, err
	}
	if object.Size != upload.TotalSize || object.SHA256 != upload.SHA256 {
		_ = backend.Delete(ctx, object.Key)
		return repository.File{}, ErrFileHash
	}
	file, err := s.files.CreateIndex(ctx, files.CreateFileInput{
		FileName: upload.FileName, MIMEType: upload.MIMEType, Size: object.Size, SHA256: object.SHA256,
		FolderID: upload.FolderID, OwnerVaultID: owner, StorageProfileID: upload.StorageProfileID, StorageKey: object.Key,
	}, actor)
	if err != nil {
		_ = backend.Delete(ctx, object.Key)
		return repository.File{}, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE upload_sessions SET state = 'completed', file_public_id = ?, updated_at = ? WHERE id = ?",
		file.PublicID, time.Now().UTC().Format(time.RFC3339Nano), uploadID)
	if err != nil {
		return repository.File{}, err
	}
	if err := os.RemoveAll(filepath.Join(s.dir, uploadID)); err != nil {
		return repository.File{}, err
	}
	if idempotencyKey != "" {
		if err := s.idem.Complete(ctx, actorScope(actor), idempotencyKey, "uploads.complete", file.PublicID, 200, nil); err != nil {
			return repository.File{}, err
		}
	}
	return file, nil
}

func (s *Service) Abort(ctx context.Context, uploadID string, actor auth.Actor) error {
	_, owner, err := s.load(ctx, uploadID)
	if err != nil {
		return err
	}
	if err := authorize(actor, owner); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE upload_sessions SET state = 'aborted', updated_at = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339Nano), uploadID); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.dir, uploadID))
}

func (s *Service) load(ctx context.Context, uploadID string) (Upload, string, error) {
	var upload Upload
	var owner, folder sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT id, owner_vault_id, storage_profile_id, folder_id, file_name, mime_type,
       total_size, chunk_size, total_chunks, sha256, state
FROM upload_sessions WHERE id = ?`, uploadID).Scan(
		&upload.ID, &owner, &upload.StorageProfileID, &folder, &upload.FileName, &upload.MIMEType,
		&upload.TotalSize, &upload.ChunkSize, &upload.TotalChunks, &upload.SHA256, &upload.State)
	if errors.Is(err, sql.ErrNoRows) {
		return Upload{}, "", ErrUploadNotFound
	}
	upload.FolderID = folder.String
	return upload, owner.String, err
}

func authorize(actor auth.Actor, owner string) error {
	if actor.Kind == auth.ActorAdmin || actor.Kind == auth.ActorToken {
		return nil
	}
	if (actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest) && actor.ID == owner {
		return nil
	}
	return files.ErrForbidden
}

func actorScope(actor auth.Actor) string {
	return string(actor.Kind) + ":" + actor.ID
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

type chunkSequence struct {
	dir     string
	total   int
	index   int
	current *os.File
}

func (r *chunkSequence) Read(p []byte) (int, error) {
	for {
		if r.current == nil {
			if r.index >= r.total {
				return 0, io.EOF
			}
			file, err := os.Open(filepath.Join(r.dir, strconv.Itoa(r.index)+".chunk"))
			if err != nil {
				return 0, err
			}
			r.current = file
			r.index++
		}
		read, err := r.current.Read(p)
		if errors.Is(err, io.EOF) {
			_ = r.current.Close()
			r.current = nil
			if read > 0 {
				return read, nil
			}
			continue
		}
		return read, err
	}
}

func (r *chunkSequence) Close() error {
	if r.current != nil {
		return r.current.Close()
	}
	return nil
}
