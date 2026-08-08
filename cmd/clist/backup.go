package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sunnyhmz7010/CList/internal/db"
)

func runBackup(arguments []string) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	dataDir := flags.String("data-dir", "/data", "CList 数据目录")
	output := flags.String("output", "", "SQLite 备份文件")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("必须指定 --output")
	}
	outputPath, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return errors.New("备份目标已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	keyOutput := outputPath + ".master.key"
	if _, err := os.Lstat(keyOutput); err == nil {
		return errors.New("主密钥备份目标已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return err
	}
	database, err := db.Open(context.Background(), filepath.Join(*dataDir, "clist.db"))
	if err != nil {
		return err
	}
	defer database.Close()
	quoted := "'" + strings.ReplaceAll(filepath.ToSlash(outputPath), "'", "''") + "'"
	if _, err := database.ExecContext(context.Background(), "VACUUM INTO "+quoted); err != nil {
		return fmt.Errorf("创建 SQLite 快照: %w", err)
	}
	if err := copyFile(filepath.Join(*dataDir, "secrets", "master.key"), keyOutput, 0o600); err != nil {
		_ = os.Remove(outputPath)
		return err
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		output.Close()
		if !ok {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
