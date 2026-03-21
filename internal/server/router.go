package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/halyph/page-analyzer/internal/presentation/rest"
	"github.com/halyph/page-analyzer/internal/presentation/web"
)

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(restHandler *rest.Handler, webHandler *web.Handler, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(Recovery(logger))
	r.Use(Logger(logger))
	r.Use(CORS)

	// Web UI routes
	r.Get("/", webHandler.HandleIndex)
	r.Post("/analyze", webHandler.HandleAnalyze)

	// Static files - serve from embedded FS
	r.Handle("/static/*", http.StripPrefix("/static", webHandler.StaticFS()))

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Health check
		r.Get("/health", restHandler.HandleHealth)

		// Analysis
		r.Post("/analyze", restHandler.HandleAnalyze)

		// Jobs (link check results)
		r.Get("/jobs/{id}", restHandler.HandleGetJob)
	})

	// 404 handler
	r.NotFound(webHandler.Handle404)

	return r
}
