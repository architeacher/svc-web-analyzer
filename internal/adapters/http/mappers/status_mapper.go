package mappers

import (
	"github.com/architeacher/svc-web-analyzer/internal/adapters/http/handlers"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

func DomainDependencyStatusToHandler(status domain.DependencyCheckStatus) handlers.DependencyCheckStatus {
	switch status {
	case domain.DependencyCheckStatusHealthy:
		return handlers.DependencyCheckStatusHealthy
	case domain.DependencyCheckStatusDegraded:
		return handlers.DependencyCheckStatusDegraded
	case domain.DependencyCheckStatusUnhealthy:
		return handlers.DependencyCheckStatusUnhealthy
	default:
		return handlers.DependencyCheckStatusUnknown
	}
}

func DomainLivenessStatusToHandler(status domain.LivenessResponseStatus) handlers.LivenessResponseStatus {
	switch status {
	case domain.LivenessResponseStatusAlive:
		return handlers.LivenessResponseStatusOK
	case domain.LivenessResponseStatusDegraded:
		return handlers.LivenessResponseStatusDEGRADED
	case domain.LivenessResponseStatusDead:
		return handlers.LivenessResponseStatusDOWN
	default:
		return handlers.LivenessResponseStatusDOWN
	}
}

func DomainReadinessStatusToHandler(status domain.ReadinessResponseStatus) handlers.ReadinessResponseStatus {
	switch status {
	case domain.ReadinessResponseStatusReady:
		return handlers.OK
	case domain.ReadinessResponseStatusDegraded:
		return handlers.DEGRADED
	case domain.ReadinessResponseStatusNotReady:
		return handlers.DOWN
	default:
		return handlers.DOWN
	}
}

func DomainHealthStatusToHandler(status domain.HealthResponseStatus) handlers.HealthResponseStatus {
	switch status {
	case domain.HealthResponseStatusHealthy:
		return handlers.HealthResponseStatusOK
	case domain.HealthResponseStatusDegraded:
		return handlers.HealthResponseStatusDEGRADED
	case domain.HealthResponseStatusUnhealthy:
		return handlers.HealthResponseStatusDOWN
	default:
		return handlers.HealthResponseStatusDOWN
	}
}
