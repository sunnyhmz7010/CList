package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type FileState string

const (
	FileStateUploading FileState = "uploading"
	FileStateActive    FileState = "active"
	FileStateTrashed   FileState = "trashed"
	FileStatePurged    FileState = "purged"
)

type Visibility string

const (
	VisibilityInherit Visibility = "inherit"
	VisibilityVisible Visibility = "visible"
	VisibilityHidden  Visibility = "hidden"
)

type File struct {
	PublicID          string
	FolderID          string
	OwnerVaultID      string
	StorageProfileID  string
	StorageKey        string
	FileName          string
	MIMEType          string
	Size              int64
	SHA256            string
	GalleryVisibility Visibility
	State             FileState
	PurgedAt          *time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FileListOptions struct {
	State  FileState
	Cursor string
	Limit  int
}

type FileRepo struct {
	db *sql.DB
}

func NewFileRepo(database *sql.DB) *FileRepo {
	return &FileRepo{db: database}
}

func (r *FileRepo) Create(ctx context.Context, file File) error {
	now := time.Now().UTC()
	if file.CreatedAt.IsZero() {
		file.CreatedAt = now
	}
	if file.UpdatedAt.IsZero() {
		file.UpdatedAt = file.CreatedAt
	}
	if file.GalleryVisibility == "" {
		file.GalleryVisibility = VisibilityInherit
	}
	if file.State == "" {
		file.State = FileStateUploading
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO files (
  public_id, folder_id, owner_vault_id, storage_profile_id, storage_key,
  file_name, mime_type, size, sha256, gallery_visibility, state,
  purged_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.PublicID,
		nullableString(file.FolderID),
		nullableString(file.OwnerVaultID),
		file.StorageProfileID,
		nullableString(file.StorageKey),
		file.FileName,
		file.MIMEType,
		file.Size,
		file.SHA256,
		file.GalleryVisibility,
		file.State,
		nullableTime(file.PurgedAt),
		nullableString(file.LastError),
		formatTime(file.CreatedAt),
		formatTime(file.UpdatedAt),
	)
	return err
}

func (r *FileRepo) Get(ctx context.Context, publicID string) (File, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT public_id, folder_id, owner_vault_id, storage_profile_id, storage_key,
       file_name, mime_type, size, sha256, gallery_visibility, state,
       purged_at, last_error, created_at, updated_at
FROM files WHERE public_id = ?`, publicID)
	file, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return file, err
}

func (r *FileRepo) List(ctx context.Context, options FileListOptions) ([]File, error) {
	limit := normalizeLimit(options.Limit)
	rows, err := r.db.QueryContext(ctx, `
SELECT public_id, folder_id, owner_vault_id, storage_profile_id, storage_key,
       file_name, mime_type, size, sha256, gallery_visibility, state,
       purged_at, last_error, created_at, updated_at
FROM files
WHERE (? = '' OR state = ?) AND (? = '' OR public_id > ?)
ORDER BY public_id
LIMIT ?`, options.State, options.State, options.Cursor, options.Cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]File, 0, limit)
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (r *FileRepo) UpdateState(ctx context.Context, publicID string, state FileState) error {
	result, err := r.db.ExecContext(
		ctx,
		"UPDATE files SET state = ?, updated_at = ? WHERE public_id = ?",
		state,
		formatTime(time.Now().UTC()),
		publicID,
	)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(scanner rowScanner) (File, error) {
	var file File
	var folderID, ownerVaultID, storageKey, purgedAt, lastError sql.NullString
	var createdAt, updatedAt string
	if err := scanner.Scan(
		&file.PublicID,
		&folderID,
		&ownerVaultID,
		&file.StorageProfileID,
		&storageKey,
		&file.FileName,
		&file.MIMEType,
		&file.Size,
		&file.SHA256,
		&file.GalleryVisibility,
		&file.State,
		&purgedAt,
		&lastError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return File{}, err
	}
	file.FolderID = folderID.String
	file.OwnerVaultID = ownerVaultID.String
	file.StorageKey = storageKey.String
	file.LastError = lastError.String

	var err error
	file.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return File{}, err
	}
	file.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return File{}, err
	}
	if purgedAt.Valid {
		value, err := parseTime(purgedAt.String)
		if err != nil {
			return File{}, err
		}
		file.PurgedAt = &value
	}
	return file, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid timestamp")
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
