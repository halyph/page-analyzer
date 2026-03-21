package server

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/halyph/page-analyzer/internal/presentation/rest"
)

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(handler *rest.Handler, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(Recovery(logger))
	r.Use(Logger(logger))
	r.Use(CORS)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Health check
		r.Get("/health", handler.HandleHealth)

		// Analysis
		r.Post("/analyze", handler.HandleAnalyze)

		// Jobs (link check results)
		r.Get("/jobs/{id}", handler.HandleGetJob)
	})

	return r
}
