package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/halyph/page-analyzer/internal/domain"
)

//go:embed templates/* static/*
var content embed.FS

// Handler holds dependencies for web UI handlers
type Handler struct {
	analyzer  domain.Analyzer
	logger    *slog.Logger
	tmpl      *template.Template
	version   string
	gitCommit string
}

// NewHandler creates a new web UI handler
func NewHandler(analyzer domain.Analyzer, logger *slog.Logger, version, gitCommit string) (*Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Parse templates
	tmpl, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Handler{
		analyzer:  analyzer,
		logger:    logger,
		tmpl:      tmpl,
		version:   version,
		gitCommit: gitCommit,
	}, nil
}

// StaticFS returns an http.Handler for serving embedded static files
func (h *Handler) StaticFS() http.Handler {
	// Create a sub filesystem for the "static" directory
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		h.logger.Error("failed to create static sub filesystem", "error", err)
		return http.NotFoundHandler()
	}

	return http.FileServer(http.FS(staticFS))
}

// HandleIndex shows the home page with form
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data := h.baseData("Home")
	data["ShowIndex"] = true

	if err := h.tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		h.logger.Error("failed to render template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleAnalyze processes the form submission
func (h *Handler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		h.renderError(w, "Failed to parse form", err)
		return
	}

	url := r.FormValue("url")
	if url == "" {
		h.renderError(w, "URL is required", nil)
		return
	}

	// Build analysis request
	req := domain.AnalysisRequest{
		URL:     url,
		Options: domain.DefaultOptions(),
	}

	// Check if link checking is requested
	checkLinks := r.FormValue("checkLinks")
	if checkLinks == "on" {
		req.Options.CheckLinks = domain.LinkCheckAsync
	} else {
		req.Options.CheckLinks = domain.LinkCheckDisabled
	}

	// Parse max links
	if maxLinksStr := r.FormValue("maxLinks"); maxLinksStr != "" {
		if maxLinks, err := strconv.Atoi(maxLinksStr); err == nil && maxLinks > 0 {
			req.Options.MaxLinks = maxLinks
		}
	}

	// Perform analysis
	result, err := h.analyzer.Analyze(r.Context(), req)
	if err != nil {
		h.renderError(w, "Analysis failed", err)
		return
	}

	// Render result
	h.renderResult(w, result)
}

// baseData creates base template data with version info
func (h *Handler) baseData(title string) map[string]interface{} {
	gitCommit := h.gitCommit
	if len(gitCommit) > 7 {
		gitCommit = gitCommit[:7]
	}

	return map[string]interface{}{
		"Title":     title,
		"Version":   h.version,
		"GitCommit": gitCommit,
	}
}

// renderResult renders the result page
func (h *Handler) renderResult(w http.ResponseWriter, result *domain.AnalysisResult) {
	// Sort links alphabetically for better readability
	if result.Links.Internal != nil {
		sort.Strings(result.Links.Internal)
	}
	if result.Links.External != nil {
		sort.Strings(result.Links.External)
	}

	// Convert to JSON for display
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		jsonBytes = []byte("{}")
	}

	data := h.baseData("Results")
	data["Result"] = result
	data["JSON"] = string(jsonBytes)

	if err := h.tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		h.logger.Error("failed to render template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderError renders an error page
func (h *Handler) renderError(w http.ResponseWriter, message string, err error) {
	h.logger.Error("web error", "message", message, "error", err)

	errorMsg := message
	if err != nil {
		errorMsg = err.Error()
	}

	data := h.baseData("Error")
	data["Error"] = errorMsg

	w.WriteHeader(http.StatusBadRequest)
	if err := h.tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		h.logger.Error("failed to render error template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
