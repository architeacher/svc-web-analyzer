package resilience

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/metrics/noop"
)

type mockCacheForBench struct {
	data []byte
}

func (m *mockCacheForBench) Get(_ context.Context, _ string) ([]byte, error) {
	return m.data, nil
}

func (m *mockCacheForBench) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func BenchmarkFallbackWrapper_SuccessPath(b *testing.B) {
	logger := appLogger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()

	fallbackWrapper := NewFallbackWrapper(
		FallbackConfig{
			Name:     "benchmark-fallback",
			Enabled:  true,
			Fallback: NewWebFetcherStaticFallbackStrategy("https://example.com"),
		},
		logger,
		metricsClient,
	)

	ctx := b.Context()
	successResult := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 200,
		HTML:       "<html><body>Success</body></html>",
	}

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = fallbackWrapper.Execute(ctx, func() (any, error) {
			return successResult, nil
		})
	}
}

func BenchmarkFallbackWrapper_CacheFallback(b *testing.B) {
	logger := appLogger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()

	// Prepare cached content
	cachedContent := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 200,
		HTML:       "<html><body>Cached content</body></html>",
	}
	cachedJSON, _ := json.Marshal(cachedContent)
	cache := &mockCacheForBench{data: cachedJSON}

	cacheFallback := NewCacheFallbackStrategy("https://example.com", cache, logger)
	fallbackWrapper := NewFallbackWrapper(
		FallbackConfig{
			Name:     "benchmark-cache-fallback",
			Enabled:  true,
			Fallback: cacheFallback,
		},
		logger,
		metricsClient,
	)

	ctx := b.Context()

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = fallbackWrapper.Execute(ctx, func() (any, error) {
			return nil, fmt.Errorf("primary operation failed")
		})
	}
}

func BenchmarkFallbackWrapper_StaticFallback(b *testing.B) {
	logger := appLogger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()

	staticFallback := NewWebFetcherStaticFallbackStrategy("https://example.com")
	fallbackWrapper := NewFallbackWrapper(
		FallbackConfig{
			Name:     "benchmark-static-fallback",
			Enabled:  true,
			Fallback: staticFallback,
		},
		logger,
		metricsClient,
	)

	ctx := b.Context()

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = fallbackWrapper.Execute(ctx, func() (any, error) {
			return nil, fmt.Errorf("primary operation failed")
		})
	}
}

func BenchmarkFallbackWrapper_DisabledFallback(b *testing.B) {
	logger := appLogger.NewTestLogger()
	metricsClient := noop.NewMetricsClient()

	fallbackWrapper := NewFallbackWrapper(
		FallbackConfig{
			Name:     "benchmark-disabled-fallback",
			Enabled:  false,
			Fallback: nil,
		},
		logger,
		metricsClient,
	)

	ctx := b.Context()

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = fallbackWrapper.Execute(ctx, func() (any, error) {
			return nil, fmt.Errorf("primary operation failed")
		})
	}
}

func BenchmarkCacheFallbackStrategy_Execute(b *testing.B) {
	logger := appLogger.NewTestLogger()

	cachedContent := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 200,
		HTML:       "<html><body>Cached content</body></html>",
	}
	cachedJSON, _ := json.Marshal(cachedContent)
	cache := &mockCacheForBench{data: cachedJSON}

	strategy := NewCacheFallbackStrategy("https://example.com", cache, logger)
	ctx := b.Context()

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = strategy.Execute(ctx)
	}
}

func BenchmarkStaticFallbackStrategy_Execute(b *testing.B) {
	strategy := NewWebFetcherStaticFallbackStrategy("https://example.com")
	ctx := b.Context()

	b.ResetTimer()

	for index := 0; index < b.N; index++ {
		_, _ = strategy.Execute(ctx)
	}
}
