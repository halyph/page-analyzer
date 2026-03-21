package rest

import (
	"encoding/json"
	"net/http"

	"github.com/halyph/page-analyzer/internal/domain"
)

// HandleAnalyze handles POST /api/analyze
func (h *Handler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate URL
	if req.URL == "" {
		h.respondError(w, http.StatusBadRequest, "url is required", nil)
		return
	}

	// Build analysis request
	analysisReq := domain.AnalysisRequest{
		URL:     req.URL,
		Options: domain.DefaultOptions(),
	}

	// Apply options if provided
	if req.Options != nil {
		if req.Options.MaxLinks > 0 {
			analysisReq.Options.MaxLinks = req.Options.MaxLinks
		}

		// Parse check links mode
		switch req.Options.CheckLinks {
		case "sync":
			analysisReq.Options.CheckLinks = domain.LinkCheckSync
		case "async":
			analysisReq.Options.CheckLinks = domain.LinkCheckAsync
		case "disabled":
			analysisReq.Options.CheckLinks = domain.LinkCheckDisabled
		default:
			// Default to async for REST API
			analysisReq.Options.CheckLinks = domain.LinkCheckAsync
		}
	} else {
		// Default to async with no link checking for REST API (fast response)
		analysisReq.Options.CheckLinks = domain.LinkCheckDisabled
	}

	// Perform analysis
	result, err := h.analyzer.Analyze(r.Context(), analysisReq)
	if err != nil {
		// Check if it's an analysis error with status code
		statusCode := http.StatusInternalServerError
		if analysisErr, ok := err.(*domain.AnalysisError); ok {
			statusCode = analysisErr.StatusCode
		}
		h.respondError(w, statusCode, "analysis failed", err)
		return
	}

	// Respond with result
	h.respondJSON(w, http.StatusOK, &AnalyzeResponse{
		AnalysisResult: result,
	})
}
