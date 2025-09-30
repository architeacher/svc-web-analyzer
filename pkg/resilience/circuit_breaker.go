package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel/attribute"
)

const (
	circuitBreakerNameKey  = "circuit_breaker.name"
	circuitBreakerStateKey = "circuit_breaker.state"

	closed   = "closed"
	halfOpen = "half-open"
	open     = "open"
)

type (
	CircuitBreakerConfig struct {
		Name             string
		MaxRequests      uint
		Interval         time.Duration
		Timeout          time.Duration
		FailureThreshold float64
		OnStateChange    func(name string, from, to gobreaker.State)
	}

	CircuitBreakerWrapper struct {
		cb            *gobreaker.CircuitBreaker
		logger        appLogger.Logger
		metricsClient metricsKit.Client
		currentCtx    context.Context
		mu            sync.RWMutex
	}
)

func NewCircuitBreaker(cfg CircuitBreakerConfig, logger appLogger.Logger, metricsClient metricsKit.Client) *CircuitBreakerWrapper {
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: uint32(cfg.MaxRequests),
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)

			return counts.Requests >= 3 && failureRatio >= cfg.FailureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info().
				Str("breaker", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("circuit breaker state changed")

			if cfg.OnStateChange != nil {
				cfg.OnStateChange(name, from, to)
			}
		},
	}

	return &CircuitBreakerWrapper{
		cb:            gobreaker.NewCircuitBreaker(settings),
		logger:        logger,
		metricsClient: metricsClient,
	}
}

func (cbw *CircuitBreakerWrapper) Execute(ctx context.Context, fn func() (any, error)) (any, error) {
	currentState := cbw.cb.State()
	cbw.recordCircuitBreakerState(ctx, cbw.cb.Name(), currentState.String())

	result, err := cbw.cb.Execute(func() (any, error) {
		return fn()
	})

	if err != nil && errors.Is(err, gobreaker.ErrOpenState) {
		cbw.logger.Warn().
			Str("breaker", cbw.cb.Name()).
			Msg("circuit breaker is open, request rejected")

		return nil, err
	}

	return result, err
}

func (cbw *CircuitBreakerWrapper) State() gobreaker.State {
	return cbw.cb.State()
}

func (cbw *CircuitBreakerWrapper) Name() string {
	return cbw.cb.Name()
}

func (cbw *CircuitBreakerWrapper) recordCircuitBreakerState(ctx context.Context, name, state string) {
	stateValue := int64(0)

	switch state {
	case closed:
		stateValue = 0
	case halfOpen:
		stateValue = 1
	case open:
		stateValue = 2
	}

	cbw.metricsClient.Inc(
		ctx,
		ports.CircuitBreakerState,
		stateValue,
		attribute.String(circuitBreakerNameKey, name),
		attribute.String(circuitBreakerStateKey, state),
	)
}
