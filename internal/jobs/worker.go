package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	mathrand "math/rand"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("job not found")
	ErrLeaseLost = errors.New("job lease lost")
	ErrUncertain = errors.New("operation result is uncertain")
)

type State string

const (
	Queued         State = "queued"
	Running        State = "running"
	RetryWait      State = "retry_wait"
	Succeeded      State = "succeeded"
	Failed         State = "failed"
	CleanupPending State = "cleanup_pending"
	Uncertain      State = "uncertain"
)

type Job struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	State         State           `json:"state"`
	Progress      float64         `json:"progress"`
	Attempts      int             `json:"attempts"`
	LeaseUntil    *time.Time      `json:"lease_until,omitempty"`
	NextAttemptAt *time.Time      `json:"next_attempt_at,omitempty"`
	LastError     string          `json:"last_error,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(database *sql.DB) *Store {
	return &Store{db: database}
}

func (s *Store) Enqueue(ctx context.Context, kind string, payload any) (Job, error) {
	id, err := randomID()
	if err != nil {
		return Job{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Job{}, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO jobs(id,kind,state,payload,created_at,updated_at) VALUES(?,?,?, ?,?,?)`,
		id, kind, Queued, raw, formatTime(now), formatTime(now))
	if err != nil {
		return Job{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Begin(ctx context.Context, id string, leaseUntil time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET state=?,attempts=attempts+1,lease_until=?,updated_at=? WHERE id=? AND state IN (?,?)`,
		Running, formatTime(leaseUntil), formatTime(time.Now()), id, Queued, RetryWait)
	return requireAffected(result, err)
}

func (s *Store) Lease(ctx context.Context, now time.Time, duration time.Duration) (Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM jobs WHERE state=? OR (state=? AND (next_attempt_at IS NULL OR next_attempt_at<=?)) ORDER BY created_at,id LIMIT 1`,
		Queued, RetryWait, formatTime(now)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET state=?,attempts=attempts+1,lease_until=?,updated_at=? WHERE id=? AND state IN (?,?)`,
		Running, formatTime(now.Add(duration)), formatTime(now), id, Queued, RetryWait)
	if err != nil {
		return Job{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return Job{}, ErrLeaseLost
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Heartbeat(ctx context.Context, id string, leaseUntil time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET lease_until=?,updated_at=? WHERE id=? AND state=?`,
		formatTime(leaseUntil), formatTime(time.Now()), id, Running)
	return requireAffected(result, err)
}

func (s *Store) Finish(ctx context.Context, id string, state State, progress float64, result any, operationErr error) error {
	if state != Succeeded && state != Failed && state != CleanupPending && state != Uncertain && state != RetryWait {
		return errors.New("invalid terminal job state")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	lastError := ""
	if operationErr != nil {
		lastError = operationErr.Error()
	}
	var next any
	if state == RetryWait {
		job, getErr := s.Get(ctx, id)
		if getErr != nil {
			return getErr
		}
		next = formatTime(time.Now().Add(backoff(job.Attempts, time.Second)))
	}
	update, err := s.db.ExecContext(ctx, `UPDATE jobs SET state=?,progress=?,lease_until=NULL,next_attempt_at=?,last_error=?,result=?,updated_at=? WHERE id=?`,
		state, math.Max(0, math.Min(1, progress)), next, nullable(lastError), raw, formatTime(time.Now()), id)
	return requireAffected(update, err)
}

func (s *Store) RecoverExpired(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET state=?,lease_until=NULL,next_attempt_at=?,last_error=?,updated_at=? WHERE state=? AND lease_until<?`,
		RetryWait, formatTime(now), "任务租约过期，等待恢复", formatTime(now), Running, formatTime(now))
	return err
}

func (s *Store) Requeue(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET state=?,lease_until=NULL,next_attempt_at=?,last_error=NULL,updated_at=? WHERE id=? AND state=?`,
		RetryWait, formatTime(now), formatTime(now), id, Uncertain)
	return requireAffected(result, err)
}

func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	var job Job
	var lease, next, last sql.NullString
	var payload, result []byte
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT id,kind,state,progress,attempts,lease_until,next_attempt_at,last_error,payload,result,created_at,updated_at FROM jobs WHERE id=?`, id).Scan(
		&job.ID, &job.Kind, &job.State, &job.Progress, &job.Attempts, &lease, &next, &last, &payload, &result, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	job.Payload, job.Result, job.LastError = payload, result, last.String
	job.CreatedAt, err = parseTime(created)
	if err != nil {
		return Job{}, err
	}
	job.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return Job{}, err
	}
	if lease.Valid {
		value, parseErr := parseTime(lease.String)
		if parseErr != nil {
			return Job{}, parseErr
		}
		job.LeaseUntil = &value
	}
	if next.Valid {
		value, parseErr := parseTime(next.String)
		if parseErr != nil {
			return Job{}, parseErr
		}
		job.NextAttemptAt = &value
	}
	return job, nil
}

type Handler func(context.Context, Job) (any, error)

type Worker struct {
	Store         *Store
	Handlers      map[string]Handler
	LeaseDuration time.Duration
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if err := w.Store.RecoverExpired(ctx, time.Now()); err != nil {
		return err
	}
	duration := w.LeaseDuration
	if duration <= 0 {
		duration = 30 * time.Second
	}
	job, err := w.Store.Lease(ctx, time.Now(), duration)
	if err != nil {
		return err
	}
	handler := w.Handlers[job.Kind]
	if handler == nil {
		return w.Store.Finish(ctx, job.ID, Failed, job.Progress, nil, errors.New("未注册任务处理器"))
	}
	done := make(chan struct{})
	heartbeatErrors := make(chan error, 1)
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		interval := duration / 3
		if interval <= 0 {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				if err := w.Store.Heartbeat(ctx, job.ID, now.Add(duration)); err != nil {
					select {
					case heartbeatErrors <- err:
					default:
					}
					return
				}
			}
		}
	}()
	result, runErr := handler(ctx, job)
	close(done)
	heartbeat.Wait()
	select {
	case heartbeatErr := <-heartbeatErrors:
		if runErr == nil {
			runErr = heartbeatErr
		}
	default:
	}
	if runErr != nil {
		return w.Store.Finish(ctx, job.ID, RetryWait, job.Progress, result, runErr)
	}
	return w.Store.Finish(ctx, job.ID, Succeeded, 1, result, nil)
}

func backoff(attempt int, seed time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 6 {
		attempt = 6
	}
	maximum := seed << attempt
	if maximum <= 1 {
		return maximum
	}
	return maximum/2 + time.Duration(mathrand.Int63n(int64(maximum/2)))
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrLeaseLost
	}
	return nil
}
