package domain

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.Contains(t, got, tt.wantSubstr)
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
	assert.Equal(t, cause, unwrapped)

	// Test error chain with errors.Is
	assert.True(t, errors.Is(err, cause))
}

func TestNewAnalysisError(t *testing.T) {
	err := NewAnalysisError(http.StatusNotFound, "Page not found")

	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Equal(t, "Page not found", err.Description)
	assert.Nil(t, err.Cause)
}

func TestWrapAnalysisError(t *testing.T) {
	cause := errors.New("network timeout")
	err := WrapAnalysisError(http.StatusGatewayTimeout, "Request timed out", cause)

	assert.Equal(t, http.StatusGatewayTimeout, err.StatusCode)
	assert.Equal(t, cause, err.Cause)

	// Should be able to unwrap
	assert.True(t, errors.Is(err, cause))
}

func TestErrInvalidURLWithReason(t *testing.T) {
	cause := errors.New("missing scheme")
	err := ErrInvalidURLWithReason("example.com", cause)

	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Contains(t, err.Error(), "example.com")
	assert.True(t, errors.Is(err, cause))
}

func TestErrFetchFailedWithStatus(t *testing.T) {
	err := ErrFetchFailedWithStatus("https://example.com", 404, "Not Found")

	assert.Equal(t, http.StatusBadGateway, err.StatusCode)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "https://example.com")
	assert.Contains(t, errMsg, "404")
	assert.Contains(t, errMsg, "Not Found")
}

func TestErrTimeoutWithContext(t *testing.T) {
	err := ErrTimeoutWithContext("https://slow.com", "30s")

	assert.Equal(t, http.StatusGatewayTimeout, err.StatusCode)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "timeout")
	assert.Contains(t, errMsg, "https://slow.com")
	assert.Contains(t, errMsg, "30s")
}

func TestErrConnectionFailed(t *testing.T) {
	cause := errors.New("connection refused")
	err := ErrConnectionFailed("https://unreachable.com", cause)

	assert.Equal(t, http.StatusBadGateway, err.StatusCode)
	assert.True(t, errors.Is(err, cause))
}

func TestErrBodyTooLargeWithSize(t *testing.T) {
	err := ErrBodyTooLargeWithSize("https://huge.com", 50*1024*1024, 10*1024*1024)

	assert.Equal(t, http.StatusBadGateway, err.StatusCode)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "https://huge.com")
	assert.Contains(t, errMsg, "52428800")
	assert.Contains(t, errMsg, "10485760")
}

func TestErrParsingFailed(t *testing.T) {
	cause := errors.New("malformed HTML")
	err := ErrParsingFailed("https://broken.com", cause)

	assert.Equal(t, http.StatusUnprocessableEntity, err.StatusCode)
	assert.True(t, errors.Is(err, cause))
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
			assert.Equal(t, tt.want, got)
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
			assert.Equal(t, tt.want, got)
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
		assert.NotNil(t, err, "common error at index %d is nil", i)
		assert.NotEmpty(t, err.Error(), "common error at index %d has empty message", i)
	}

	// Test that they can be compared with errors.Is
	testErr := ErrInvalidURL
	assert.True(t, errors.Is(testErr, ErrInvalidURL))
}
