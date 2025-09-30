package resilience

import (
	"context"
	"errors"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
)

const fallbackNameKey = "fallback.name"

type (
	FallbackConfig struct {
		Name     string
		Enabled  bool
		Fallback FallbackStrategy
	}

	FallbackWrapper struct {
		config        FallbackConfig
		logger        appLogger.Logger
		metricsClient metricsKit.Client
	}
)

func NewFallbackWrapper(
	cfg FallbackConfig,
	logger appLogger.Logger,
	metrics metricsKit.Client,
) *FallbackWrapper {
	return &FallbackWrapper{
		config:        cfg,
		logger:        logger,
		metricsClient: metrics,
	}
}

func (fw *FallbackWrapper) Execute(ctx context.Context, fn func() (any, error)) (any, error) {
	result, err := fn()
	if err == nil {
		return result, nil
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		fw.logger.Warn().
			Err(err).
			Str("fallback_name", fw.config.Name).
			Msg("context canceled or deadline exceeded, skipping fallback")

		return nil, err
	}

	if !fw.config.Enabled {
		fw.logger.Debug().
			Str("fallback_name", fw.config.Name).
			Msg("fallback is disabled")

		return nil, err
	}

	if fw.config.Fallback == nil {
		fw.logger.Debug().
			Str("fallback_name", fw.config.Name).
			Msg("no fallback strategy configured")

		return nil, err
	}

	fw.logger.Warn().
		Err(err).
		Str("fallback_name", fw.config.Name).
		Msg("primary operation failed, attempting fallback")

	fw.recordFallbackAttempt(ctx, fw.config.Name)

	fallbackResult, fallbackErr := fw.config.Fallback.Execute(ctx)
	if fallbackErr != nil {
		fw.logger.Error().
			Err(fallbackErr).
			Str("fallback_name", fw.config.Name).
			Msg("fallback strategy failed")

		fw.recordFallbackFailure(ctx, fw.config.Name)

		return nil, fmt.Errorf("fallback failed after primary error: %w", fallbackErr)
	}

	fw.logger.Info().
		Str("fallback_name", fw.config.Name).
		Msg("fallback strategy succeeded")

	fw.recordFallbackSuccess(ctx, fw.config.Name)

	return fallbackResult, nil
}

func (fw *FallbackWrapper) recordFallbackAttempt(ctx context.Context, name string) {
	fw.metricsClient.Inc(
		ctx,
		ports.FallbackAttemptTotal,
		int64(1),
		attribute.String(fallbackNameKey, name),
	)
}

func (fw *FallbackWrapper) recordFallbackFailure(ctx context.Context, name string) {
	fw.metricsClient.Inc(
		ctx,
		ports.FallbackFailureTotal,
		int64(1),
		attribute.String(fallbackNameKey, name),
	)
}

func (fw *FallbackWrapper) recordFallbackSuccess(ctx context.Context, name string) {
	fw.metricsClient.Inc(ctx,
		ports.FallbackSuccessTotal,
		int64(1),
		attribute.String(fallbackNameKey, name),
	)
}
