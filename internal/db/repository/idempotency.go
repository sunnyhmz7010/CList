package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type IdempotencyState string

const (
	IdempotencyReserved  IdempotencyState = "reserved"
	IdempotencyCompleted IdempotencyState = "completed"
)

type IdempotencyRecord struct {
	Scope          string
	Key            string
	Operation      string
	State          IdempotencyState
	ResourceID     string
	ResponseStatus int
	ResponseBody   []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      time.Time
}

type IdempotencyRepo struct {
	db *sql.DB
}

func NewIdempotencyRepo(database *sql.DB) *IdempotencyRepo {
	return &IdempotencyRepo{db: database}
}

func (r *IdempotencyRepo) Get(
	ctx context.Context,
	scope, key, operation string,
) (IdempotencyRecord, error) {
	var record IdempotencyRecord
	var resourceID sql.NullString
	var responseStatus sql.NullInt64
	var createdAt, updatedAt, expiresAt string
	err := r.db.QueryRowContext(ctx, `
SELECT scope, key, operation, state, resource_id, response_status, response_body,
       created_at, updated_at, expires_at
FROM idempotency_keys
WHERE scope = ? AND key = ? AND operation = ?`, scope, key, operation).Scan(
		&record.Scope,
		&record.Key,
		&record.Operation,
		&record.State,
		&resourceID,
		&responseStatus,
		&record.ResponseBody,
		&createdAt,
		&updatedAt,
		&expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return IdempotencyRecord{}, ErrNotFound
	}
	if err != nil {
		return IdempotencyRecord{}, err
	}
	record.ResourceID = resourceID.String
	if responseStatus.Valid {
		record.ResponseStatus = int(responseStatus.Int64)
	}
	record.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	record.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	record.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	return record, nil
}

func (r *IdempotencyRepo) Reserve(ctx context.Context, record IdempotencyRecord) (bool, error) {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = now.Add(24 * time.Hour)
	}

	result, err := r.db.ExecContext(ctx, `
INSERT INTO idempotency_keys (
  scope, key, operation, state, created_at, updated_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(scope, key, operation) DO NOTHING`,
		record.Scope,
		record.Key,
		record.Operation,
		IdempotencyReserved,
		formatTime(record.CreatedAt),
		formatTime(record.UpdatedAt),
		formatTime(record.ExpiresAt),
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *IdempotencyRepo) Complete(
	ctx context.Context,
	scope, key, operation, resourceID string,
	responseStatus int,
	responseBody []byte,
) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE idempotency_keys
SET state = ?, resource_id = ?, response_status = ?, response_body = ?, updated_at = ?
WHERE scope = ? AND key = ? AND operation = ? AND state = ?`,
		IdempotencyCompleted,
		nullableString(resourceID),
		responseStatus,
		responseBody,
		formatTime(time.Now().UTC()),
		scope,
		key,
		operation,
		IdempotencyReserved,
	)
	if err != nil {
		return err
	}
	return requireAffected(result)
}
