package rest

import (
	"encoding/json"
	"net/http"
)

// HandleHealthLive handles GET /api/health/live (liveness probe)
func (h *Handler) HandleHealthLive(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker == nil {
		// Fallback to basic health check
		h.HandleHealth(w, r)
		return
	}

	status := h.healthChecker.CheckLiveness(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if status.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(status)
}

// HandleHealthReady handles GET /api/health/ready (readiness probe)
func (h *Handler) HandleHealthReady(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker == nil {
		// Fallback to basic health check
		h.HandleHealth(w, r)
		return
	}

	status := h.healthChecker.CheckReadiness(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if status.Status == "error" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if status.Status == "degraded" {
		w.WriteHeader(http.StatusOK) // Still ready, but degraded
	}

	_ = json.NewEncoder(w).Encode(status)
}
