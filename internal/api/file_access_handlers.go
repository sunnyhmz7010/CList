package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
)

type FileAccessHandlers struct {
	service *auth.FilePasswordService
	files   *files.FileService
}

func NewFileAccessHandlers(service *auth.FilePasswordService, fileService *files.FileService) *FileAccessHandlers {
	return &FileAccessHandlers{service: service, files: fileService}
}
func (h *FileAccessHandlers) Set(w http.ResponseWriter, r *http.Request) {
	var request passwordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	actor, _ := auth.ActorFromContext(r.Context())
	if _, err := h.files.Get(r.Context(), id, actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := h.service.Set(r.Context(), id, request.Password); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *FileAccessHandlers) Clear(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	actor, _ := auth.ActorFromContext(r.Context())
	if _, err := h.files.Get(r.Context(), id, actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := h.service.Clear(r.Context(), id); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *FileAccessHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	var request passwordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	session, err := h.service.Verify(r.Context(), id, request.Password)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	setScopedCookie(w, auth.FileCookieName(id), session)
	w.WriteHeader(http.StatusNoContent)
}
