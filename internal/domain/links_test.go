package domain

import (
	"testing"
	"time"
)

func TestLinkAnalysis_Counts(t *testing.T) {
	la := LinkAnalysis{
		Internal: []string{"https://example.com/page1", "https://example.com/page2"},
		External: []string{"https://other.com"},
	}

	if got := la.InternalCount(); got != 2 {
		t.Errorf("InternalCount() = %d, want 2", got)
	}

	if got := la.ExternalCount(); got != 1 {
		t.Errorf("ExternalCount() = %d, want 1", got)
	}

	if got := la.TotalCollected(); got != 3 {
		t.Errorf("TotalCollected() = %d, want 3", got)
	}
}

func TestLinkAnalysis_Empty(t *testing.T) {
	la := LinkAnalysis{}

	if got := la.InternalCount(); got != 0 {
		t.Errorf("InternalCount() = %d, want 0", got)
	}

	if got := la.ExternalCount(); got != 0 {
		t.Errorf("ExternalCount() = %d, want 0", got)
	}

	if got := la.TotalCollected(); got != 0 {
		t.Errorf("TotalCollected() = %d, want 0", got)
	}
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

	if got := result.InaccessibleCount(); got != 2 {
		t.Errorf("InaccessibleCount() = %d, want 2", got)
	}
}

func TestLinkCheckResult_SuccessRate(t *testing.T) {
	tests := []struct {
		name       string
		result     LinkCheckResult
		want       float64
		wantApprox bool
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
			want:       0.8,
			wantApprox: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.SuccessRate()
			if tt.wantApprox {
				if got < tt.want-0.01 || got > tt.want+0.01 {
					t.Errorf("SuccessRate() = %f, want approximately %f", got, tt.want)
				}
			} else {
				if got != tt.want {
					t.Errorf("SuccessRate() = %f, want %f", got, tt.want)
				}
			}
		})
	}
}

func TestLinkError_Error(t *testing.T) {
	tests := []struct {
		name      string
		linkError LinkError
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
				if !contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
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

			if got := job.IsComplete(); got != tt.wantComplete {
				t.Errorf("IsComplete() = %v, want %v", got, tt.wantComplete)
			}

			if got := job.IsPending(); got != tt.wantPending {
				t.Errorf("IsPending() = %v, want %v", got, tt.wantPending)
			}

			if got := job.IsInProgress(); got != tt.wantInProgress {
				t.Errorf("IsInProgress() = %v, want %v", got, tt.wantInProgress)
			}
		})
	}
}

func TestLinkCheckJob_Age(t *testing.T) {
	now := time.Now()
	job := &LinkCheckJob{
		CreatedAt: now.Add(-5 * time.Second),
	}

	age := job.Age()
	if age < 4*time.Second || age > 6*time.Second {
		t.Errorf("Age() = %v, expected approximately 5s", age)
	}
}

func TestNewLinkCheckJob(t *testing.T) {
	id := "test-123"
	urls := []string{"https://example.com", "https://other.com"}
	baseURL := "https://source.com"

	job := NewLinkCheckJob(id, urls, baseURL)

	if job.ID != id {
		t.Errorf("ID = %s, want %s", job.ID, id)
	}

	if len(job.URLs) != len(urls) {
		t.Errorf("len(URLs) = %d, want %d", len(job.URLs), len(urls))
	}

	if job.BaseURL != baseURL {
		t.Errorf("BaseURL = %s, want %s", job.BaseURL, baseURL)
	}

	if job.Status != LinkCheckPending {
		t.Errorf("Status = %s, want %s", job.Status, LinkCheckPending)
	}

	if job.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if job.IsPending() != true {
		t.Error("new job should be pending")
	}
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
		if seen[status] {
			t.Errorf("duplicate LinkCheckStatus: %s", status)
		}
		seen[status] = true
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
