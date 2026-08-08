package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SettingsRepo struct {
	db *sql.DB
}

func NewSettingsRepo(database *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: database}
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key,
		value,
		formatTime(time.Now().UTC()),
	)
	return err
}

func (r *SettingsRepo) Delete(ctx context.Context, key string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return err
	}
	return requireAffected(result)
}
