package files

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
)

func TestMoveDoesNotChangePublicID(t *testing.T) {
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
	fileService := NewFileService(fileRepo, folderRepo)
	folderService := NewFolderService(folderRepo)
	actor := auth.Actor{Kind: auth.ActorAdmin}
	a, err := folderService.Create(context.Background(), CreateFolderInput{Name: "a-folder"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	b, err := folderService.Create(context.Background(), CreateFolderInput{Name: "b-folder"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	file, err := fileService.CreateIndex(context.Background(), CreateFileInput{
		FileName: "a.txt", MIMEType: "text/plain", Size: 1, SHA256: "x", FolderID: a.ID, StorageKey: "objects/a",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	before := file.PublicID
	if err := fileService.Move(context.Background(), file.PublicID, b.ID, actor); err != nil {
		t.Fatal(err)
	}
	got, err := fileService.Get(context.Background(), before, actor)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicID != before || got.FolderID != b.ID {
		t.Fatalf("got %+v", got)
	}
}
