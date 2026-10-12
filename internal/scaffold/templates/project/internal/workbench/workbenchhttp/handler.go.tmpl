package workbenchhttp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
	"github.com/namezzy/goscaffoldforlevi/internal/workbench"
)

// Handler serves the workbench endpoint.
type Handler struct {
	svc *workbench.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *workbench.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the workbench endpoint. It carries no requirePermission:
// every signed-in principal gets an overview, just a narrower one — see
// workbench.Service.Overview.
func (h *Handler) Routes() func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", h.Overview)
	}
}

// Overview returns the caller's workbench snapshot.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.Overview(r.Context())
	if err != nil {
		httpx.WriteError(r.Context(), w, h.log, err)
		return
	}
	httpx.WriteJSON(r.Context(), w, h.log, http.StatusOK, toOverviewResponse(overview))
}
