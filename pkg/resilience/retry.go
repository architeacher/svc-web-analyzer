package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
)

const (
	attemptKey     = "attempt"
	attemptsKey    = "attempts"
	maxAttemptsKey = "max_attempts"
	operationKey   = "operation"
)

type (
	RetryConfig struct {
		MaxRetries      int
		BackoffStrategy backoff.Strategy
		RetryableErrors []error
	}

	RetryWrapper struct {
		config        RetryConfig
		logger        appLogger.Logger
		metricsClient metricsKit.Client
	}
)

func NewRetryWrapper(cfg RetryConfig, logger appLogger.Logger, metricsClient metricsKit.Client) *RetryWrapper {
	return &RetryWrapper{
		config:        cfg,
		logger:        logger,
		metricsClient: metricsClient,
	}
}

func (rw *RetryWrapper) Execute(ctx context.Context, operation string, fn func() (any, error)) (any, error) {
	var lastErr error

	for attempt := 0; attempt <= rw.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rw.config.BackoffStrategy.Backoff(attempt - 1)
			rw.logger.Debug().
				Str("operation", operation).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("retrying after backoff")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := fn()
		if err == nil {
			if attempt > 0 {
				rw.recordRetrySuccess(ctx, operation, uint(attempt))
			}

			return result, nil
		}

		lastErr = err

		if !rw.isRetryable(err) {
			rw.logger.Warn().
				Err(err).
				Str("operation", operation).
				Msg("non-retryable error encountered")

			return nil, err
		}

		if attempt < rw.config.MaxRetries {
			rw.logger.Warn().
				Err(err).
				Str("operation", operation).
				Int("attempt", attempt).
				Msg("operation failed, will retry")

			rw.recordRetryAttempt(ctx, operation, uint(attempt))
		}
	}

	rw.recordRetryFailure(ctx, operation, uint(rw.config.MaxRetries))

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (rw *RetryWrapper) isRetryable(err error) bool {
	if len(rw.config.RetryableErrors) == 0 {
		return true
	}

	for _, retryableErr := range rw.config.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

func (rw *RetryWrapper) recordRetryAttempt(ctx context.Context, operation string, attempt uint) {
	rw.metricsClient.Inc(
		ctx,
		ports.RetryAttemptTotal,
		int64(1),
		attribute.String(operationKey, operation),
		attribute.Int(attemptKey, int(attempt)),
	)
}

func (rw *RetryWrapper) recordRetrySuccess(ctx context.Context, operation string, attempts uint) {
	rw.metricsClient.Inc(
		ctx,
		ports.RetrySuccessTotal,
		int64(1),
		attribute.String(operationKey, operation),
		attribute.Int(attemptsKey, int(attempts)),
	)
}

func (rw *RetryWrapper) recordRetryFailure(ctx context.Context, operation string, maxAttempts uint) {
	rw.metricsClient.Inc(
		ctx,
		ports.RetryFailureTotal,
		int64(1),
		attribute.String(operationKey, operation),
		attribute.Int(maxAttemptsKey, int(maxAttempts)),
	)
}
