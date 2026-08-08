package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

type Token struct {
	ID         string     `json:"id"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type TokenService struct{ db *sql.DB }

func NewTokenService(database *sql.DB) *TokenService { return &TokenService{db: database} }

func (s *TokenService) Create(scopes []string, expiresAt *time.Time) (string, Token, error) {
	scopes, err := normalizeScopes(scopes)
	if err != nil {
		return "", Token{}, err
	}
	plainValue, err := randomToken(32)
	if err != nil {
		return "", Token{}, err
	}
	plain := "clist_" + plainValue
	id, err := randomToken(16)
	if err != nil {
		return "", Token{}, err
	}
	created := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO api_tokens(id,token_hash,scopes,expires_at,created_at) VALUES(?,?,?,?,?)`, id, hashAPIToken(plain), strings.Join(scopes, ","), nullableTimeValue(expiresAt), created.Format(time.RFC3339Nano))
	if err != nil {
		return "", Token{}, err
	}
	return plain, Token{ID: id, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: created}, nil
}

func (s *TokenService) Authenticate(ctx context.Context, plain string) (Actor, error) {
	var id, encodedScopes string
	var expiresAt, revokedAt sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT id,scopes,expires_at,revoked_at FROM api_tokens WHERE token_hash=?", hashAPIToken(plain)).Scan(&id, &encodedScopes, &expiresAt, &revokedAt)
	if err != nil || revokedAt.Valid {
		return Actor{}, ErrUnauthorized
	}
	if expiresAt.Valid {
		expires, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil || !expires.After(time.Now().UTC()) {
			return Actor{}, ErrUnauthorized
		}
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), id)
	actor := Actor{Kind: ActorToken, ID: id, Scopes: make(map[string]struct{})}
	for _, scope := range strings.Split(encodedScopes, ",") {
		if scope != "" {
			actor.Scopes[scope] = struct{}{}
		}
	}
	return actor, nil
}

func (s *TokenService) Revoke(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE api_tokens SET revoked_at=? WHERE id=? AND revoked_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (s *TokenService) List(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,scopes,expires_at,last_used_at,revoked_at,created_at FROM api_tokens ORDER BY created_at DESC,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []Token
	for rows.Next() {
		var token Token
		var scopes, created string
		var expires, used, revoked sql.NullString
		if err := rows.Scan(&token.ID, &scopes, &expires, &used, &revoked, &created); err != nil {
			return nil, err
		}
		token.Scopes = strings.Split(scopes, ",")
		token.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		token.ExpiresAt = parseOptionalTime(expires)
		token.LastUsedAt = parseOptionalTime(used)
		token.RevokedAt = parseOptionalTime(revoked)
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func HasScope(actor Actor, scope string) bool {
	_, ok := actor.Scopes[scope]
	return ok
}

func normalizeScopes(scopes []string) ([]string, error) {
	allowed := map[string]struct{}{"upload": {}, "read": {}, "manage": {}, "delete": {}}
	unique := make(map[string]struct{})
	for _, scope := range scopes {
		if _, ok := allowed[scope]; !ok {
			return nil, errors.New("invalid API token scope")
		}
		unique[scope] = struct{}{}
	}
	if len(unique) == 0 {
		return nil, errors.New("API token requires a scope")
	}
	result := make([]string, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

func hashAPIToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}
func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}
