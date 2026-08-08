package auth

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

var (
	ErrAlreadyInitialized = errors.New("admin already initialized")
	ErrNotInitialized     = errors.New("admin is not initialized")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrRateLimited        = errors.New("rate limited")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AdminService struct {
	db       *sql.DB
	mu       sync.Mutex
	failures map[string]failureState
}

type failureState struct {
	Count   int
	Blocked time.Time
}

func NewAdminService(database *sql.DB) *AdminService {
	return &AdminService{db: database, failures: make(map[string]failureState)}
}

func (s *AdminService) IsInitialized(ctx context.Context) (bool, error) {
	var account string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'admin.account'").Scan(&account)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *AdminService) Initialize(ctx context.Context, account, password string) error {
	initialized, err := s.IsInitialized(ctx)
	if err != nil {
		return err
	}
	if initialized {
		return ErrAlreadyInitialized
	}
	if account == "" {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'admin.account'").Scan(&existing)
	if err == nil {
		return ErrAlreadyInitialized
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO settings(key, value, updated_at) VALUES('admin.account', ?, ?), ('admin.password_hash', ?, ?)",
		account, now, hash, now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AdminService) Login(ctx context.Context, account, password string) (Session, error) {
	if s.blocked(account) {
		return Session{}, ErrRateLimited
	}
	var storedAccount, storedHash string
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM settings WHERE key = 'admin.account'").Scan(&storedAccount)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotInitialized
	}
	if err != nil {
		return Session{}, err
	}
	if err := s.db.QueryRowContext(ctx,
		"SELECT value FROM settings WHERE key = 'admin.password_hash'").Scan(&storedHash); err != nil {
		return Session{}, ErrNotInitialized
	}
	validPassword, err := verifyPassword(storedHash, password)
	if err != nil {
		return Session{}, err
	}
	if storedAccount != account || !validPassword {
		s.recordFailure(account)
		return Session{}, ErrUnauthorized
	}
	s.clearFailures(account)
	return newSession(ctx, s.db, ActorAdmin, account, "")
}

func (s *AdminService) ChangePassword(ctx context.Context, oldPassword, newPassword string) error {
	var storedHash string
	if err := s.db.QueryRowContext(ctx,
		"SELECT value FROM settings WHERE key = 'admin.password_hash'").Scan(&storedHash); err != nil {
		return ErrNotInitialized
	}
	valid, err := verifyPassword(storedHash, oldPassword)
	if err != nil {
		return err
	}
	if !valid {
		return ErrUnauthorized
	}
	newHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		"UPDATE settings SET value = ?, updated_at = ? WHERE key = 'admin.password_hash'",
		newHash, now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE kind = 'admin'"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AdminService) blocked(account string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.failures[account]
	return state.Blocked.After(time.Now())
}

func (s *AdminService) recordFailure(account string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.failures[account]
	state.Count++
	if state.Count >= 5 {
		state.Blocked = time.Now().Add(time.Minute)
		state.Count = 0
	}
	s.failures[account] = state
}

func (s *AdminService) clearFailures(account string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, account)
}
