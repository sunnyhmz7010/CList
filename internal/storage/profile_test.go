package storage

import (
	"bytes"
	"context"
	"io"
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
	registry := NewRegistry()
	registry.SetFactory(func(_ string, _ map[string]string) (Backend, error) {
		return profileBackend{}, nil
	})
	service := NewProfileService(database, bytes.Repeat([]byte("x"), 32), registry)
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

type profileBackend struct{}

func (profileBackend) Validate(context.Context) error    { return nil }
func (profileBackend) Capabilities() Capabilities        { return Capabilities{} }
func (profileBackend) HealthCheck(context.Context) error { return nil }
func (profileBackend) Put(context.Context, io.Reader, ObjectMeta) (Object, error) {
	return Object{}, nil
}
func (profileBackend) Open(context.Context, string, *ByteRange) (Reader, error) {
	return Reader{}, nil
}
func (profileBackend) Delete(context.Context, string) error { return nil }
