package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestVaultStoresHashAndRecoversAcrossDevices(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	service := NewVaultService(database)
	vault, key, err := service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if vault.KeyHash == key || len(key) < 32 {
		t.Fatal("weak or plaintext key")
	}
	got, _, err := service.Recover(context.Background(), key)
	if err != nil || got.ID != vault.ID {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
