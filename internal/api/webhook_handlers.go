package api

import "net/http"

type WebhookHandlers struct{ handler http.Handler }

func NewWebhookHandlers(handler http.Handler) *WebhookHandlers {
	return &WebhookHandlers{handler: handler}
}

func (h *WebhookHandlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}
