package api

import (
	"net/http"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/gallery"
)

type GalleryHandlers struct{ service *gallery.Service }

func NewGalleryHandlers(service *gallery.Service) *GalleryHandlers {
	return &GalleryHandlers{service: service}
}

func (h *GalleryHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	page, err := h.service.List(r.Context(), actor, gallery.ListOptions{
		Cursor: r.URL.Query().Get("cursor"), MIMEType: r.URL.Query().Get("type"), FolderID: r.URL.Query().Get("folder"), Name: r.URL.Query().Get("name"),
		From: queryTime(r, "from"), To: queryTime(r, "to"),
	})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func queryTime(r *http.Request, name string) *time.Time {
	value, err := time.Parse(time.RFC3339, r.URL.Query().Get(name))
	if err != nil {
		return nil
	}
	return &value
}
