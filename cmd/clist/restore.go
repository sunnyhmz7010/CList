package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
)

func runRestore(arguments []string) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	dataDir := flags.String("data-dir", "/data", "CList 数据目录")
	input := flags.String("input", "", "SQLite 备份文件")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *input == "" {
		return errors.New("必须指定 --input")
	}
	inputPath, err := filepath.Abs(*input)
	if err != nil {
		return err
	}
	keyInput := inputPath + ".master.key"
	for _, source := range []string{inputPath, keyInput} {
		if info, err := os.Stat(source); err != nil || !info.Mode().IsRegular() {
			return errors.New("备份文件或主密钥不存在")
		}
	}
	if err := os.MkdirAll(filepath.Join(*dataDir, "secrets"), 0o750); err != nil {
		return err
	}
	databaseTarget := filepath.Join(*dataDir, "clist.db")
	keyTarget := filepath.Join(*dataDir, "secrets", "master.key")
	for _, target := range []string{databaseTarget, keyTarget} {
		if _, err := os.Lstat(target); err == nil {
			return errors.New("恢复目标非空，拒绝覆盖")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := copyFile(inputPath, databaseTarget, 0o600); err != nil {
		return err
	}
	if err := copyFile(keyInput, keyTarget, 0o600); err != nil {
		_ = os.Remove(databaseTarget)
		return err
	}
	return nil
}
