package repository

import (
	"context"
	"database/sql"
	"time"
)

type FolderState string

const (
	FolderStateActive  FolderState = "active"
	FolderStateTrashed FolderState = "trashed"
	FolderStatePurged  FolderState = "purged"
)

type Folder struct {
	ID                string
	ParentID          string
	Name              string
	OwnerVaultID      string
	GalleryVisibility Visibility
	State             FolderState
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FolderListOptions struct {
	ParentID string
	Cursor   string
	Limit    int
	State    FolderState
}

type FolderRepo struct {
	db *sql.DB
}

func NewFolderRepo(database *sql.DB) *FolderRepo {
	return &FolderRepo{db: database}
}

func (r *FolderRepo) Create(ctx context.Context, folder Folder) error {
	now := time.Now().UTC()
	if folder.CreatedAt.IsZero() {
		folder.CreatedAt = now
	}
	if folder.UpdatedAt.IsZero() {
		folder.UpdatedAt = folder.CreatedAt
	}
	if folder.GalleryVisibility == "" {
		folder.GalleryVisibility = VisibilityInherit
	}
	if folder.State == "" {
		folder.State = FolderStateActive
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO folders (
  id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		folder.ID,
		nullableString(folder.ParentID),
		folder.Name,
		nullableString(folder.OwnerVaultID),
		folder.GalleryVisibility,
		folder.State,
		formatTime(folder.CreatedAt),
		formatTime(folder.UpdatedAt),
	)
	return err
}

func (r *FolderRepo) List(ctx context.Context, options FolderListOptions) ([]Folder, error) {
	if options.State == "" {
		options.State = FolderStateActive
	}
	limit := normalizeLimit(options.Limit)
	var rows *sql.Rows
	var err error
	if options.ParentID == "" {
		rows, err = r.db.QueryContext(ctx, `
SELECT id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
FROM folders
WHERE parent_id IS NULL AND state = ? AND (? = '' OR id > ?)
ORDER BY id
LIMIT ?`, options.State, options.Cursor, options.Cursor, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
SELECT id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
FROM folders
WHERE parent_id = ? AND state = ? AND (? = '' OR id > ?)
ORDER BY id
LIMIT ?`, options.ParentID, options.State, options.Cursor, options.Cursor, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]Folder, 0, limit)
	for rows.Next() {
		folder, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}

func (r *FolderRepo) Move(ctx context.Context, id, parentID string) error {
	result, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET parent_id = ?, updated_at = ? WHERE id = ?",
		nullableString(parentID),
		formatTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func scanFolder(scanner rowScanner) (Folder, error) {
	var folder Folder
	var parentID, ownerVaultID sql.NullString
	var createdAt, updatedAt string
	if err := scanner.Scan(
		&folder.ID,
		&parentID,
		&folder.Name,
		&ownerVaultID,
		&folder.GalleryVisibility,
		&folder.State,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Folder{}, err
	}
	folder.ParentID = parentID.String
	folder.OwnerVaultID = ownerVaultID.String

	var err error
	folder.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Folder{}, err
	}
	folder.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Folder{}, err
	}
	return folder, nil
}
