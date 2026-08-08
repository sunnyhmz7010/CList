package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sunnyhmz7010/CList/internal/config"
	"github.com/sunnyhmz7010/CList/internal/crypto"
	"github.com/sunnyhmz7010/CList/internal/db"
)

// frontend 包含由 Vite 构建的前端静态资源。
//
//go:embed all:web-dist
var frontend embed.FS

func defaultDataDir() string {
	return "/data"
}

func frontendHandler() (http.Handler, error) {
	content, err := fs.Sub(frontend, "web-dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(content)), nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(defaultDataDir())
	if err != nil {
		return err
	}
	if err := prepareDataDirs(cfg.DataDir); err != nil {
		return err
	}
	if _, err := crypto.MasterKey.LoadOrCreate(filepath.Join(cfg.DataDir, "secrets", "master.key")); err != nil {
		return err
	}
	database, err := db.Open(context.Background(), filepath.Join(cfg.DataDir, "clist.db"))
	if err != nil {
		return err
	}
	defer database.Close()
	if err := db.Apply(context.Background(), database); err != nil {
		return err
	}

	handler, err := frontendHandler()
	if err != nil {
		return err
	}
	return http.ListenAndServe(cfg.ListenAddr, handler)
}

func prepareDataDirs(dataDir string) error {
	for _, dir := range []string{
		filepath.Join(dataDir, "files"),
		filepath.Join(dataDir, "chunks"),
		filepath.Join(dataDir, "migrations"),
		filepath.Join(dataDir, "cache", "previews"),
		filepath.Join(dataDir, "secrets"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	return nil
}
