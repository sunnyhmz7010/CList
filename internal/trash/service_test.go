package trash

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
)

func TestDeleteMovesToTrashWithoutPhysicalBackend(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewFileRepo(database)
	if err := repo.Create(context.Background(), repository.File{PublicID: "file", StorageProfileID: "local-default", StorageKey: "objects/file", FileName: "a.txt", MIMEType: "text/plain", Size: 1, SHA256: "hash", State: repository.FileStateActive}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(database).DeleteFile(context.Background(), "file", auth.Actor{Kind: auth.ActorAdmin}); err != nil {
		t.Fatal(err)
	}
	file, err := repo.Get(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	if file.State != repository.FileStateTrashed || file.StorageKey != "objects/file" {
		t.Fatalf("文件 = %+v", file)
	}
}
