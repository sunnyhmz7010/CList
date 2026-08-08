package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestApplyCreatesCoreTables(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := Apply(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"settings", "storage_profiles", "guest_vaults", "folders", "files",
		"file_secrets", "trash_batches", "trash_items", "jobs", "upload_sessions",
		"upload_chunks", "api_tokens", "sessions", "audit_logs", "telegram_messages",
		"idempotency_keys",
	} {
		var got string
		if err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			name,
		).Scan(&got); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}
