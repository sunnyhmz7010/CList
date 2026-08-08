package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
)

type RouterDeps struct {
	Auth          *AuthHandlers
	Authenticator *auth.Authenticator
	Files         *FileHandlers
	Folders       *FolderHandlers
	Uploads       *UploadHandlers
	Downloads     *DownloadHandlers
	Guests        *GuestHandlers
	FileAccess    *FileAccessHandlers
	Vaults        *VaultHandlers
	Storage       *StorageHandlers
	Gallery       *GalleryHandlers
	GalleryAuth   *auth.GuestAuthenticator
	Webhook       http.Handler
	Frontend      http.Handler
}

func NewRouter(deps RouterDeps) http.Handler {
	router := chi.NewRouter()
	deps.Auth.Routes(router)
	router.Post("/api/v1/guest/{scope}/session", deps.Guests.Login)
	router.Post("/api/v1/files/{id}/access", deps.FileAccess.Verify)
	router.Post("/api/v1/vault", deps.Vaults.Create)
	router.Post("/api/v1/vault/recover", deps.Vaults.Recover)
	if deps.Gallery != nil && deps.GalleryAuth != nil {
		router.With(deps.GalleryAuth.RequireScope(auth.ScopeGallery)).Get("/api/v1/gallery", deps.Gallery.List)
	}
	if deps.Webhook != nil {
		router.Post("/webhooks/telegram/{profileSecret}", deps.Webhook.ServeHTTP)
	}
	router.Get("/f/{publicID}/{filename}", deps.Downloads.Serve)
	router.Head("/f/{publicID}/{filename}", deps.Downloads.Serve)
	router.Route("/api/v1", func(api chi.Router) {
		api.Group(func(protected chi.Router) {
			protected.Use(deps.Authenticator.RequireManager)
			protected.Get("/files", deps.Files.List)
			protected.Get("/files/{id}", deps.Files.Get)
			protected.Patch("/files/{id}", deps.Files.Patch)
			protected.Delete("/files/{id}", deps.Files.Delete)
			protected.Get("/folders", deps.Folders.List)
			protected.Post("/folders", deps.Folders.Create)
			protected.Patch("/folders/{id}", deps.Folders.Patch)
			protected.Delete("/folders/{id}", deps.Folders.Delete)
			protected.Post("/uploads", deps.Uploads.Init)
			protected.Put("/uploads/{id}/chunks/{index}", deps.Uploads.PutChunk)
			protected.Get("/uploads/{id}", deps.Uploads.Get)
			protected.Post("/uploads/{id}/complete", deps.Uploads.Complete)
			protected.Delete("/uploads/{id}", deps.Uploads.Abort)
			protected.Put("/files/{id}/password", deps.FileAccess.Set)
			protected.Delete("/files/{id}/password", deps.FileAccess.Clear)
			protected.Post("/vault/revoke", deps.Vaults.Revoke)
			protected.Get("/vault/files", deps.Files.List)
		})
		api.Group(func(admin chi.Router) {
			admin.Use(deps.Authenticator.RequireAdmin)
			admin.Put("/admin/access/{scope}", deps.Guests.SetPassword)
			admin.Get("/storage-profiles", deps.Storage.List)
			admin.Post("/storage-profiles", deps.Storage.Create)
			admin.Post("/storage-profiles/{id}/default", deps.Storage.SetDefault)
		})
	})
	if deps.Frontend != nil {
		router.NotFound(deps.Frontend.ServeHTTP)
	}
	return router
}
