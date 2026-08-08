package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/storage"
	"net/http"
)

type StorageHandlers struct{ service *storage.ProfileService }

func NewStorageHandlers(service *storage.ProfileService) *StorageHandlers {
	return &StorageHandlers{service: service}
}
func (h *StorageHandlers) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h *StorageHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var input storage.ProfileInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeAPIError(w, r, err)
		return
	}
	p, err := h.service.Create(r.Context(), input)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
func (h *StorageHandlers) SetDefault(w http.ResponseWriter, r *http.Request) {
	if err := h.service.SetDefault(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
