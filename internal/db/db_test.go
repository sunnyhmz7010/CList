package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenEnablesWALAndForeignKeys(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var journal string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	var foreignKeys int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journal, "wal") || foreignKeys != 1 {
		t.Fatalf("journal=%s fk=%d", journal, foreignKeys)
	}
}
