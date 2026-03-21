package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// HandleGetJob handles GET /api/jobs/:id
func (h *Handler) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		h.respondError(w, http.StatusBadRequest, "job id is required", nil)
		return
	}

	// Get job from link checker
	job, ok := h.linkChecker.GetJob(jobID)
	if !ok {
		h.respondError(w, http.StatusNotFound, "job not found", nil)
		return
	}

	// Convert to response format
	resp := JobResponse{
		ID:      job.ID,
		Status:  job.Status,
		URLs:    job.URLs,
		BaseURL: job.BaseURL,
		Result:  job.Result,
	}

	// Format timestamps
	resp.CreatedAt = job.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	if job.StartedAt != nil {
		startedStr := job.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &startedStr
	}
	if job.CompletedAt != nil {
		completedStr := job.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CompletedAt = &completedStr
	}

	h.respondJSON(w, http.StatusOK, resp)
}
