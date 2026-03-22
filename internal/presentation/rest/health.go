package rest

import (
	"net/http"
	"time"
)

var startTime = time.Now()

// HandleHealth handles GET /api/health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime)

	resp := HealthResponse{
		Status:    "ok",
		Version:   h.version,
		GitCommit: h.gitCommit,
		Uptime:    uptime.String(),
	}

	h.respondJSON(w, http.StatusOK, resp)
}
