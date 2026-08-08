package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Apply(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("创建迁移记录表: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录: %w", err)
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)

	for _, version := range versions {
		applied, err := migrationApplied(ctx, database, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, database, version); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, database *sql.DB, version string) (bool, error) {
	var stored string
	err := database.QueryRowContext(
		ctx,
		"SELECT version FROM schema_migrations WHERE version = ?",
		version,
	).Scan(&stored)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("查询迁移 %s: %w", version, err)
}

func applyMigration(ctx context.Context, database *sql.DB, version string) error {
	script, err := migrationFiles.ReadFile("migrations/" + version)
	if err != nil {
		return fmt.Errorf("读取迁移 %s: %w", version, err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始迁移 %s: %w", version, err)
	}
	defer tx.Rollback()

	if err := executeScript(ctx, tx, string(script)); err != nil {
		return fmt.Errorf("执行迁移 %s: %w", version, err)
	}
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)",
		version,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("记录迁移 %s: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移 %s: %w", version, err)
	}
	return nil
}

func executeScript(ctx context.Context, tx *sql.Tx, script string) error {
	for _, statement := range strings.Split(script, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
