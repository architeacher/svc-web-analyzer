package domain

type (
	DependencyCheckStatus string

	LivenessResponseStatus string

	ReadinessResponseStatus string

	HealthResponseStatus string
)

const (
	DependencyCheckStatusHealthy   DependencyCheckStatus = "healthy"
	DependencyCheckStatusDegraded  DependencyCheckStatus = "degraded"
	DependencyCheckStatusUnhealthy DependencyCheckStatus = "unhealthy"
)

const (
	LivenessResponseStatusAlive    LivenessResponseStatus = "alive"
	LivenessResponseStatusDegraded LivenessResponseStatus = "degraded"
	LivenessResponseStatusDead     LivenessResponseStatus = "dead"
)

const (
	ReadinessResponseStatusReady    ReadinessResponseStatus = "ready"
	ReadinessResponseStatusDegraded ReadinessResponseStatus = "degraded"
	ReadinessResponseStatusNotReady ReadinessResponseStatus = "not_ready"
)

const (
	HealthResponseStatusHealthy   HealthResponseStatus = "healthy"
	HealthResponseStatusDegraded  HealthResponseStatus = "degraded"
	HealthResponseStatusUnhealthy HealthResponseStatus = "unhealthy"
)
