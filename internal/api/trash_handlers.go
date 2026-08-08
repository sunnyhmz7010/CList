package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/trash"
)

type TrashHandlers struct {
	service *trash.Service
	purge   *trash.PurgeService
}

func NewTrashHandlers(service *trash.Service, purge *trash.PurgeService) *TrashHandlers {
	return &TrashHandlers{service: service, purge: purge}
}

func (h *TrashHandlers) DeleteFile(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if _, err := h.service.DeleteFile(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TrashHandlers) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if _, err := h.service.DeleteFolder(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TrashHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	items, err := h.service.List(r.Context(), actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *TrashHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if err := h.service.RestoreBatch(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TrashHandlers) Purge(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if err := h.purge.PurgeBatch(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
