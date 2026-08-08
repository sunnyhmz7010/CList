package trash

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

func TestPurgeRetainsFailedItem(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewFileRepo(database)
	if err := repo.Create(context.Background(), repository.File{PublicID: "file", StorageProfileID: "local-default", StorageKey: "objects/file", FileName: "a", MIMEType: "text/plain", Size: 1, SHA256: "hash", State: repository.FileStateTrashed}); err != nil {
		t.Fatal(err)
	}
	registry := storage.NewRegistry()
	registry.Register("local-default", failingDeleteBackend{})
	if err := NewPurgeService(database, registry).PurgeFile(context.Background(), "file", auth.Actor{Kind: auth.ActorAdmin}); err == nil {
		t.Fatal("预期物理删除失败")
	}
	file, err := repo.Get(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	if file.State != repository.FileStateTrashed || file.LastError == "" {
		t.Fatalf("文件 = %+v", file)
	}
}

type failingDeleteBackend struct{}

func (failingDeleteBackend) Validate(context.Context) error     { return nil }
func (failingDeleteBackend) Capabilities() storage.Capabilities { return storage.Capabilities{} }
func (failingDeleteBackend) HealthCheck(context.Context) error  { return nil }
func (failingDeleteBackend) Put(context.Context, io.Reader, storage.ObjectMeta) (storage.Object, error) {
	return storage.Object{}, nil
}
func (failingDeleteBackend) Open(context.Context, string, *storage.ByteRange) (storage.Reader, error) {
	return storage.Reader{}, nil
}
func (failingDeleteBackend) Delete(context.Context, string) error { return errors.New("offline") }
