package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestInitializeIsSingleUseAndLoginUsesConstantTimeCheck(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}

	service := NewAdminService(database)
	if err := service.Initialize(context.Background(), "admin", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if err := service.Initialize(context.Background(), "other", "another"); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("got %v", err)
	}
	if _, err := service.Login(context.Background(), "admin", "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}
