package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/db/repository"
	"github.com/sunnyhmz7010/CList/internal/files"
)

type AuthHandlers struct {
	admin         *auth.AdminService
	authenticator *auth.Authenticator
}

type credentialsRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Retriable bool   `json:"retriable"`
}

var errInvalidRequest = errors.New("invalid request")

func NewAuthHandlers(admin *auth.AdminService, authenticator *auth.Authenticator) *AuthHandlers {
	return &AuthHandlers{admin: admin, authenticator: authenticator}
}

func (h *AuthHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/status", h.status)
	mux.HandleFunc("POST /api/v1/admin/initialize", h.initialize)
	mux.HandleFunc("POST /api/v1/admin/login", h.login)
	mux.HandleFunc("POST /api/v1/admin/logout", h.logout)
	mux.Handle("POST /api/v1/admin/password", h.authenticator.RequireAdmin(http.HandlerFunc(h.changePassword)))
}

func (h *AuthHandlers) Routes(router chi.Router) {
	router.Get("/api/v1/admin/status", h.status)
	router.Post("/api/v1/admin/initialize", h.initialize)
	router.Post("/api/v1/admin/login", h.login)
	router.Post("/api/v1/admin/logout", h.logout)
	router.With(h.authenticator.RequireAdmin).Post("/api/v1/admin/password", h.changePassword)
}

func (h *AuthHandlers) status(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.admin.IsInitialized(r.Context())
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"initialized": initialized})
}

func (h *AuthHandlers) initialize(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := h.admin.Initialize(r.Context(), request.Account, request.Password); err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"initialized": true})
}

func (h *AuthHandlers) login(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	session, err := h.admin.Login(r.Context(), request.Account, request.Password)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	setSessionCookies(w, session)
	writeJSON(w, http.StatusOK, map[string]string{"account": session.ActorID})
}

func (h *AuthHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.authenticator.Revoke(r); err != nil {
		writeAPIError(w, r, err)
		return
	}
	clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandlers) changePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := h.admin.ChangePassword(r.Context(), request.OldPassword, request.NewPassword); err != nil {
		writeAPIError(w, r, err)
		return
	}
	clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func setSessionCookies(w http.ResponseWriter, session auth.Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     auth.AdminCookieName,
		Value:    session.Token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    session.CSRFToken,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  session.ExpiresAt,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{auth.AdminCookieName, auth.CSRFCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: name == auth.AdminCookieName,
			SameSite: http.SameSiteStrictMode,
		})
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %v", errInvalidRequest, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errInvalidRequest
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	response := errorResponse{Code: "internal_error", Message: "服务器内部错误", RequestID: requestID(r)}
	switch {
	case errors.Is(err, auth.ErrAlreadyInitialized):
		status, response.Code, response.Message = http.StatusConflict, "already_initialized", "管理员已经初始化"
	case errors.Is(err, auth.ErrNotInitialized):
		status, response.Code, response.Message = http.StatusPreconditionRequired, "not_initialized", "管理员尚未初始化"
	case errors.Is(err, auth.ErrUnauthorized):
		status, response.Code, response.Message = http.StatusUnauthorized, "unauthorized", "账号或密码错误"
	case errors.Is(err, auth.ErrRateLimited):
		status, response.Code, response.Message, response.Retriable = http.StatusTooManyRequests, "rate_limited", "尝试次数过多", true
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, errInvalidRequest):
		status, response.Code, response.Message = http.StatusBadRequest, "invalid_request", "请求参数无效"
	case errors.Is(err, files.ErrForbidden):
		status, response.Code, response.Message = http.StatusForbidden, "forbidden", "没有操作权限"
	case errors.Is(err, files.ErrGone):
		status, response.Code, response.Message = http.StatusGone, "gone", "文件已删除"
	case errors.Is(err, files.ErrInvalidInput), errors.Is(err, files.ErrFolderCycle):
		status, response.Code, response.Message = http.StatusBadRequest, "invalid_request", "请求参数无效"
	case errors.Is(err, repository.ErrNotFound):
		status, response.Code, response.Message = http.StatusNotFound, "not_found", "资源不存在"
	}
	writeJSON(w, status, response)
}

func requestID(r *http.Request) string {
	if value := r.Header.Get("X-Request-ID"); value != "" {
		return value
	}
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "unavailable"
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
