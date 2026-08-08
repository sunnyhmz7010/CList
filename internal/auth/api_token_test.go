package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestTokenCannotUseMissingScope(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	service := NewTokenService(database)
	plain, _, err := service.Create([]string{"upload"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := service.Authenticate(context.Background(), plain)
	if err != nil {
		t.Fatal(err)
	}
	if HasScope(actor, "delete") {
		t.Fatal("upload token unexpectedly has delete scope")
	}
}
