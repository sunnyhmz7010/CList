package api

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/files"
	"github.com/sunnyhmz7010/CList/internal/storage"
)

type DownloadHandlers struct {
	files     *files.FileService
	registry  *storage.Registry
	passwords *auth.FilePasswordService
	admin     *auth.Authenticator
}

func NewDownloadHandlers(fileService *files.FileService, registry *storage.Registry, passwords *auth.FilePasswordService, admin *auth.Authenticator) *DownloadHandlers {
	return &DownloadHandlers{files: fileService, registry: registry, passwords: passwords, admin: admin}
}

func (h *DownloadHandlers) Serve(w http.ResponseWriter, r *http.Request) {
	if err := h.passwords.AuthorizeRequest(r, chi.URLParam(r, "publicID"), h.admin); err != nil {
		writeAPIError(w, r, err)
		return
	}
	file, err := h.files.Get(r.Context(), chi.URLParam(r, "publicID"), auth.Actor{Kind: auth.ActorPublic})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	backend, err := h.registry.Resolve(file.StorageProfileID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	capabilities := backend.Capabilities()
	w.Header().Set("X-CList-Storage-Capabilities", capabilityHeader(capabilities))
	if capabilities.Range {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	if r.Method == http.MethodHead && !capabilities.Head {
		writeAPIError(w, r, storage.ErrRangeUnsupported)
		return
	}
	if r.Header.Get("Range") != "" && !capabilities.Range {
		writeAPIError(w, r, storage.ErrRangeUnsupported)
		return
	}
	byteRange, err := parseRange(r.Header.Get("Range"), file.Size)
	if err != nil {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(file.Size, 10))
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	reader, err := backend.Open(r.Context(), file.StorageKey, byteRange)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", file.MIMEType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.FileName}))
	w.Header().Set("Content-Length", strconv.FormatInt(reader.Size, 10))
	status := http.StatusOK
	if reader.Partial {
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", reader.ContentRange)
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, reader)
	}
}

func parseRange(value string, size int64) (*storage.ByteRange, error) {
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return nil, storage.ErrInvalidRange
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		return nil, storage.ErrInvalidRange
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return nil, storage.ErrInvalidRange
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || end >= size {
			return nil, storage.ErrInvalidRange
		}
	}
	return &storage.ByteRange{Start: start, End: end}, nil
}

func capabilityHeader(capabilities storage.Capabilities) string {
	return fmt.Sprintf("range=%t; head=%t; streaming=%t", capabilities.Range, capabilities.Head, capabilities.Streaming)
}
