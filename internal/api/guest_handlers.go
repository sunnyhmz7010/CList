package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
)

type GuestHandlers struct{ service *auth.GuestService }
type passwordRequest struct {
	Password string `json:"password"`
}

func NewGuestHandlers(service *auth.GuestService) *GuestHandlers {
	return &GuestHandlers{service: service}
}

func (h *GuestHandlers) Login(w http.ResponseWriter, r *http.Request) {
	scope, ok := requestScope(chi.URLParam(r, "scope"))
	if !ok {
		writeAPIError(w, r, errInvalidRequest)
		return
	}
	var request passwordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	session, err := h.service.Login(r.Context(), scope, request.Password)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	setScopedCookie(w, auth.GuestCookieName(scope), session)
	writeJSON(w, http.StatusOK, map[string]string{"scope": string(scope)})
}

func (h *GuestHandlers) SetPassword(w http.ResponseWriter, r *http.Request) {
	scope, ok := requestScope(chi.URLParam(r, "scope"))
	if !ok {
		writeAPIError(w, r, errInvalidRequest)
		return
	}
	var request passwordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	var err error
	if request.Password == "" {
		err = h.service.ClearPassword(r.Context(), scope)
	} else if scope == auth.ScopeHome {
		err = h.service.SetHomePassword(r.Context(), request.Password)
	} else {
		err = h.service.SetGalleryPassword(r.Context(), request.Password)
	}
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requestScope(value string) (auth.Scope, bool) {
	scope := auth.Scope(value)
	return scope, scope == auth.ScopeHome || scope == auth.ScopeGallery
}
func setScopedCookie(w http.ResponseWriter, name string, session auth.Session) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: session.Token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds())})
}
