package resilience_test

import (
	"context"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/resilience"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FallbackStrategiesTestSuite struct {
	suite.Suite
	logger appLogger.Logger
	ctx    context.Context
}

func TestFallbackStrategiesTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FallbackStrategiesTestSuite))
}

func (suite *FallbackStrategiesTestSuite) SetupTest() {
	suite.logger = appLogger.NewTestLogger()
	suite.ctx = context.Background()
}

func (suite *FallbackStrategiesTestSuite) TestStaticFallbackStrategy_Execute() {
	t := suite.T()
	t.Parallel()

	expectedContent := &domain.WebPageContent{
		URL:            "https://example.com",
		StatusCode:     503,
		HTML:           "<html><body>Service Unavailable</body></html>",
		ContentType:    "text/html",
		FromFallback:   true,
		FallbackReason: "static_fallback",
	}

	strategy := resilience.NewStaticFallbackStrategy(expectedContent)
	result, err := strategy.Execute(suite.ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, expectedContent, result)
}

func (suite *FallbackStrategiesTestSuite) TestFunctionFallbackStrategy_Execute() {
	t := suite.T()
	t.Parallel()

	expectedContent := &domain.WebPageContent{
		URL:         "https://example.com",
		StatusCode:  200,
		HTML:        "<html><body>Cached Content</body></html>",
		ContentType: "text/html",
	}

	strategy := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return expectedContent, nil
	})

	result, err := strategy.Execute(suite.ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, expectedContent, result)
}

func (suite *FallbackStrategiesTestSuite) TestCacheFallbackStrategy_Execute_CacheHit() {
	t := suite.T()
	t.Parallel()

	url := "https://example.com"
	cachedContent := &domain.WebPageContent{
		URL:           url,
		StatusCode:    200,
		HTML:          "<html><body>Cached Content</body></html>",
		ContentType:   "text/html",
		FetchDuration: 100 * time.Millisecond,
	}

	mockCache := &mockCacheClient{
		getFunc: func(ctx context.Context, key string) ([]byte, error) {
			return []byte(`{"URL":"https://example.com","StatusCode":200,"HTML":"<html><body>Cached Content</body></html>","ContentType":"text/html","FetchDuration":100000000}`), nil
		},
	}

	strategy := resilience.NewCacheFallbackStrategy(url, mockCache, suite.logger)
	result, err := strategy.Execute(suite.ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	content, ok := result.(*domain.WebPageContent)
	require.True(t, ok)
	require.Equal(t, url, content.URL)
	require.Equal(t, cachedContent.HTML, content.HTML)
	require.True(t, content.FromFallback)
	require.Equal(t, "cache_fallback", content.FallbackReason)
}

func (suite *FallbackStrategiesTestSuite) TestCacheFallbackStrategy_Execute_CacheMiss() {
	t := suite.T()
	t.Parallel()

	url := "https://example.com"

	mockCache := &mockCacheClient{
		getFunc: func(ctx context.Context, key string) ([]byte, error) {
			return nil, nil
		},
	}

	strategy := resilience.NewCacheFallbackStrategy(url, mockCache, suite.logger)
	result, err := strategy.Execute(suite.ctx)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "no cached content available")
}

func (suite *FallbackStrategiesTestSuite) TestWebFetcherStaticFallbackStrategy_Execute() {
	t := suite.T()
	t.Parallel()

	url := "https://example.com"

	strategy := resilience.NewWebFetcherStaticFallbackStrategy(url)
	result, err := strategy.Execute(suite.ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	content, ok := result.(*domain.WebPageContent)
	require.True(t, ok)
	require.Equal(t, url, content.URL)
	require.Equal(t, 503, content.StatusCode)
	require.True(t, content.FromFallback)
	require.Equal(t, "static_fallback", content.FallbackReason)
	require.Contains(t, content.HTML, "Service Temporarily Unavailable")
}

type mockCacheClient struct {
	getFunc func(ctx context.Context, key string) ([]byte, error)
	setFunc func(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

func (m *mockCacheClient) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, key)
	}

	return nil, nil
}

func (m *mockCacheClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.setFunc != nil {
		return m.setFunc(ctx, key, value, ttl)
	}

	return nil
}

func (m *mockCacheClient) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockCacheClient) Ping(_ context.Context) error {
	return nil
}

func (m *mockCacheClient) Close() error {
	return nil
}
