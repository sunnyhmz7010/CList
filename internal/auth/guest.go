package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

var ErrScopeRequired = errors.New("scope required")

type Scope string

const (
	ScopeHome    Scope = "home"
	ScopeGallery Scope = "gallery"
)

type GuestService struct{ db *sql.DB }

func NewGuestService(database *sql.DB) *GuestService { return &GuestService{db: database} }

func (s *GuestService) SetHomePassword(ctx context.Context, password string) error {
	return s.setPassword(ctx, ScopeHome, password)
}
func (s *GuestService) SetGalleryPassword(ctx context.Context, password string) error {
	return s.setPassword(ctx, ScopeGallery, password)
}
func (s *GuestService) ClearPassword(ctx context.Context, scope Scope) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM settings WHERE key = ?", guestPasswordKey(scope))
	return err
}

func (s *GuestService) PasswordEnabled(ctx context.Context, scope Scope) (bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", guestPasswordKey(scope)).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *GuestService) Login(ctx context.Context, scope Scope, password string) (Session, error) {
	var encoded string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", guestPasswordKey(scope)).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return newSession(ctx, s.db, ActorGuest, "", string(scope))
	}
	if err != nil {
		return Session{}, err
	}
	valid, err := verifyPassword(encoded, password)
	if err != nil {
		return Session{}, err
	}
	if !valid {
		return Session{}, ErrUnauthorized
	}
	return newSession(ctx, s.db, ActorGuest, "", string(scope))
}

func (s *GuestService) setPassword(ctx context.Context, scope Scope, password string) error {
	if len(password) < 8 {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES(?,?,?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`,
		guestPasswordKey(scope), hash, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func guestPasswordKey(scope Scope) string { return "guest." + string(scope) + ".password_hash" }
func GuestCookieName(scope Scope) string  { return "clist_guest_" + string(scope) }

type GuestAuthenticator struct {
	db    *sql.DB
	admin *Authenticator
}

func NewGuestAuthenticator(database *sql.DB, admin *Authenticator) *GuestAuthenticator {
	return &GuestAuthenticator{db: database, admin: admin}
}

func (a *GuestAuthenticator) RequireScope(scope Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor, err := a.admin.AdminActor(r); err == nil {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
				return
			}
			actor, err := a.scopeActor(r, scope)
			if err != nil {
				writeAuthError(w, r, http.StatusUnauthorized, "scope_required", "需要访问密码")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
		})
	}
}

func (a *GuestAuthenticator) scopeActor(r *http.Request, scope Scope) (Actor, error) {
	cookie, err := r.Cookie(GuestCookieName(scope))
	if err != nil {
		return Actor{}, ErrScopeRequired
	}
	var expiresAt string
	err = a.db.QueryRowContext(r.Context(), `SELECT expires_at FROM sessions
WHERE token_hash=? AND kind='guest' AND scope=? AND revoked_at IS NULL`, hashToken(cookie.Value), scope).Scan(&expiresAt)
	if err != nil {
		return Actor{}, ErrScopeRequired
	}
	expires, err := parseSessionTime(expiresAt)
	if err != nil || !expires.After(time.Now().UTC()) {
		return Actor{}, ErrScopeRequired
	}
	return Actor{Kind: ActorGuest, Scopes: map[string]struct{}{string(scope): {}}}, nil
}

func RequireScope(actor Actor, scope Scope) error {
	if actor.Kind == ActorAdmin {
		return nil
	}
	if _, ok := actor.Scopes[string(scope)]; !ok {
		return ErrScopeRequired
	}
	return nil
}

func RequireFile(actor Actor, publicID string) error {
	if actor.Kind == ActorAdmin {
		return nil
	}
	if _, ok := actor.Scopes["file:"+publicID]; !ok {
		return ErrScopeRequired
	}
	return nil
}
