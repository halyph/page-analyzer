package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Common error variables
var (
	ErrInvalidURL       = errors.New("invalid URL")
	ErrEmptyURL         = errors.New("URL cannot be empty")
	ErrUnsupportedScheme = errors.New("unsupported URL scheme (must be http or https)")
	ErrFetchFailed      = errors.New("failed to fetch URL")
	ErrTimeout          = errors.New("request timeout")
	ErrBodyTooLarge     = errors.New("response body exceeds maximum size")
	ErrTooManyLinks     = errors.New("page contains too many links")
	ErrCacheMiss        = errors.New("cache miss")
	ErrCacheUnavailable = errors.New("cache unavailable")
)

// AnalysisError represents an error that occurred during webpage analysis
// It includes an HTTP status code for proper error reporting
type AnalysisError struct {
	StatusCode  int    // HTTP status code (e.g., 404, 500, 502)
	Description string // Human-readable error description
	Cause       error  // Underlying error (for error wrapping)
}

// Error implements the error interface
func (e *AnalysisError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP %d: %s: %v", e.StatusCode, e.Description, e.Cause)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Description)
}

// Unwrap implements error unwrapping for Go 1.13+ error chains
func (e *AnalysisError) Unwrap() error {
	return e.Cause
}

// NewAnalysisError creates a new AnalysisError with the given status and description
func NewAnalysisError(statusCode int, description string) *AnalysisError {
	return &AnalysisError{
		StatusCode:  statusCode,
		Description: description,
	}
}

// WrapAnalysisError wraps an error with additional context and status code
func WrapAnalysisError(statusCode int, description string, cause error) *AnalysisError {
	return &AnalysisError{
		StatusCode:  statusCode,
		Description: description,
		Cause:       cause,
	}
}

// Common error constructors for convenience

// ErrInvalidURLWithReason creates an error for invalid URLs
func ErrInvalidURLWithReason(url string, reason error) *AnalysisError {
	return WrapAnalysisError(
		http.StatusBadRequest,
		fmt.Sprintf("invalid URL: %s", url),
		reason,
	)
}

// ErrFetchFailedWithStatus creates an error for failed HTTP requests
func ErrFetchFailedWithStatus(url string, statusCode int, statusText string) *AnalysisError {
	return NewAnalysisError(
		http.StatusBadGateway,
		fmt.Sprintf("failed to fetch %s: HTTP %d %s", url, statusCode, statusText),
	)
}

// ErrTimeoutWithContext creates an error for timeouts
func ErrTimeoutWithContext(url string, duration string) *AnalysisError {
	return NewAnalysisError(
		http.StatusGatewayTimeout,
		fmt.Sprintf("timeout fetching %s after %s", url, duration),
	)
}

// ErrConnectionFailed creates an error for connection failures
func ErrConnectionFailed(url string, cause error) *AnalysisError {
	return WrapAnalysisError(
		http.StatusBadGateway,
		fmt.Sprintf("connection failed: %s", url),
		cause,
	)
}

// ErrBodyTooLargeWithSize creates an error when response body exceeds limit
func ErrBodyTooLargeWithSize(url string, size int64, limit int64) *AnalysisError {
	return NewAnalysisError(
		http.StatusBadGateway,
		fmt.Sprintf("response body too large: %s (size: %d bytes, limit: %d bytes)", url, size, limit),
	)
}

// ErrParsingFailed creates an error for HTML parsing failures
func ErrParsingFailed(url string, cause error) *AnalysisError {
	return WrapAnalysisError(
		http.StatusUnprocessableEntity,
		fmt.Sprintf("failed to parse HTML: %s", url),
		cause,
	)
}

// IsAnalysisError checks if an error is an AnalysisError
func IsAnalysisError(err error) bool {
	var ae *AnalysisError
	return errors.As(err, &ae)
}

// GetStatusCode extracts the HTTP status code from an error
// Returns 500 if the error is not an AnalysisError
func GetStatusCode(err error) int {
	var ae *AnalysisError
	if errors.As(err, &ae) {
		return ae.StatusCode
	}
	return http.StatusInternalServerError
}
