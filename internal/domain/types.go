package domain

import (
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/handlers"
)

type (
	// DependencyStatus represents the health status of a dependency
	DependencyStatus struct {
		Status       handlers.DependencyCheckStatus
		ResponseTime float32
		LastChecked  time.Time
		Error        string
	}

	// LivenessResult contains liveness check results
	LivenessResult struct {
		OverallStatus handlers.LivenessResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
	}

	// ReadinessResult contains readiness check results
	ReadinessResult struct {
		OverallStatus handlers.ReadinessResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
	}

	// HealthResult contains comprehensive health check results
	HealthResult struct {
		OverallStatus handlers.HealthResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
		Uptime        float32
	}
)
