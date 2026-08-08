package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sunnyhmz7010/CList/internal/api"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/config"
	"github.com/sunnyhmz7010/CList/internal/crypto"
	"github.com/sunnyhmz7010/CList/internal/db"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/gallery"
	"github.com/sunnyhmz7010/CList/internal/preview"
	"github.com/sunnyhmz7010/CList/internal/storage"
	"github.com/sunnyhmz7010/CList/internal/storage/local"
	"github.com/sunnyhmz7010/CList/internal/storage/streaming"
	"github.com/sunnyhmz7010/CList/internal/storage/telegram"
	"github.com/sunnyhmz7010/CList/internal/trash"
	"github.com/sunnyhmz7010/CList/internal/uploads"
	"github.com/sunnyhmz7010/CList/internal/webhook"
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
	masterKey, err := crypto.MasterKey.LoadOrCreate(filepath.Join(cfg.DataDir, "secrets", "master.key"))
	if err != nil {
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
	authenticator := auth.NewAuthenticator(database)
	fileRepo := repository.NewFileRepo(database)
	folderRepo := repository.NewFolderRepo(database)
	fileService := files.NewFileService(fileRepo, folderRepo)
	folderService := files.NewFolderService(folderRepo)
	registry := storage.NewRegistry()
	registry.SetFactory(func(kind string, values map[string]string) (storage.Backend, error) {
		switch kind {
		case "local":
			return local.New(values["root"]), nil
		case "telegram_official":
			return telegram.New(telegram.Config{
				BaseURL:   values["base_url"],
				BotToken:  values["bot_token"],
				ChannelID: values["channel_id"],
			}), nil
		case "telegram_streaming":
			return streaming.New(streaming.Config{
				BaseURL:   values["base_url"],
				BotToken:  values["bot_token"],
				ChannelID: values["channel_id"],
			}), nil
		default:
			return nil, storage.ErrBackendNotFound
		}
	})
	profileService := storage.NewProfileService(database, masterKey, registry)
	registry.Register("local-default", local.New(filepath.Join(cfg.DataDir, "files")))
	if err := profileService.LoadEnabled(context.Background()); err != nil {
		return err
	}
	uploadService := uploads.NewService(database, filepath.Join(cfg.DataDir, "chunks"), registry, fileService)
	guestService := auth.NewGuestService(database)
	guestAuthenticator := auth.NewGuestAuthenticator(database, authenticator)
	galleryService := gallery.NewService(database, fileRepo, folderRepo)
	previewService := preview.NewService(fileService, registry)
	trashService := trash.NewService(database)
	filePasswordService := auth.NewFilePasswordService(database)
	vaultService := auth.NewVaultService(database)
	webhookResolver := webhook.NewStorageResolver(profileService, func(baseURL, token string) webhook.Sender {
		return telegram.NewClient(baseURL, token, nil)
	})
	webhookHandler := api.NewWebhookHandlers(webhook.NewHandler(webhook.NewService(database, webhookResolver)))
	router := api.NewRouter(api.RouterDeps{
		Auth:          api.NewAuthHandlers(auth.NewAdminService(database), authenticator),
		Authenticator: authenticator,
		Files:         api.NewFileHandlers(fileService),
		Folders:       api.NewFolderHandlers(folderService),
		Uploads:       api.NewUploadHandlers(uploadService),
		Downloads:     api.NewDownloadHandlers(fileService, registry, filePasswordService, authenticator),
		Guests:        api.NewGuestHandlers(guestService),
		FileAccess:    api.NewFileAccessHandlers(filePasswordService, fileService),
		Vaults:        api.NewVaultHandlers(vaultService),
		Storage:       api.NewStorageHandlers(profileService),
		Gallery:       api.NewGalleryHandlers(galleryService),
		GalleryAuth:   guestAuthenticator,
		Preview:       api.NewPreviewHandlers(previewService, filePasswordService, authenticator),
		Trash:         api.NewTrashHandlers(trashService),
		Webhook:       webhookHandler,
		Frontend:      handler,
	})
	return http.ListenAndServe(cfg.ListenAddr, router)
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
