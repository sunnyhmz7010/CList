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
	Frontend      http.Handler
}

func NewRouter(deps RouterDeps) http.Handler {
	router := chi.NewRouter()
	deps.Auth.Routes(router)
	router.Get("/f/{publicID}/{filename}", deps.Downloads.Serve)
	router.Head("/f/{publicID}/{filename}", deps.Downloads.Serve)
	router.Route("/api/v1", func(api chi.Router) {
		api.Group(func(protected chi.Router) {
			protected.Use(deps.Authenticator.RequireAdmin)
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
		})
	})
	if deps.Frontend != nil {
		router.NotFound(deps.Frontend.ServeHTTP)
	}
	return router
}
