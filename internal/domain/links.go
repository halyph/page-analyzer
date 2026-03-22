package domain

import (
	"fmt"
	"sync"
	"time"
)

// LinkAnalysis contains the results of analyzing links in a webpage
type LinkAnalysis struct {
	Internal    []string         `json:"internal"`               // Collected internal URLs
	External    []string         `json:"external"`               // Collected external URLs
	TotalFound  int              `json:"total_found"`            // Total discovered (may exceed max)
	Truncated   bool             `json:"truncated"`              // Hit MaxLinks limit
	CheckJobID  string           `json:"check_job_id,omitempty"` // Link check job ID (async)
	CheckStatus LinkCheckStatus  `json:"check_status"`           // Link check status
	CheckResult *LinkCheckResult `json:"check_result,omitempty"` // Link check results
}

// InternalCount returns the number of internal links
func (la LinkAnalysis) InternalCount() int {
	return len(la.Internal)
}

// ExternalCount returns the number of external links
func (la LinkAnalysis) ExternalCount() int {
	return len(la.External)
}

// TotalCollected returns the total number of links collected (may be truncated)
func (la LinkAnalysis) TotalCollected() int {
	return len(la.Internal) + len(la.External)
}

// LinkCheckStatus represents the state of a link checking job
type LinkCheckStatus string

const (
	LinkCheckPending    LinkCheckStatus = "pending"
	LinkCheckInProgress LinkCheckStatus = "in_progress"
	LinkCheckCompleted  LinkCheckStatus = "completed"
	LinkCheckFailed     LinkCheckStatus = "failed"
)

// LinkCheckResult contains the results of checking link accessibility
type LinkCheckResult struct {
	Checked      int         `json:"checked"`      // Number of links checked
	Accessible   int         `json:"accessible"`   // Number of accessible links
	Inaccessible []LinkError `json:"inaccessible"` // Links that failed checks
	Duration     string      `json:"duration"`     // Time taken (e.g., "2.5s")
	CompletedAt  time.Time   `json:"completed_at"` // When check completed
}

// InaccessibleCount returns the number of inaccessible links
func (lcr LinkCheckResult) InaccessibleCount() int {
	return len(lcr.Inaccessible)
}

// SuccessRate returns the percentage of accessible links (0.0-1.0)
func (lcr LinkCheckResult) SuccessRate() float64 {
	if lcr.Checked == 0 {
		return 0.0
	}
	return float64(lcr.Accessible) / float64(lcr.Checked)
}

// LinkError describes a link that failed accessibility check
type LinkError struct {
	URL        string `json:"url"`                   // The URL that failed
	StatusCode int    `json:"status_code,omitempty"` // HTTP status code (if applicable)
	Reason     string `json:"reason"`                // Reason for failure
}

// CachedLinkCheck represents a cached result of checking a single link
type CachedLinkCheck struct {
	URL        string `json:"url"`                   // The URL that was checked
	Accessible bool   `json:"accessible"`            // Whether the link is accessible
	StatusCode int    `json:"status_code,omitempty"` // HTTP status code (if not accessible)
	Reason     string `json:"reason,omitempty"`      // Reason for failure (if not accessible)
	CheckedAt  int64  `json:"checked_at"`            // Unix timestamp of when check occurred
}

// Error implements the error interface
func (le LinkError) Error() string {
	if le.StatusCode > 0 {
		return fmt.Sprintf("%s: HTTP %d - %s", le.URL, le.StatusCode, le.Reason)
	}
	return le.URL + ": " + le.Reason
}

// LinkCheckJob represents an asynchronous link checking job
type LinkCheckJob struct {
	Mu          sync.RWMutex     // Protects concurrent access to job fields
	ID          string           // Unique job identifier
	URLs        []string         // URLs to check
	BaseURL     string           // Context: the page being analyzed
	Result      *LinkCheckResult // Results (nil until completed)
	Status      LinkCheckStatus  // Current status
	CreatedAt   time.Time        // When job was created
	StartedAt   *time.Time       // When processing started
	CompletedAt *time.Time       // When processing completed
	Error       string           // Error message if failed
}

// IsComplete returns true if the job has finished (success or failure)
func (j *LinkCheckJob) IsComplete() bool {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.Status == LinkCheckCompleted || j.Status == LinkCheckFailed
}

// IsPending returns true if the job is waiting to be processed
func (j *LinkCheckJob) IsPending() bool {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.Status == LinkCheckPending
}

// IsInProgress returns true if the job is currently being processed
func (j *LinkCheckJob) IsInProgress() bool {
	return j.Status == LinkCheckInProgress
}

// Age returns how long ago the job was created
func (j *LinkCheckJob) Age() time.Duration {
	return time.Since(j.CreatedAt)
}

// NewLinkCheckJob creates a new link check job
func NewLinkCheckJob(id string, urls []string, baseURL string) *LinkCheckJob {
	return &LinkCheckJob{
		ID:        id,
		URLs:      urls,
		BaseURL:   baseURL,
		Status:    LinkCheckPending,
		CreatedAt: time.Now(),
	}
}
