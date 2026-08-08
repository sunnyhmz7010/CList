package files

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
)

var (
	ErrForbidden    = errors.New("forbidden")
	ErrGone         = errors.New("file is gone")
	ErrInvalidInput = errors.New("invalid input")
	ErrFolderCycle  = errors.New("folder cycle")
)

type CreateFileInput struct {
	FileName         string
	MIMEType         string
	Size             int64
	SHA256           string
	FolderID         string
	OwnerVaultID     string
	StorageProfileID string
	StorageKey       string
}

type CreateFolderInput struct {
	ParentID     string
	Name         string
	OwnerVaultID string
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type FileService struct {
	files   *repository.FileRepo
	folders *repository.FolderRepo
}

type FolderService struct {
	folders *repository.FolderRepo
}

func NewFileService(files *repository.FileRepo, folders *repository.FolderRepo) *FileService {
	return &FileService{files: files, folders: folders}
}

func NewFolderService(folders *repository.FolderRepo) *FolderService {
	return &FolderService{folders: folders}
}

func (s *FileService) CreateIndex(ctx context.Context, input CreateFileInput, actor auth.Actor) (repository.File, error) {
	if !validName(input.FileName) || input.Size < 0 || input.MIMEType == "" || input.SHA256 == "" || input.StorageKey == "" {
		return repository.File{}, ErrInvalidInput
	}
	if input.StorageProfileID == "" {
		input.StorageProfileID = "local-default"
	}
	if actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest {
		input.OwnerVaultID = actor.ID
	} else if actor.Kind != auth.ActorAdmin && !hasScope(actor, "upload") {
		return repository.File{}, ErrForbidden
	}
	if input.FolderID != "" {
		folder, err := s.folders.Get(ctx, input.FolderID)
		if err != nil {
			return repository.File{}, err
		}
		if err := canManageFolder(actor, folder); err != nil {
			return repository.File{}, err
		}
	}
	publicID, err := newPublicID()
	if err != nil {
		return repository.File{}, err
	}
	file := repository.File{
		PublicID:          publicID,
		FolderID:          input.FolderID,
		OwnerVaultID:      input.OwnerVaultID,
		StorageProfileID:  input.StorageProfileID,
		StorageKey:        input.StorageKey,
		FileName:          input.FileName,
		MIMEType:          input.MIMEType,
		Size:              input.Size,
		SHA256:            input.SHA256,
		GalleryVisibility: repository.VisibilityInherit,
		State:             repository.FileStateActive,
	}
	if err := s.files.Create(ctx, file); err != nil {
		return repository.File{}, err
	}
	return s.files.Get(ctx, publicID)
}

func (s *FileService) List(ctx context.Context, actor auth.Actor, cursor string, limit int) (Page[repository.File], error) {
	limit = normalizePageLimit(limit)
	options := repository.FileListOptions{State: repository.FileStateActive, Cursor: cursor, Limit: limit + 1}
	if actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest {
		options.OwnerVaultID = &actor.ID
	} else if actor.Kind != auth.ActorAdmin && !hasScope(actor, "read") {
		return Page[repository.File]{}, ErrForbidden
	}
	items, err := s.files.List(ctx, options)
	if err != nil {
		return Page[repository.File]{}, err
	}
	return page(items, limit, func(file repository.File) string { return file.PublicID }), nil
}

func (s *FileService) Get(ctx context.Context, publicID string, actor auth.Actor) (repository.File, error) {
	file, err := s.files.Get(ctx, publicID)
	if err != nil {
		return repository.File{}, err
	}
	if file.State == repository.FileStateTrashed || file.State == repository.FileStatePurged {
		return repository.File{}, ErrGone
	}
	if actor.Kind == auth.ActorAdmin || hasScope(actor, "read") {
		return file, nil
	}
	if actor.Kind == auth.ActorPublic {
		return file, nil
	}
	if (actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest) && actor.ID == file.OwnerVaultID {
		return file, nil
	}
	return repository.File{}, ErrForbidden
}

func (s *FileService) Rename(ctx context.Context, publicID, name string, actor auth.Actor) error {
	if !validName(name) {
		return ErrInvalidInput
	}
	file, err := s.files.Get(ctx, publicID)
	if err != nil {
		return err
	}
	if err := canManageFile(actor, file); err != nil {
		return err
	}
	return s.files.Rename(ctx, publicID, name)
}

func (s *FileService) Move(ctx context.Context, publicID, folderID string, actor auth.Actor) error {
	file, err := s.files.Get(ctx, publicID)
	if err != nil {
		return err
	}
	if err := canManageFile(actor, file); err != nil {
		return err
	}
	if folderID != "" {
		folder, err := s.folders.Get(ctx, folderID)
		if err != nil {
			return err
		}
		if err := canManageFolder(actor, folder); err != nil {
			return err
		}
		if file.OwnerVaultID != folder.OwnerVaultID {
			return ErrForbidden
		}
	}
	return s.files.Move(ctx, publicID, folderID)
}

func (s *FileService) Delete(ctx context.Context, publicID string, actor auth.Actor) error {
	file, err := s.files.Get(ctx, publicID)
	if err != nil {
		return err
	}
	if err := canManageFile(actor, file); err != nil {
		return err
	}
	return s.files.UpdateState(ctx, publicID, repository.FileStateTrashed)
}

func (s *FolderService) Create(ctx context.Context, input CreateFolderInput, actor auth.Actor) (repository.Folder, error) {
	if !validName(input.Name) {
		return repository.Folder{}, ErrInvalidInput
	}
	if actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest {
		input.OwnerVaultID = actor.ID
	} else if actor.Kind != auth.ActorAdmin && !hasScope(actor, "manage") {
		return repository.Folder{}, ErrForbidden
	}
	if input.ParentID != "" {
		parent, err := s.folders.Get(ctx, input.ParentID)
		if err != nil {
			return repository.Folder{}, err
		}
		if err := canManageFolder(actor, parent); err != nil || parent.OwnerVaultID != input.OwnerVaultID {
			return repository.Folder{}, ErrForbidden
		}
	}
	id, err := newPublicID()
	if err != nil {
		return repository.Folder{}, err
	}
	folder := repository.Folder{
		ID:                id,
		ParentID:          input.ParentID,
		Name:              input.Name,
		OwnerVaultID:      input.OwnerVaultID,
		GalleryVisibility: repository.VisibilityInherit,
		State:             repository.FolderStateActive,
	}
	if err := s.folders.Create(ctx, folder); err != nil {
		return repository.Folder{}, err
	}
	return s.folders.Get(ctx, id)
}

func (s *FolderService) List(ctx context.Context, actor auth.Actor, parentID, cursor string, limit int) (Page[repository.Folder], error) {
	limit = normalizePageLimit(limit)
	options := repository.FolderListOptions{ParentID: parentID, Cursor: cursor, Limit: limit + 1, State: repository.FolderStateActive}
	if actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest {
		options.OwnerFilter = &actor.ID
	} else if actor.Kind != auth.ActorAdmin && !hasScope(actor, "read") {
		return Page[repository.Folder]{}, ErrForbidden
	}
	items, err := s.folders.List(ctx, options)
	if err != nil {
		return Page[repository.Folder]{}, err
	}
	return page(items, limit, func(folder repository.Folder) string { return folder.ID }), nil
}

func (s *FolderService) Move(ctx context.Context, id, parentID string, actor auth.Actor) error {
	folder, err := s.folders.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := canManageFolder(actor, folder); err != nil {
		return err
	}
	if id == parentID {
		return ErrFolderCycle
	}
	current := parentID
	for depth := 0; current != "" && depth < 1024; depth++ {
		parent, err := s.folders.Get(ctx, current)
		if err != nil {
			return err
		}
		if parent.ID == id {
			return ErrFolderCycle
		}
		if parent.OwnerVaultID != folder.OwnerVaultID {
			return ErrForbidden
		}
		current = parent.ParentID
	}
	if current != "" {
		return ErrFolderCycle
	}
	return s.folders.Move(ctx, id, parentID)
}

func (s *FolderService) Rename(ctx context.Context, id, name string, actor auth.Actor) error {
	if !validName(name) {
		return ErrInvalidInput
	}
	folder, err := s.folders.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := canManageFolder(actor, folder); err != nil {
		return err
	}
	return s.folders.Rename(ctx, id, name)
}

func (s *FolderService) Delete(ctx context.Context, id string, actor auth.Actor) error {
	folder, err := s.folders.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := canManageFolder(actor, folder); err != nil {
		return err
	}
	return s.folders.UpdateState(ctx, id, repository.FolderStateTrashed)
}

func canManageFile(actor auth.Actor, file repository.File) error {
	if actor.Kind == auth.ActorAdmin || hasScope(actor, "manage") {
		return nil
	}
	if (actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest) && actor.ID == file.OwnerVaultID {
		return nil
	}
	return ErrForbidden
}

func canManageFolder(actor auth.Actor, folder repository.Folder) error {
	if actor.Kind == auth.ActorAdmin || hasScope(actor, "manage") {
		return nil
	}
	if (actor.Kind == auth.ActorVault || actor.Kind == auth.ActorGuest) && actor.ID == folder.OwnerVaultID {
		return nil
	}
	return ErrForbidden
}

func hasScope(actor auth.Actor, scope string) bool {
	_, ok := actor.Scopes[scope]
	return ok
}

func validName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && name != "." && name != ".." && len(name) <= 255 && !strings.ContainsAny(name, `/\\`)
}

func newPublicID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func page[T any](items []T, limit int, cursor func(T) string) Page[T] {
	result := Page[T]{Items: items}
	if len(items) > limit {
		result.Items = items[:limit]
		result.NextCursor = cursor(result.Items[len(result.Items)-1])
	}
	return result
}

func normalizePageLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}
