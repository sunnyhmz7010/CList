package api

import (
	"database/sql"
	"net/http"

	"github.com/sunnyhmz7010/CList/internal/health"
)

type HealthHandlers struct {
	checker *health.Checker
	db      *sql.DB
}

func NewHealthHandlers(checker *health.Checker, database *sql.DB) *HealthHandlers {
	return &HealthHandlers{checker: checker, db: database}
}

func (h *HealthHandlers) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
func (h *HealthHandlers) Ready(w http.ResponseWriter, r *http.Request) {
	result := h.checker.Ready(r.Context())
	writeJSON(w, result.Status(), result)
}
func (h *HealthHandlers) Diagnostics(w http.ResponseWriter, r *http.Request) {
	result := h.checker.Ready(r.Context())
	var pending int
	_ = h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM jobs WHERE state IN ('queued','running','retry_wait','cleanup_pending','uncertain')").Scan(&pending)
	writeJSON(w, http.StatusOK, map[string]any{"ready": result, "pending_jobs": pending})
}
