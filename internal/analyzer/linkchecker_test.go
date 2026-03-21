package analyzer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oivasiv/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLinkCheckWorkerPool(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	assert.NotNil(t, pool)
	assert.Equal(t, 20, pool.workers)
	assert.NotNil(t, pool.client)
	assert.NotNil(t, pool.jobs)
}

func TestNewLinkCheckWorkerPool_DefaultValues(t *testing.T) {
	config := LinkCheckConfig{}
	pool := NewLinkCheckWorkerPool(config)

	assert.Equal(t, 20, pool.workers)
	assert.Equal(t, 5*time.Second, pool.timeout)
	assert.Equal(t, 10*time.Minute, pool.maxAge)
}

func TestLinkCheckWorkerPool_Submit(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	urls := []string{"https://example.com", "https://example.org"}
	jobID := pool.Submit(urls, "https://example.com")

	assert.NotEmpty(t, jobID)

	job, ok := pool.GetJob(jobID)
	require.True(t, ok)
	assert.Equal(t, jobID, job.ID)
	assert.Equal(t, urls, job.URLs)
	assert.Equal(t, domain.LinkCheckPending, job.Status)
}

func TestLinkCheckWorkerPool_GetJob_NotFound(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	job, ok := pool.GetJob("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, job)
}

func TestLinkCheckWorkerPool_ProcessJob_AllAccessible(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	config.Timeout = 2 * time.Second
	pool := NewLinkCheckWorkerPool(config)
	pool.Start()
	defer pool.Stop()

	urls := []string{server.URL, server.URL + "/page1", server.URL + "/page2"}
	jobID := pool.Submit(urls, server.URL)

	// Wait for job to complete
	job, err := pool.WaitForJob(jobID, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, domain.LinkCheckCompleted, job.Status)
	assert.Equal(t, 3, job.Result.Checked)
	assert.Equal(t, 3, job.Result.Accessible)
	assert.Equal(t, 0, job.Result.InaccessibleCount())
}

func TestLinkCheckWorkerPool_ProcessJob_SomeInaccessible(t *testing.T) {
	// Create test server with mixed responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.URL.Path == "/500" {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	config.Timeout = 2 * time.Second
	pool := NewLinkCheckWorkerPool(config)
	pool.Start()
	defer pool.Stop()

	urls := []string{
		server.URL + "/ok",
		server.URL + "/404",
		server.URL + "/500",
	}
	jobID := pool.Submit(urls, server.URL)

	// Wait for job to complete
	job, err := pool.WaitForJob(jobID, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, domain.LinkCheckCompleted, job.Status)
	assert.Equal(t, 3, job.Result.Checked)
	assert.Equal(t, 1, job.Result.Accessible)
	assert.Equal(t, 2, job.Result.InaccessibleCount())

	// Check that broken links are recorded
	assert.Len(t, job.Result.Inaccessible, 2)
}

func TestLinkCheckWorkerPool_ProcessJob_Redirects(t *testing.T) {
	redirectCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" && redirectCount < 2 {
			redirectCount++
			http.Redirect(w, r, "/final", http.StatusFound)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	config.Timeout = 2 * time.Second
	pool := NewLinkCheckWorkerPool(config)
	pool.Start()
	defer pool.Stop()

	urls := []string{server.URL + "/redirect"}
	jobID := pool.Submit(urls, server.URL)

	job, err := pool.WaitForJob(jobID, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, domain.LinkCheckCompleted, job.Status)
	assert.Equal(t, 1, job.Result.Accessible)
}

func TestLinkCheckWorkerPool_FullQueue(t *testing.T) {
	config := DefaultLinkCheckConfig()
	config.QueueSize = 1
	pool := NewLinkCheckWorkerPool(config)
	// Don't start workers - queue will fill up

	// Fill the queue
	jobID1 := pool.Submit([]string{"https://example.com"}, "https://example.com")
	job1, ok := pool.GetJob(jobID1)
	require.True(t, ok)
	assert.Equal(t, domain.LinkCheckPending, job1.Status)

	// Try to submit when queue is full
	jobID2 := pool.Submit([]string{"https://example.org"}, "https://example.org")
	job2, ok := pool.GetJob(jobID2)
	require.True(t, ok)
	assert.Equal(t, domain.LinkCheckFailed, job2.Status)
	assert.Contains(t, job2.Result.Inaccessible[0].Reason, "queue_full")
}

func TestLinkCheckWorkerPool_WaitForJob_Timeout(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)
	// Don't start workers - job won't complete

	jobID := pool.Submit([]string{"https://example.com"}, "https://example.com")

	_, err := pool.WaitForJob(jobID, 200*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestLinkCheckWorkerPool_WaitForJob_NotFound(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	_, err := pool.WaitForJob("nonexistent", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")
}

func TestCheckLink_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	err := pool.checkLink(pool.ctx, server.URL)
	assert.NoError(t, err)
}

func TestCheckLink_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	err := pool.checkLink(pool.ctx, server.URL)
	assert.Error(t, err)
	assert.Equal(t, 404, extractStatusCode(err))
	assert.Contains(t, extractReason(err), "404")
}

func TestCheckLink_InvalidURL(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	err := pool.checkLink(pool.ctx, "ht!tp://invalid url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_url")
}

func TestCheckLink_AcceptsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer server.Close()

	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)

	// 3xx status codes should be considered accessible
	err := pool.checkLink(pool.ctx, server.URL)
	assert.NoError(t, err)
}

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "http error",
			err:      &httpError{StatusCode: 404},
			expected: 404,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some error"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := extractStatusCode(tt.err)
			assert.Equal(t, tt.expected, code)
		})
	}
}

func TestExtractReason(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "timeout",
			err:      fmt.Errorf("timeout: connection timeout"),
			expected: "timeout",
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("connection_refused: dial failed"),
			expected: "connection refused",
		},
		{
			name:     "http error",
			err:      &httpError{StatusCode: 404},
			expected: "HTTP 404",
		},
		{
			name:     "invalid url",
			err:      fmt.Errorf("invalid_url: bad format"),
			expected: "invalid URL",
		},
		{
			name:     "unknown error",
			err:      fmt.Errorf("something unexpected"),
			expected: "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := extractReason(tt.err)
			assert.Equal(t, tt.expected, reason)
		})
	}
}

func TestGenerateJobID(t *testing.T) {
	id1 := generateJobID()
	id2 := generateJobID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestDefaultLinkCheckConfig(t *testing.T) {
	config := DefaultLinkCheckConfig()

	assert.Equal(t, 20, config.Workers)
	assert.Equal(t, 100, config.QueueSize)
	assert.Equal(t, 5*time.Second, config.Timeout)
	assert.Equal(t, 10*time.Minute, config.JobMaxAge)
	assert.Equal(t, "PageAnalyzer/1.0", config.UserAgent)
}

func TestLinkCheckWorkerPool_Stop(t *testing.T) {
	config := DefaultLinkCheckConfig()
	pool := NewLinkCheckWorkerPool(config)
	pool.Start()

	// Stop should not panic
	pool.Stop()
}

func TestLinkCheckWorkerPool_GarbageCollection(t *testing.T) {
	config := DefaultLinkCheckConfig()
	config.JobMaxAge = 100 * time.Millisecond
	pool := NewLinkCheckWorkerPool(config)

	// Create an old job manually
	job := &domain.LinkCheckJob{
		ID:        "old-job",
		Status:    domain.LinkCheckCompleted,
		CreatedAt: time.Now().Add(-5 * time.Minute),
	}
	pool.results.Store("old-job", job)

	// Create a recent job
	recentJobID := pool.Submit([]string{"https://example.com"}, "https://example.com")

	// Run garbage collection
	pool.gcOldJobs()

	// Old job should be removed
	_, ok := pool.GetJob("old-job")
	assert.False(t, ok)

	// Recent job should still exist
	_, ok = pool.GetJob(recentJobID)
	assert.True(t, ok)
}
