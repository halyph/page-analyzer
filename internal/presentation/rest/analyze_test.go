package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
)

// mockAnalyzer implements domain.Analyzer for testing
type mockAnalyzer struct {
	analyzeFunc func(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error)
}

func (m *mockAnalyzer) Analyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, req)
	}
	return &domain.AnalysisResult{
		URL:         req.URL,
		HTMLVersion: "HTML5",
		Title:       "Test Page",
	}, nil
}

func TestHandleAnalyze_TrimURL(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "url_with_leading_space",
			inputURL:    " https://example.com",
			expectedURL: "https://example.com",
		},
		{
			name:        "url_with_trailing_space",
			inputURL:    "https://example.com ",
			expectedURL: "https://example.com",
		},
		{
			name:        "url_with_both_spaces",
			inputURL:    " https://example.com ",
			expectedURL: "https://example.com",
		},
		{
			name:        "url_with_tabs",
			inputURL:    "\thttps://example.com\t",
			expectedURL: "https://example.com",
		},
		{
			name:        "url_with_newlines",
			inputURL:    "\nhttps://example.com\n",
			expectedURL: "https://example.com",
		},
		{
			name:        "url_without_whitespace",
			inputURL:    "https://example.com",
			expectedURL: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock analyzer that captures the URL
			var capturedURL string
			mockAnalyzer := &mockAnalyzer{
				analyzeFunc: func(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
					capturedURL = req.URL
					return &domain.AnalysisResult{
						URL:         req.URL,
						HTMLVersion: "HTML5",
						Title:       "Test",
					}, nil
				},
			}

			// Create handler
			logger := slog.Default()
			handler := NewHandler(mockAnalyzer, nil, nil, logger, "test", "test")

			// Create request
			reqBody := AnalyzeRequest{
				URL: tt.inputURL,
			}
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handler.HandleAnalyze(w, req)

			// Assert response
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedURL, capturedURL, "URL should be trimmed")
		})
	}
}

func TestHandleAnalyze_EmptyURL(t *testing.T) {
	tests := []struct {
		name     string
		inputURL string
	}{
		{
			name:     "empty_string",
			inputURL: "",
		},
		{
			name:     "only_spaces",
			inputURL: "   ",
		},
		{
			name:     "only_tabs",
			inputURL: "\t\t",
		},
		{
			name:     "only_newlines",
			inputURL: "\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock analyzer
			mockAnalyzer := &mockAnalyzer{}

			// Create handler
			logger := slog.Default()
			handler := NewHandler(mockAnalyzer, nil, nil, logger, "test", "test")

			// Create request
			reqBody := AnalyzeRequest{
				URL: tt.inputURL,
			}
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handler.HandleAnalyze(w, req)

			// Assert response
			assert.Equal(t, http.StatusBadRequest, w.Code)

			var errResp ErrorResponse
			_ = json.NewDecoder(w.Body).Decode(&errResp)
			assert.Contains(t, errResp.Message, "url is required")
		})
	}
}
