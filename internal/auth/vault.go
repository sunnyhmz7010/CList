package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

const VaultCookieName = "clist_vault"

type GuestVault struct {
	ID        string     `json:"id"`
	KeyHash   string     `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type VaultService struct{ db *sql.DB }

func NewVaultService(database *sql.DB) *VaultService { return &VaultService{db: database} }

func (s *VaultService) Create(ctx context.Context) (GuestVault, string, error) {
	key, err := randomToken(32)
	if err != nil {
		return GuestVault{}, "", err
	}
	id, err := randomToken(16)
	if err != nil {
		return GuestVault{}, "", err
	}
	created := time.Now().UTC()
	hash := vaultHash(key)
	_, err = s.db.ExecContext(ctx, "INSERT INTO guest_vaults(id,key_hash,created_at) VALUES(?,?,?)", id, hash, created.Format(time.RFC3339Nano))
	if err != nil {
		return GuestVault{}, "", err
	}
	return GuestVault{ID: id, KeyHash: hash, CreatedAt: created}, key, nil
}

func (s *VaultService) Recover(ctx context.Context, key string) (GuestVault, Session, error) {
	var vault GuestVault
	var createdAt string
	err := s.db.QueryRowContext(ctx, "SELECT id,key_hash,created_at FROM guest_vaults WHERE key_hash=? AND revoked_at IS NULL", vaultHash(key)).Scan(&vault.ID, &vault.KeyHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GuestVault{}, Session{}, ErrUnauthorized
	}
	if err != nil {
		return GuestVault{}, Session{}, err
	}
	vault.CreatedAt, err = parseSessionTime(createdAt)
	if err != nil {
		return GuestVault{}, Session{}, err
	}
	session, err := newSession(ctx, s.db, ActorVault, vault.ID, "vault")
	return vault, session, err
}

func (s *VaultService) Revoke(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE guest_vaults SET revoked_at=? WHERE id=? AND revoked_at IS NULL", now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE kind='vault' AND actor_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func vaultHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
