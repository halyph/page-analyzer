package observability

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/halyph/page-analyzer/internal/cache"
)

// HealthChecker performs health checks on system components
type HealthChecker struct {
	cache cache.Cache
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cache cache.Cache) *HealthChecker {
	return &HealthChecker{
		cache: cache,
	}
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status string            `json:"status"` // "ok", "degraded", "error"
	Checks map[string]string `json:"checks,omitempty"`
	Memory *MemoryStats      `json:"memory,omitempty"`
}

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
}

// CheckLiveness performs a basic liveness check (fast, minimal logic)
func (h *HealthChecker) CheckLiveness(ctx context.Context) HealthStatus {
	return HealthStatus{
		Status: "ok",
	}
}

// CheckReadiness performs detailed readiness checks (slower, comprehensive)
func (h *HealthChecker) CheckReadiness(ctx context.Context) HealthStatus {
	checks := make(map[string]string)
	overallStatus := "ok"

	// Check cache connectivity
	cacheStatus := h.checkCache(ctx)
	checks["cache"] = cacheStatus
	if cacheStatus != "ok" {
		overallStatus = "degraded"
	}

	// Get memory stats
	memStats := h.getMemoryStats()

	return HealthStatus{
		Status: overallStatus,
		Checks: checks,
		Memory: memStats,
	}
}

// checkCache verifies cache is accessible
func (h *HealthChecker) checkCache(ctx context.Context) string {
	if h.cache == nil {
		return "no_cache"
	}

	// Try a simple cache operation with timeout
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Attempt to get a non-existent key (lightweight check)
	_, err := h.cache.GetHTML(ctx, "health-check-probe")
	if err == cache.ErrCacheMiss || err == nil {
		return "ok"
	}

	if err == cache.ErrCacheUnavailable {
		return "unavailable"
	}

	return fmt.Sprintf("error: %v", err)
}

// getMemoryStats collects memory usage statistics
func (h *HealthChecker) getMemoryStats() *MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &MemoryStats{
		AllocMB:      m.Alloc / 1024 / 1024,
		TotalAllocMB: m.TotalAlloc / 1024 / 1024,
		SysMB:        m.Sys / 1024 / 1024,
		NumGC:        m.NumGC,
	}
}
