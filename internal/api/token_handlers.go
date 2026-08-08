package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
)

type TokenHandlers struct{ service *auth.TokenService }

func NewTokenHandlers(service *auth.TokenService) *TokenHandlers {
	return &TokenHandlers{service: service}
}

type createTokenRequest struct {
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *TokenHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var request createTokenRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	plain, token, err := h.service.Create(request.Scopes, request.ExpiresAt)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plaintext": plain, "token": token})
}

func (h *TokenHandlers) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *TokenHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Revoke(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
