package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
)

type FolderHandlers struct {
	service *files.FolderService
}

type createFolderRequest struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
}

type patchFolderRequest struct {
	ParentID *string `json:"parent_id"`
	Name     *string `json:"name"`
}

func NewFolderHandlers(service *files.FolderService) *FolderHandlers {
	return &FolderHandlers{service: service}
}

func (h *FolderHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	page, err := h.service.List(
		r.Context(), actor, r.URL.Query().Get("parent_id"), r.URL.Query().Get("cursor"), queryLimit(r),
	)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *FolderHandlers) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request createFolderRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	folder, err := h.service.Create(r.Context(), files.CreateFolderInput{
		ParentID: request.ParentID,
		Name:     request.Name,
	}, actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

func (h *FolderHandlers) Patch(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request patchFolderRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if request.Name != nil {
		if err := h.service.Rename(r.Context(), id, *request.Name, actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	if request.ParentID != nil {
		if err := h.service.Move(r.Context(), id, *request.ParentID, actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FolderHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
