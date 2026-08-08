package uploads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/storage"
	"github.com/sunnyhmz7010/CList/internal/storage/local"
)

func TestChunksAreResumableAndCompleteChecksSHA256(t *testing.T) {
	root := t.TempDir()
	database, err := db.Open(context.Background(), filepath.Join(root, "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	registry := storage.NewRegistry()
	registry.Register("local-default", local.New(root))
	fileService := files.NewFileService(repository.NewFileRepo(database), repository.NewFolderRepo(database))
	service := NewService(database, filepath.Join(root, "chunks"), registry, fileService)
	actor := auth.Actor{Kind: auth.ActorAdmin}
	upload, err := service.Init(context.Background(), InitInput{
		FileName: "a.txt", MIMEType: "text/plain", TotalSize: 11, ChunkSize: 6, TotalChunks: 2,
		SHA256: sum("hello world"), Actor: actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.PutChunk(context.Background(), upload.ID, 1, strings.NewReader("world"), sum("world"), actor); err != nil {
		t.Fatal(err)
	}
	if err := service.PutChunk(context.Background(), upload.ID, 0, strings.NewReader("hello "), sum("hello "), actor); err != nil {
		t.Fatal(err)
	}
	if err := service.PutChunk(context.Background(), upload.ID, 0, strings.NewReader("hello "), sum("hello "), actor); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), upload.ID, actor, ""); err != nil {
		t.Fatal(err)
	}
}

func sum(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
