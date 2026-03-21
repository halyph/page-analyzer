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
		Status:  "ok",
		Version: "dev", // TODO: inject version from build
		Uptime:  uptime.String(),
		Checks:  make(map[string]string),
	}

	// Could add more health checks here:
	// - Database connectivity
	// - Redis connectivity
	// - Disk space
	// - Memory usage

	h.respondJSON(w, http.StatusOK, resp)
}
