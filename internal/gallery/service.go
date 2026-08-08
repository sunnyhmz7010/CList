package gallery

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
)

var ErrDisabled = errors.New("gallery is disabled")

type Service struct {
	db      *sql.DB
	files   *repository.FileRepo
	folders *repository.FolderRepo
}

func NewService(database *sql.DB, fileRepo *repository.FileRepo, folderRepo *repository.FolderRepo) *Service {
	return &Service{db: database, files: fileRepo, folders: folderRepo}
}

func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES('gallery.enabled',?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, value, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Service) Enabled(ctx context.Context) (bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='gallery.enabled'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	return value == "1", err
}

func (s *Service) ResolveVisibility(ctx context.Context, id string) (bool, error) {
	file, err := s.files.Get(ctx, id)
	if err != nil {
		return false, err
	}
	switch file.GalleryVisibility {
	case repository.VisibilityVisible:
		return true, nil
	case repository.VisibilityHidden:
		return false, nil
	default:
		return s.folders.EffectiveVisibility(ctx, file.FolderID)
	}
}

type ListOptions struct {
	Cursor, MIMEType, FolderID, Name string
	From, To                         *time.Time
}

func (s *Service) List(ctx context.Context, actor auth.Actor, options ListOptions) (files.Page[repository.File], error) {
	enabled, err := s.Enabled(ctx)
	if err != nil {
		return files.Page[repository.File]{}, err
	}
	if !enabled {
		return files.Page[repository.File]{}, ErrDisabled
	}
	if err := auth.RequireScope(actor, auth.ScopeGallery); err != nil && actor.Kind != auth.ActorAdmin {
		return files.Page[repository.File]{}, err
	}
	visible := make([]repository.File, 0, 201)
	cursor := options.Cursor
	for len(visible) <= 200 {
		items, err := s.files.List(ctx, repository.FileListOptions{State: repository.FileStateActive, Cursor: cursor, Limit: 201, MIMEType: options.MIMEType, FolderID: options.FolderID, Name: options.Name, From: options.From, To: options.To})
		if err != nil {
			return files.Page[repository.File]{}, err
		}
		for _, item := range items {
			ok, err := s.ResolveVisibility(ctx, item.PublicID)
			if err != nil {
				return files.Page[repository.File]{}, err
			}
			if ok {
				visible = append(visible, item)
			}
		}
		if len(items) < 201 {
			break
		}
		cursor = items[len(items)-1].PublicID
	}
	result := files.Page[repository.File]{Items: visible}
	if len(visible) > 200 {
		result.Items = visible[:200]
		result.NextCursor = visible[199].PublicID
	}
	return result, nil
}
