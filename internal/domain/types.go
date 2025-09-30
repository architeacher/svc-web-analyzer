package domain

import (
	"time"
)

type (
	// DependencyStatus represents the health status of a dependency
	DependencyStatus struct {
		Status       DependencyCheckStatus
		ResponseTime float32
		LastChecked  time.Time
		Error        string
	}

	// LivenessResult contains liveness check results
	LivenessResult struct {
		OverallStatus LivenessResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
	}

	// ReadinessResult contains readiness check results
	ReadinessResult struct {
		OverallStatus ReadinessResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
	}

	// HealthResult contains comprehensive health check results
	HealthResult struct {
		OverallStatus HealthResponseStatus
		Storage       DependencyStatus
		Cache         DependencyStatus
		Queue         DependencyStatus
		Uptime        float32
	}
)
