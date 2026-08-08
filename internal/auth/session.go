package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	AdminCookieName = "clist_admin"
	CSRFCookieName  = "clist_csrf"
)

type ActorKind string

const (
	ActorAdmin  ActorKind = "admin"
	ActorGuest  ActorKind = "guest"
	ActorVault  ActorKind = "vault"
	ActorToken  ActorKind = "token"
	ActorPublic ActorKind = "public"
)

type Actor struct {
	Kind   ActorKind
	ID     string
	Scopes map[string]struct{}
}

type Session struct {
	ID        string
	Token     string
	CSRFToken string
	ActorKind ActorKind
	ActorID   string
	Scope     string
	ExpiresAt time.Time
}

type Authenticator struct {
	db *sql.DB
}

type contextKey string

const actorContextKey contextKey = "clist.actor"

func NewAuthenticator(database *sql.DB) *Authenticator {
	return &Authenticator{db: database}
}

func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := a.AdminActor(r)
		if err != nil {
			writeAuthError(w, r, http.StatusUnauthorized, "unauthorized", "需要管理员登录")
			return
		}
		if err := a.verifyCSRF(r); err != nil {
			writeAuthError(w, r, http.StatusForbidden, "csrf_required", "CSRF 校验失败")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
	})
}

func (a *Authenticator) RequireManager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if actor, err := a.BearerActor(r); err == nil {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
			return
		}
		if actor, err := a.AdminActor(r); err == nil {
			if err := a.verifyCSRF(r); err != nil {
				writeAuthError(w, r, http.StatusForbidden, "csrf_required", "CSRF 校验失败")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
			return
		}
		actor, err := a.VaultActor(r)
		if err != nil {
			writeAuthError(w, r, http.StatusUnauthorized, "unauthorized", "需要管理员或恢复密钥")
			return
		}
		if err := a.verifyScopedCSRF(r, VaultCookieName, "vault"); err != nil {
			writeAuthError(w, r, http.StatusForbidden, "csrf_required", "CSRF 校验失败")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, actor)))
	})
}

func (a *Authenticator) BearerActor(r *http.Request) (Actor, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return Actor{}, ErrUnauthorized
	}
	return NewTokenService(a.db).Authenticate(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
}

func (a *Authenticator) VaultActor(r *http.Request) (Actor, error) {
	cookie, err := r.Cookie(VaultCookieName)
	if err != nil {
		return Actor{}, ErrUnauthorized
	}
	var actorID, expiresAt string
	err = a.db.QueryRowContext(r.Context(), `SELECT actor_id,expires_at FROM sessions
WHERE token_hash=? AND kind='vault' AND revoked_at IS NULL`, hashToken(cookie.Value)).Scan(&actorID, &expiresAt)
	if err != nil {
		return Actor{}, ErrUnauthorized
	}
	expires, err := parseSessionTime(expiresAt)
	if err != nil || !expires.After(time.Now().UTC()) {
		return Actor{}, ErrUnauthorized
	}
	return Actor{Kind: ActorVault, ID: actorID, Scopes: map[string]struct{}{"read": {}, "upload": {}, "manage": {}}}, nil
}

func (a *Authenticator) AdminActor(r *http.Request) (Actor, error) {
	cookie, err := r.Cookie(AdminCookieName)
	if err != nil || cookie.Value == "" {
		return Actor{}, ErrUnauthorized
	}
	hash := hashToken(cookie.Value)
	var actorID, csrfHash, expiresAt string
	err = a.db.QueryRowContext(r.Context(), `
SELECT actor_id, csrf_token_hash, expires_at
FROM sessions
WHERE token_hash = ? AND kind = 'admin' AND revoked_at IS NULL`, hash).Scan(&actorID, &csrfHash, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Actor{}, ErrUnauthorized
	}
	if err != nil {
		return Actor{}, err
	}
	expires, err := parseSessionTime(expiresAt)
	if err != nil || !expires.After(time.Now().UTC()) {
		return Actor{}, ErrUnauthorized
	}
	return Actor{Kind: ActorAdmin, ID: actorID, Scopes: map[string]struct{}{"manage": {}}}, nil
}

func (a *Authenticator) Revoke(r *http.Request) error {
	cookie, err := r.Cookie(AdminCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	_, err = a.db.ExecContext(r.Context(),
		"UPDATE sessions SET revoked_at = ? WHERE token_hash = ?",
		time.Now().UTC().Format(time.RFC3339Nano), hashToken(cookie.Value))
	return err
}

func (a *Authenticator) verifyCSRF(r *http.Request) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return nil
	}
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return ErrUnauthorized
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return ErrUnauthorized
	}
	adminCookie, err := r.Cookie(AdminCookieName)
	if err != nil || adminCookie.Value == "" {
		return ErrUnauthorized
	}
	var expected string
	err = a.db.QueryRowContext(r.Context(),
		"SELECT csrf_token_hash FROM sessions WHERE token_hash = ? AND kind = 'admin' AND revoked_at IS NULL",
		hashToken(adminCookie.Value)).Scan(&expected)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(hashToken(header))) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (a *Authenticator) verifyScopedCSRF(r *http.Request, sessionCookie, kind string) error {
	csrfCookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ErrUnauthorized
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" || subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(header)) != 1 {
		return ErrUnauthorized
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ErrUnauthorized
	}
	var expected string
	err = a.db.QueryRowContext(r.Context(), "SELECT csrf_token_hash FROM sessions WHERE token_hash=? AND kind=? AND revoked_at IS NULL", hashToken(cookie.Value), kind).Scan(&expected)
	if err != nil || subtle.ConstantTimeCompare([]byte(expected), []byte(hashToken(header))) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey).(Actor)
	return actor, ok
}

func newSession(ctx context.Context, database *sql.DB, kind ActorKind, actorID, scope string) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	id, err := randomToken(16)
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = database.ExecContext(ctx, `
INSERT INTO sessions(
  id, token_hash, kind, actor_id, scope, csrf_token_hash, created_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		hashToken(token),
		kind,
		actorID,
		nullableSessionScope(scope),
		hashToken(csrfToken),
		time.Now().UTC().Format(time.RFC3339Nano),
		expiresAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Session{}, err
	}
	return Session{ID: id, Token: token, CSRFToken: csrfToken, ActorKind: kind, ActorID: actorID, Scope: scope, ExpiresAt: expiresAt}, nil
}

func randomBytes(dst []byte) error {
	_, err := rand.Read(dst)
	return err
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if err := randomBytes(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func nullableSessionScope(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func parseSessionTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "unavailable"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":       code,
		"message":    message,
		"request_id": requestID,
		"retriable":  false,
	})
}
