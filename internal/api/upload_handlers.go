package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/uploads"
)

type UploadHandlers struct {
	service *uploads.Service
}

type initUploadRequest struct {
	FileName         string `json:"file_name"`
	MIMEType         string `json:"mime_type"`
	TotalSize        int64  `json:"total_size"`
	ChunkSize        int64  `json:"chunk_size"`
	TotalChunks      int    `json:"total_chunks"`
	SHA256           string `json:"sha256"`
	FolderID         string `json:"folder_id"`
	StorageProfileID string `json:"storage_profile_id"`
}

func NewUploadHandlers(service *uploads.Service) *UploadHandlers {
	return &UploadHandlers{service: service}
}

func (h *UploadHandlers) Init(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request initUploadRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	upload, err := h.service.Init(r.Context(), uploads.InitInput{
		FileName: request.FileName, MIMEType: request.MIMEType, TotalSize: request.TotalSize,
		ChunkSize: request.ChunkSize, TotalChunks: request.TotalChunks, SHA256: request.SHA256,
		FolderID: request.FolderID, StorageProfileID: request.StorageProfileID,
		IdempotencyKey: r.Header.Get("Idempotency-Key"), Actor: actor,
	})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, upload)
}

func (h *UploadHandlers) PutChunk(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil {
		writeAPIError(w, r, files.ErrInvalidInput)
		return
	}
	if err := h.service.PutChunk(
		r.Context(), chi.URLParam(r, "id"), index, r.Body, r.Header.Get("X-Chunk-SHA256"), actor,
	); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UploadHandlers) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	upload, err := h.service.Get(r.Context(), chi.URLParam(r, "id"), actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, upload)
}

func (h *UploadHandlers) Complete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	file, err := h.service.Complete(
		r.Context(), chi.URLParam(r, "id"), actor, r.Header.Get("Idempotency-Key"),
	)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (h *UploadHandlers) Abort(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if err := h.service.Abort(r.Context(), chi.URLParam(r, "id"), actor); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
