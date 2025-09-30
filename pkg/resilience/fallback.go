package resilience

import (
	"context"
	"fmt"

	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
)

type (
	FallbackStrategy interface {
		Execute(ctx context.Context) (any, error)
	}

	FallbackChain struct {
		primary   func(ctx context.Context) (any, error)
		fallbacks []FallbackStrategy
		logger    appLogger.Logger
	}

	StaticFallbackStrategy struct {
		data any
	}

	FunctionFallbackStrategy struct {
		fn func(ctx context.Context) (any, error)
	}
)

func NewFallbackChain(primary func(ctx context.Context) (any, error), logger appLogger.Logger) *FallbackChain {
	return &FallbackChain{
		primary:   primary,
		fallbacks: make([]FallbackStrategy, 0),
		logger:    logger,
	}
}

func NewStaticFallbackStrategy(data any) *StaticFallbackStrategy {
	return &StaticFallbackStrategy{data: data}
}

func NewFunctionFallbackStrategy(fn func(ctx context.Context) (any, error)) *FunctionFallbackStrategy {
	return &FunctionFallbackStrategy{fn: fn}
}

func (fc *FallbackChain) AddFallback(strategy FallbackStrategy) *FallbackChain {
	fc.fallbacks = append(fc.fallbacks, strategy)

	return fc
}

func (fc *FallbackChain) Execute(ctx context.Context) (any, error) {
	result, err := fc.primary(ctx)
	if err == nil {
		return result, nil
	}

	fc.logger.Warn().
		Err(err).
		Msg("primary operation failed, trying fallbacks")

	var lastErr error
	for index, fallback := range fc.fallbacks {
		fc.logger.Debug().
			Int("fallback_index", index).
			Msg("attempting fallback strategy")

		result, err := fallback.Execute(ctx)
		if err == nil {
			fc.logger.Info().
				Int("fallback_index", index).
				Msg("fallback strategy succeeded")

			return result, nil
		}

		lastErr = err
		fc.logger.Warn().
			Err(err).
			Int("fallback_index", index).
			Msg("fallback strategy failed")
	}

	return nil, fmt.Errorf("all fallback strategies exhausted: %w", lastErr)
}

func (sfs *StaticFallbackStrategy) Execute(_ context.Context) (any, error) {
	return sfs.data, nil
}

func (ffs *FunctionFallbackStrategy) Execute(ctx context.Context) (any, error) {
	return ffs.fn(ctx)
}
