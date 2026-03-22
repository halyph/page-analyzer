package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLinkAnalysis_Counts(t *testing.T) {
	la := LinkAnalysis{
		Internal: []string{"https://example.com/page1", "https://example.com/page2"},
		External: []string{"https://other.com"},
	}

	assert.Equal(t, 2, la.InternalCount())
	assert.Equal(t, 1, la.ExternalCount())
	assert.Equal(t, 3, la.TotalCollected())
}

func TestLinkAnalysis_Empty(t *testing.T) {
	la := LinkAnalysis{}

	assert.Equal(t, 0, la.InternalCount())
	assert.Equal(t, 0, la.ExternalCount())
	assert.Equal(t, 0, la.TotalCollected())
}

func TestLinkCheckResult_InaccessibleCount(t *testing.T) {
	result := LinkCheckResult{
		Checked:    10,
		Accessible: 8,
		Inaccessible: []LinkError{
			{URL: "https://broken1.com", Reason: "404"},
			{URL: "https://broken2.com", Reason: "timeout"},
		},
	}

	assert.Equal(t, 2, result.InaccessibleCount())
}

func TestLinkCheckResult_SuccessRate(t *testing.T) {
	tests := []struct {
		name   string
		result LinkCheckResult
		want   float64
	}{
		{
			name: "perfect score",
			result: LinkCheckResult{
				Checked:    10,
				Accessible: 10,
			},
			want: 1.0,
		},
		{
			name: "half success",
			result: LinkCheckResult{
				Checked:    10,
				Accessible: 5,
			},
			want: 0.5,
		},
		{
			name: "no links checked",
			result: LinkCheckResult{
				Checked:    0,
				Accessible: 0,
			},
			want: 0.0,
		},
		{
			name: "80% success",
			result: LinkCheckResult{
				Checked:    100,
				Accessible: 80,
			},
			want: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.SuccessRate()
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestLinkError_Error(t *testing.T) {
	tests := []struct {
		name         string
		linkError    LinkError
		wantContains []string
	}{
		{
			name: "with status code",
			linkError: LinkError{
				URL:        "https://example.com",
				StatusCode: 404,
				Reason:     "Not Found",
			},
			wantContains: []string{"https://example.com", "404", "Not Found"},
		},
		{
			name: "without status code",
			linkError: LinkError{
				URL:    "https://example.com",
				Reason: "timeout",
			},
			wantContains: []string{"https://example.com", "timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.linkError.Error()
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
		})
	}
}

func TestLinkCheckJob_StatusMethods(t *testing.T) {
	tests := []struct {
		name           string
		status         LinkCheckStatus
		wantComplete   bool
		wantPending    bool
		wantInProgress bool
	}{
		{
			name:           "pending",
			status:         LinkCheckPending,
			wantComplete:   false,
			wantPending:    true,
			wantInProgress: false,
		},
		{
			name:           "in progress",
			status:         LinkCheckInProgress,
			wantComplete:   false,
			wantPending:    false,
			wantInProgress: true,
		},
		{
			name:           "completed",
			status:         LinkCheckCompleted,
			wantComplete:   true,
			wantPending:    false,
			wantInProgress: false,
		},
		{
			name:           "failed",
			status:         LinkCheckFailed,
			wantComplete:   true,
			wantPending:    false,
			wantInProgress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &LinkCheckJob{Status: tt.status}

			assert.Equal(t, tt.wantComplete, job.IsComplete())
			assert.Equal(t, tt.wantPending, job.IsPending())
			assert.Equal(t, tt.wantInProgress, job.IsInProgress())
		})
	}
}

func TestLinkCheckJob_Age(t *testing.T) {
	now := time.Now()
	job := &LinkCheckJob{
		CreatedAt: now.Add(-5 * time.Second),
	}

	age := job.Age()
	assert.InDelta(t, 5*time.Second, age, float64(time.Second))
}

func TestNewLinkCheckJob(t *testing.T) {
	id := "test-123"
	urls := []string{"https://example.com", "https://other.com"}
	baseURL := "https://source.com"

	job := NewLinkCheckJob(id, urls, baseURL)

	assert.Equal(t, id, job.ID)
	assert.Len(t, job.URLs, len(urls))
	assert.Equal(t, baseURL, job.BaseURL)
	assert.Equal(t, LinkCheckPending, job.Status)
	assert.False(t, job.CreatedAt.IsZero())
	assert.True(t, job.IsPending())
}

func TestLinkCheckStatusConstants(t *testing.T) {
	statuses := []LinkCheckStatus{
		LinkCheckPending,
		LinkCheckInProgress,
		LinkCheckCompleted,
		LinkCheckFailed,
	}

	// Ensure all statuses are unique
	seen := make(map[LinkCheckStatus]bool)
	for _, status := range statuses {
		assert.False(t, seen[status], "duplicate LinkCheckStatus: %s", status)
		seen[status] = true
	}
}
