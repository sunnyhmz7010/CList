package storage

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func TestProfileConfigIsEncryptedAtRest(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	service := NewProfileService(database, bytes.Repeat([]byte("x"), 32), NewRegistry())
	profile, err := service.Create(context.Background(), ProfileInput{Type: "local", Name: "local", Enabled: true, Config: map[string]string{"root": root, "bot_token": "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := database.QueryRow("SELECT encrypted_config FROM storage_profiles WHERE id=?", profile.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("secret")) {
		t.Fatal("plaintext secret persisted")
	}
}
