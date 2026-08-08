package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

type FilePasswordService struct{ db *sql.DB }

func NewFilePasswordService(database *sql.DB) *FilePasswordService {
	return &FilePasswordService{db: database}
}

func (s *FilePasswordService) Set(ctx context.Context, publicID, password string) error {
	if len(password) < 8 {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO file_secrets(file_id,password_hash,salt,created_at,updated_at)
VALUES(?,?,X'',?,?) ON CONFLICT(file_id) DO UPDATE SET password_hash=excluded.password_hash,updated_at=excluded.updated_at`,
		publicID, []byte(hash), now, now)
	return err
}

func (s *FilePasswordService) Clear(ctx context.Context, publicID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM file_secrets WHERE file_id=?", publicID)
	return err
}

func (s *FilePasswordService) Verify(ctx context.Context, publicID, password string) (Session, error) {
	var hash []byte
	err := s.db.QueryRowContext(ctx, "SELECT password_hash FROM file_secrets WHERE file_id=?", publicID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return newSession(ctx, s.db, ActorGuest, "", "file:"+publicID)
	}
	if err != nil {
		return Session{}, err
	}
	valid, err := verifyPassword(string(hash), password)
	if err != nil {
		return Session{}, err
	}
	if !valid {
		return Session{}, ErrUnauthorized
	}
	return newSession(ctx, s.db, ActorGuest, "", "file:"+publicID)
}

func (s *FilePasswordService) AuthorizeRequest(r *http.Request, publicID string, admin *Authenticator) error {
	if _, err := admin.AdminActor(r); err == nil {
		return nil
	}
	var required int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM file_secrets WHERE file_id=?", publicID).Scan(&required); err != nil {
		return err
	}
	if required == 0 {
		return nil
	}
	cookie, err := r.Cookie(FileCookieName(publicID))
	if err != nil {
		return ErrScopeRequired
	}
	var expiresAt string
	err = s.db.QueryRowContext(r.Context(), `SELECT expires_at FROM sessions
WHERE token_hash=? AND kind='guest' AND scope=? AND revoked_at IS NULL`, hashToken(cookie.Value), "file:"+publicID).Scan(&expiresAt)
	if err != nil {
		return ErrScopeRequired
	}
	expires, err := parseSessionTime(expiresAt)
	if err != nil || !expires.After(time.Now().UTC()) {
		return ErrScopeRequired
	}
	return nil
}

func FileCookieName(publicID string) string { return "clist_file_" + publicID }
