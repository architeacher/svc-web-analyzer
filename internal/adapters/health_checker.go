package adapters

import (
	"context"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

// HealthChecker implements the health checking functionality
type HealthChecker struct {
	startTime time.Time
}

// NewHealthChecker creates a new health checker instance
func NewHealthChecker() ports.HealthChecker {
	return &HealthChecker{
		startTime: time.Now(),
	}
}

// CheckReadiness performs readiness check and returns detailed results
func (h *HealthChecker) CheckReadiness(ctx context.Context) *domain.ReadinessResult {
	// Check all dependencies
	storageStatus := h.checkStorageHealth(ctx)
	cacheStatus := h.checkCacheHealth(ctx)
	queueStatus := h.checkQueueHealth(ctx)

	// Determine overall readiness status
	overallStatus := domain.ReadinessResponseStatusReady
	if storageStatus.Status == domain.DependencyCheckStatusUnhealthy {
		overallStatus = domain.ReadinessResponseStatusNotReady
	}

	return &domain.ReadinessResult{
		OverallStatus: overallStatus,
		Storage:       storageStatus,
		Cache:         cacheStatus,
		Queue:         queueStatus,
	}
}

// CheckLiveness performs liveness check and returns detailed results
func (h *HealthChecker) CheckLiveness(ctx context.Context) *domain.LivenessResult {
	// Check all dependencies
	storageStatus := h.checkStorageHealth(ctx)
	cacheStatus := h.checkCacheHealth(ctx)
	queueStatus := h.checkQueueHealth(ctx)

	// Determine overall liveness status
	overallStatus := domain.LivenessResponseStatusAlive
	if storageStatus.Status == domain.DependencyCheckStatusUnhealthy {
		overallStatus = domain.LivenessResponseStatusDead
	}

	return &domain.LivenessResult{
		OverallStatus: overallStatus,
		Storage:       storageStatus,
		Cache:         cacheStatus,
		Queue:         queueStatus,
	}
}

// CheckHealth performs a comprehensive health check and returns detailed results
func (h *HealthChecker) CheckHealth(ctx context.Context) *domain.HealthResult {
	// Check all dependencies
	storageStatus := h.checkStorageHealth(ctx)
	cacheStatus := h.checkCacheHealth(ctx)
	queueStatus := h.checkQueueHealth(ctx)

	// Determine overall status
	overallStatus := h.calculateOverallHealthStatus(storageStatus, cacheStatus, queueStatus)

	return &domain.HealthResult{
		OverallStatus: overallStatus,
		Storage:       storageStatus,
		Cache:         cacheStatus,
		Queue:         queueStatus,
		Uptime:        float32(time.Since(h.startTime).Seconds()),
	}
}

// calculateOverallHealthStatus determines overall health based on dependency statuses
func (h *HealthChecker) calculateOverallHealthStatus(storage, cache, queue domain.DependencyStatus) domain.HealthResponseStatus {
	// Storage is critical - if it's down, service is down
	if storage.Status == domain.DependencyCheckStatusUnhealthy {
		return domain.HealthResponseStatusUnhealthy
	}

	// Cache and queue failures are less critical, but we still consider them
	unhealthyCount := 0
	if cache.Status == domain.DependencyCheckStatusUnhealthy {
		unhealthyCount++
	}
	if queue.Status == domain.DependencyCheckStatusUnhealthy {
		unhealthyCount++
	}

	// If multiple non-critical dependencies are down, consider degraded
	if unhealthyCount >= 2 {
		return domain.HealthResponseStatusDegraded
	}

	// Service can still function without cache or queue individually
	return domain.HealthResponseStatusHealthy
}

// checkStorageHealth checks the health of the storage/database
func (h *HealthChecker) checkStorageHealth(ctx context.Context) domain.DependencyStatus {
	start := time.Now()

	// Simple health check that doesn't depend on application logic
	// In a real implementation, this could ping the database directly
	select {
	case <-time.After(10 * time.Millisecond): // Todo: Apply do the actual check instead of the simulation of the storage check
		// Continue
	case <-ctx.Done():
		return domain.DependencyStatus{
			Status:       domain.DependencyCheckStatusUnhealthy,
			ResponseTime: float32(time.Since(start).Milliseconds()),
			LastChecked:  time.Now(),
			Error:        "Health check timeout",
		}
	}

	responseTime := float32(time.Since(start).Milliseconds())

	// For now, assume storage is healthy
	// In a real implementation; you'd ping the database connection
	return domain.DependencyStatus{
		Status:       domain.DependencyCheckStatusHealthy,
		ResponseTime: responseTime,
		LastChecked:  time.Now(),
		Error:        "",
	}
}

// checkCacheHealth checks the health of the cache system
func (h *HealthChecker) checkCacheHealth(ctx context.Context) domain.DependencyStatus {
	start := time.Now()

	// Simple health check that doesn't depend on application logic
	select {
	case <-time.After(5 * time.Millisecond): // Todo: Use the cache method in the infrastructure layers.
		// Continue
	case <-ctx.Done():
		return domain.DependencyStatus{
			Status:       domain.DependencyCheckStatusUnhealthy,
			ResponseTime: float32(time.Since(start).Milliseconds()),
			LastChecked:  time.Now(),
			Error:        "Health check timeout",
		}
	}

	responseTime := float32(time.Since(start).Milliseconds())

	// For now, assume cache is healthy
	// In a real implementation; you'd ping the cache connection
	return domain.DependencyStatus{
		Status:       domain.DependencyCheckStatusHealthy,
		ResponseTime: responseTime,
		LastChecked:  time.Now(),
		Error:        "",
	}
}

// checkQueueHealth checks the health of any message queue system
func (h *HealthChecker) checkQueueHealth(ctx context.Context) domain.DependencyStatus {
	start := time.Now()

	// Add a small delay to simulate the actual queue check.
	select {
	case <-time.After(1 * time.Millisecond):
		// Todo: Continue with health check
	case <-ctx.Done():
		// Context cancelled
		return domain.DependencyStatus{
			Status:       domain.DependencyCheckStatusUnhealthy,
			ResponseTime: float32(time.Since(start).Milliseconds()),
			LastChecked:  time.Now(),
			Error:        "Health check timeout",
		}
	}

	// For now, we'll assume the queue is healthy since we don't have queue operations
	// In a real implementation, you'd check if your message queue (Redis, RabbitMQ, etc.) is responding
	responseTime := float32(time.Since(start).Milliseconds())

	return domain.DependencyStatus{
		Status:       domain.DependencyCheckStatusHealthy,
		ResponseTime: responseTime,
		LastChecked:  time.Now(),
		Error:        "",
	}
}
