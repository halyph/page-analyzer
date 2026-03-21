package domain

import (
	"errors"
	"net/http"
	"testing"
)

func TestAnalysisError_Error(t *testing.T) {
	tests := []struct {
		name       string
		err        *AnalysisError
		wantSubstr string
	}{
		{
			name: "simple error",
			err: &AnalysisError{
				StatusCode:  404,
				Description: "Not Found",
			},
			wantSubstr: "HTTP 404: Not Found",
		},
		{
			name: "error with cause",
			err: &AnalysisError{
				StatusCode:  502,
				Description: "Connection failed",
				Cause:       errors.New("connection refused"),
			},
			wantSubstr: "connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if !containsString(got, tt.wantSubstr) {
				t.Errorf("Error() = %q, want to contain %q", got, tt.wantSubstr)
			}
		})
	}
}

func TestAnalysisError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &AnalysisError{
		StatusCode:  500,
		Description: "Something went wrong",
		Cause:       cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test error chain with errors.Is
	if !errors.Is(err, cause) {
		t.Error("errors.Is() should find the cause in the error chain")
	}
}

func TestNewAnalysisError(t *testing.T) {
	err := NewAnalysisError(http.StatusNotFound, "Page not found")

	if err.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusNotFound)
	}

	if err.Description != "Page not found" {
		t.Errorf("Description = %s, want 'Page not found'", err.Description)
	}

	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestWrapAnalysisError(t *testing.T) {
	cause := errors.New("network timeout")
	err := WrapAnalysisError(http.StatusGatewayTimeout, "Request timed out", cause)

	if err.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusGatewayTimeout)
	}

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}

	// Should be able to unwrap
	if !errors.Is(err, cause) {
		t.Error("Should be able to unwrap to find cause")
	}
}

func TestErrInvalidURLWithReason(t *testing.T) {
	cause := errors.New("missing scheme")
	err := ErrInvalidURLWithReason("example.com", cause)

	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadRequest)
	}

	errMsg := err.Error()
	if !containsString(errMsg, "example.com") {
		t.Errorf("Error should contain URL, got: %s", errMsg)
	}

	if !errors.Is(err, cause) {
		t.Error("Should wrap the cause")
	}
}

func TestErrFetchFailedWithStatus(t *testing.T) {
	err := ErrFetchFailedWithStatus("https://example.com", 404, "Not Found")

	if err.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadGateway)
	}

	errMsg := err.Error()
	wantSubstrings := []string{"https://example.com", "404", "Not Found"}
	for _, want := range wantSubstrings {
		if !containsString(errMsg, want) {
			t.Errorf("Error should contain %q, got: %s", want, errMsg)
		}
	}
}

func TestErrTimeoutWithContext(t *testing.T) {
	err := ErrTimeoutWithContext("https://slow.com", "30s")

	if err.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusGatewayTimeout)
	}

	errMsg := err.Error()
	wantSubstrings := []string{"timeout", "https://slow.com", "30s"}
	for _, want := range wantSubstrings {
		if !containsString(errMsg, want) {
			t.Errorf("Error should contain %q, got: %s", want, errMsg)
		}
	}
}

func TestErrConnectionFailed(t *testing.T) {
	cause := errors.New("connection refused")
	err := ErrConnectionFailed("https://unreachable.com", cause)

	if err.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadGateway)
	}

	if !errors.Is(err, cause) {
		t.Error("Should wrap the cause")
	}
}

func TestErrBodyTooLargeWithSize(t *testing.T) {
	err := ErrBodyTooLargeWithSize("https://huge.com", 50*1024*1024, 10*1024*1024)

	if err.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadGateway)
	}

	errMsg := err.Error()
	wantSubstrings := []string{"https://huge.com", "52428800", "10485760"}
	for _, want := range wantSubstrings {
		if !containsString(errMsg, want) {
			t.Errorf("Error should contain %q, got: %s", want, errMsg)
		}
	}
}

func TestErrParsingFailed(t *testing.T) {
	cause := errors.New("malformed HTML")
	err := ErrParsingFailed("https://broken.com", cause)

	if err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want %d", err.StatusCode, http.StatusUnprocessableEntity)
	}

	if !errors.Is(err, cause) {
		t.Error("Should wrap the cause")
	}
}

func TestIsAnalysisError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "is AnalysisError",
			err:  NewAnalysisError(500, "error"),
			want: true,
		},
		{
			name: "is wrapped AnalysisError",
			err:  WrapAnalysisError(500, "error", errors.New("cause")),
			want: true,
		},
		{
			name: "is not AnalysisError",
			err:  errors.New("regular error"),
			want: false,
		},
		{
			name: "is nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAnalysisError(tt.err)
			if got != tt.want {
				t.Errorf("IsAnalysisError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "AnalysisError with 404",
			err:  NewAnalysisError(404, "not found"),
			want: 404,
		},
		{
			name: "AnalysisError with 500",
			err:  NewAnalysisError(500, "server error"),
			want: 500,
		},
		{
			name: "regular error",
			err:  errors.New("regular error"),
			want: 500, // Default to 500
		},
		{
			name: "nil error",
			err:  nil,
			want: 500, // Default to 500
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStatusCode(tt.err)
			if got != tt.want {
				t.Errorf("GetStatusCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCommonErrors(t *testing.T) {
	// Test that common error variables are defined
	commonErrors := []error{
		ErrInvalidURL,
		ErrEmptyURL,
		ErrUnsupportedScheme,
		ErrFetchFailed,
		ErrTimeout,
		ErrBodyTooLarge,
		ErrTooManyLinks,
		ErrCacheMiss,
		ErrCacheUnavailable,
	}

	for i, err := range commonErrors {
		if err == nil {
			t.Errorf("common error at index %d is nil", i)
		}
		if err.Error() == "" {
			t.Errorf("common error at index %d has empty message", i)
		}
	}

	// Test that they can be compared with errors.Is
	testErr := ErrInvalidURL
	if !errors.Is(testErr, ErrInvalidURL) {
		t.Error("errors.Is should work with common errors")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || containsString(s[1:], substr))))
}
