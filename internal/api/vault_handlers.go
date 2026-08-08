package api

import (
	"net/http"
	"time"

	"github.com/sunnyhmz7010/CList/internal/auth"
)

type VaultHandlers struct{ service *auth.VaultService }
type recoverVaultRequest struct {
	Key string `json:"key"`
}

func NewVaultHandlers(service *auth.VaultService) *VaultHandlers {
	return &VaultHandlers{service: service}
}

func (h *VaultHandlers) Create(w http.ResponseWriter, r *http.Request) {
	vault, key, err := h.service.Create(r.Context())
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	_, session, err := h.service.Recover(r.Context(), key)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	setVaultCookies(w, session)
	writeJSON(w, http.StatusCreated, map[string]any{"vault": vault, "recovery_key": key})
}

func (h *VaultHandlers) Recover(w http.ResponseWriter, r *http.Request) {
	var request recoverVaultRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	vault, session, err := h.service.Recover(r.Context(), request.Key)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	setVaultCookies(w, session)
	writeJSON(w, http.StatusOK, vault)
}

func (h *VaultHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	if actor.Kind != auth.ActorVault {
		writeAPIError(w, r, auth.ErrUnauthorized)
		return
	}
	if err := h.service.Revoke(r.Context(), actor.ID); err != nil {
		writeAPIError(w, r, err)
		return
	}
	clearNamedCookie(w, auth.VaultCookieName, true)
	w.WriteHeader(http.StatusNoContent)
}

func setVaultCookies(w http.ResponseWriter, session auth.Session) {
	setScopedCookie(w, auth.VaultCookieName, session)
	http.SetCookie(w, &http.Cookie{Name: auth.CSRFCookieName, Value: session.CSRFToken, Path: "/", SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds())})
}

func clearNamedCookie(w http.ResponseWriter, name string, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: httpOnly, SameSite: http.SameSiteStrictMode})
}
