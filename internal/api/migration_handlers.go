package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sunnyhmz7010/CList/internal/auth"
	"github.com/sunnyhmz7010/CList/internal/migration"
)

type MigrationHandlers struct{ service *migration.Service }

func NewMigrationHandlers(service *migration.Service) *MigrationHandlers {
	return &MigrationHandlers{service: service}
}

func (h *MigrationHandlers) Start(w http.ResponseWriter, r *http.Request) {
	var input migration.MigrationInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeAPIError(w, r, err)
		return
	}
	input.Actor, _ = auth.ActorFromContext(r.Context())
	input.IdempotencyKey = r.Header.Get("Idempotency-Key")
	job, err := h.service.Start(r.Context(), input)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *MigrationHandlers) GetJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.GetJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}
