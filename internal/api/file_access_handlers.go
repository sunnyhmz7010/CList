package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
)

type FileAccessHandlers struct{ service *auth.FilePasswordService }

func NewFileAccessHandlers(service *auth.FilePasswordService) *FileAccessHandlers {
	return &FileAccessHandlers{service: service}
}
func (h *FileAccessHandlers) Set(w http.ResponseWriter, r *http.Request) {
	var request passwordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := h.service.Set(r.Context(), chi.URLParam(r, "id"), request.Password); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *FileAccessHandlers) Clear(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Clear(r.Context(), chi.URLParam(r, "id")); err != nil {
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
