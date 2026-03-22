package rest

import "github.com/halyph/page-analyzer/internal/domain"

// AnalyzeRequest represents the request body for POST /api/analyze
type AnalyzeRequest struct {
	URL     string          `json:"url"`
	Options *AnalyzeOptions `json:"options,omitempty"`
}

// AnalyzeOptions configures the analysis
type AnalyzeOptions struct {
	CheckLinks string `json:"check_links,omitempty"` // "sync", "async", "disabled"
	MaxLinks   int    `json:"max_links,omitempty"`   // Default: 10000
}

// AnalyzeResponse represents the response for POST /api/analyze
type AnalyzeResponse struct {
	*domain.AnalysisResult
}

// JobResponse represents the response for GET /api/jobs/:id
type JobResponse struct {
	ID          string                  `json:"id"`
	Status      domain.LinkCheckStatus  `json:"status"`
	URLs        []string                `json:"urls,omitempty"`
	BaseURL     string                  `json:"base_url,omitempty"`
	Result      *domain.LinkCheckResult `json:"result,omitempty"`
	CreatedAt   string                  `json:"created_at"`
	StartedAt   *string                 `json:"started_at,omitempty"`
	CompletedAt *string                 `json:"completed_at,omitempty"`
}

// HealthResponse represents the response for GET /api/health
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	GitCommit string            `json:"git_commit,omitempty"`
	Uptime    string            `json:"uptime"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}
