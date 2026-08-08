package gallery

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
)

func TestFileVisibilityOverridesFolder(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	fileRepo := repository.NewFileRepo(database)
	folderRepo := repository.NewFolderRepo(database)
	if err := folderRepo.Create(context.Background(), repository.Folder{ID: "folder", Name: "隐藏目录", GalleryVisibility: repository.VisibilityHidden}); err != nil {
		t.Fatal(err)
	}
	if err := fileRepo.Create(context.Background(), repository.File{PublicID: "file", FolderID: "folder", StorageProfileID: "local-default", StorageKey: "objects/file", FileName: "a.png", MIMEType: "image/png", Size: 1, SHA256: "hash", GalleryVisibility: repository.VisibilityVisible, State: repository.FileStateActive}); err != nil {
		t.Fatal(err)
	}
	visible, err := NewService(database, fileRepo, folderRepo).ResolveVisibility(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Fatal("文件的 visible 未覆盖隐藏目录")
	}
}
