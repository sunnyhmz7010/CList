package api

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/preview"
)

type PreviewHandlers struct {
	service  *preview.Service
	password *auth.FilePasswordService
	auth     *auth.Authenticator
}

func NewPreviewHandlers(service *preview.Service, password *auth.FilePasswordService, authenticator *auth.Authenticator) *PreviewHandlers {
	return &PreviewHandlers{service: service, password: password, auth: authenticator}
}

func (h *PreviewHandlers) Serve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.password != nil && h.auth != nil {
		if err := h.password.AuthorizeRequest(r, id, h.auth); err != nil {
			writeAPIError(w, r, err)
			return
		}
	}
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		actor = auth.Actor{Kind: auth.ActorPublic}
	}
	item, err := h.service.Open(r.Context(), id, actor)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	defer item.Body.Close()
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; media-src 'self'; frame-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", item.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(item.Size, 10))
	if item.Kind == preview.KindDownload {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "preview"}))
	}
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, item.Body)
	}
}
