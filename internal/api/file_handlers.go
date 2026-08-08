package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
)

type FileHandlers struct {
	service *files.FileService
}

type patchFileRequest struct {
	FileName          *string                `json:"file_name"`
	FolderID          *string                `json:"folder_id"`
	GalleryVisibility *repository.Visibility `json:"gallery_visibility"`
}

func NewFileHandlers(service *files.FileService) *FileHandlers {
	return &FileHandlers{service: service}
}

func (h *FileHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	page, err := h.service.List(r.Context(), actor, r.URL.Query().Get("cursor"), queryLimit(r))
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *FileHandlers) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	file, err := h.service.Get(r.Context(), chi.URLParam(r, "id"), actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (h *FileHandlers) Patch(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request patchFileRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if request.FileName != nil {
		if err := h.service.Rename(r.Context(), id, *request.FileName, actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	if request.FolderID != nil {
		if err := h.service.Move(r.Context(), id, *request.FolderID, actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	if request.GalleryVisibility != nil {
		if err := h.service.SetGalleryVisibility(r.Context(), id, *request.GalleryVisibility, actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	file, err := h.service.Get(r.Context(), id, actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (h *FileHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queryLimit(r *http.Request) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return 50
	}
	if value > 200 {
		return 200
	}
	return value
}
