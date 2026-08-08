package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type FolderState string

const (
	FolderStateActive  FolderState = "active"
	FolderStateTrashed FolderState = "trashed"
	FolderStatePurged  FolderState = "purged"
)

type Folder struct {
	ID                string      `json:"id"`
	ParentID          string      `json:"parent_id,omitempty"`
	Name              string      `json:"name"`
	OwnerVaultID      string      `json:"owner_vault_id,omitempty"`
	GalleryVisibility Visibility  `json:"gallery_visibility"`
	State             FolderState `json:"state"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type FolderListOptions struct {
	ParentID    string
	OwnerFilter *string
	Cursor      string
	Limit       int
	State       FolderState
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
	var query string
	var arguments []any
	if options.ParentID == "" {
		query = `
SELECT id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
FROM folders
	WHERE parent_id IS NULL AND state = ? AND (? = '' OR id > ?)`
		arguments = []any{options.State, options.Cursor, options.Cursor}
	} else {
		query = `
SELECT id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
FROM folders
	WHERE parent_id = ? AND state = ? AND (? = '' OR id > ?)`
		arguments = []any{options.ParentID, options.State, options.Cursor, options.Cursor}
	}
	if options.OwnerFilter != nil {
		if *options.OwnerFilter == "" {
			query += " AND owner_vault_id IS NULL"
		} else {
			query += " AND owner_vault_id = ?"
			arguments = append(arguments, *options.OwnerFilter)
		}
	}
	query += " ORDER BY id LIMIT ?"
	arguments = append(arguments, limit)
	rows, err := r.db.QueryContext(ctx, query, arguments...)
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

func (r *FolderRepo) Get(ctx context.Context, id string) (Folder, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, parent_id, name, owner_vault_id, gallery_visibility, state, created_at, updated_at
FROM folders WHERE id = ?`, id)
	folder, err := scanFolder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, ErrNotFound
	}
	return folder, err
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

func (r *FolderRepo) Rename(ctx context.Context, id, name string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE folders SET name = ?, updated_at = ? WHERE id = ?",
		name, formatTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (r *FolderRepo) UpdateState(ctx context.Context, id string, state FolderState) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE folders SET state = ?, updated_at = ? WHERE id = ?",
		state, formatTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (r *FolderRepo) SetGalleryVisibility(ctx context.Context, id string, visibility Visibility) error {
	result, err := r.db.ExecContext(ctx, "UPDATE folders SET gallery_visibility=?,updated_at=? WHERE id=?", visibility, formatTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (r *FolderRepo) EffectiveVisibility(ctx context.Context, id string) (bool, error) {
	for depth := 0; id != "" && depth < 1024; depth++ {
		folder, err := r.Get(ctx, id)
		if err != nil {
			return false, err
		}
		switch folder.GalleryVisibility {
		case VisibilityVisible:
			return true, nil
		case VisibilityHidden:
			return false, nil
		}
		id = folder.ParentID
	}
	if id != "" {
		return false, errors.New("folder visibility cycle")
	}
	return true, nil
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
