package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/domain"
)

// Handler holds dependencies for REST handlers
type Handler struct {
	analyzer    domain.Analyzer
	linkChecker *analyzer.LinkCheckWorkerPool
	logger      *slog.Logger
}

// NewHandler creates a new REST API handler
func NewHandler(analyzer domain.Analyzer, linkChecker *analyzer.LinkCheckWorkerPool, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		analyzer:    analyzer,
		linkChecker: linkChecker,
		logger:      logger,
	}
}

// respondJSON writes a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.Error("failed to encode JSON response", "error", err)
		}
	}
}

// respondError writes an error response
func (h *Handler) respondError(w http.ResponseWriter, statusCode int, message string, err error) {
	h.logger.Error("request error",
		"status", statusCode,
		"message", message,
		"error", err,
	)

	errMsg := message
	if err != nil {
		errMsg = err.Error()
	}

	h.respondJSON(w, statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: errMsg,
		Code:    statusCode,
	})
}
