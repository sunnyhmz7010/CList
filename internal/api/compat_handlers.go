package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/uploads"
)

type CompatHandlers struct{ uploads *uploads.Service }

func NewCompatHandlers(service *uploads.Service) *CompatHandlers {
	return &CompatHandlers{uploads: service}
}

func (h *CompatHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if actor.Kind == auth.ActorToken && !auth.HasScope(actor, "upload") {
		writeAPIError(w, r, files.ErrForbidden)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeAPIError(w, r, errInvalidRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, errInvalidRequest)
		return
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeAPIError(w, r, err)
		return
	}
	const chunkSize = int64(8 << 20)
	totalChunks := int((size + chunkSize - 1) / chunkSize)
	if totalChunks == 0 {
		totalChunks = 1
	}
	upload, err := h.uploads.Init(r.Context(), uploads.InitInput{FileName: header.Filename, MIMEType: header.Header.Get("Content-Type"), TotalSize: size, ChunkSize: chunkSize, TotalChunks: totalChunks, SHA256: hex.EncodeToString(hash.Sum(nil)), IdempotencyKey: r.Header.Get("Idempotency-Key"), Actor: actor})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	buffer := make([]byte, chunkSize)
	for index := 0; index < totalChunks; index++ {
		read, readErr := io.ReadFull(file, buffer)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			writeAPIError(w, r, readErr)
			return
		}
		chunk := buffer[:read]
		chunkHash := sha256.Sum256(chunk)
		if err := h.uploads.PutChunk(r.Context(), upload.ID, index, bytes.NewReader(chunk), hex.EncodeToString(chunkHash[:]), actor); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	created, err := h.uploads.Complete(r.Context(), upload.ID, actor, r.Header.Get("Idempotency-Key")+":complete")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	response := map[string]any{"url": "/f/" + created.PublicID + "/" + created.FileName, "public_id": created.PublicID}
	if actor.Kind != auth.ActorToken || auth.HasScope(actor, "delete") || auth.HasScope(actor, "manage") {
		response["delete_url"] = "/api/v1/files/" + created.PublicID
	}
	writeJSON(w, http.StatusCreated, response)
}
