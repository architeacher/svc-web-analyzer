package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/resilience"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FallbackWrapperTestSuite struct {
	suite.Suite
	logger  appLogger.Logger
	metrics infrastructure.Metrics
	ctx     context.Context
}

func TestFallbackWrapperTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FallbackWrapperTestSuite))
}

func (suite *FallbackWrapperTestSuite) SetupTest() {
	suite.logger = appLogger.NewTestLogger()
	suite.metrics = infrastructure.NewTestMetrics()
	suite.ctx = context.Background()
}

func (suite *FallbackWrapperTestSuite) TestExecute_PrimarySucceeds() {
	t := suite.T()
	t.Parallel()

	expectedResult := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 200,
		HTML:       "<html><body>Test</body></html>",
	}

	primaryFn := func() (any, error) {
		return expectedResult, nil
	}

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: nil,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, expectedResult, result)
}

func (suite *FallbackWrapperTestSuite) TestExecute_PrimaryFailsUsesStaticFallback() {
	t := suite.T()
	t.Parallel()

	primaryErr := errors.New("primary operation failed")
	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	fallbackData := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 503,
		HTML:       "<html><body>Service Unavailable</body></html>",
	}

	staticFallback := resilience.NewStaticFallbackStrategy(fallbackData)

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: staticFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, fallbackData, result)
}

func (suite *FallbackWrapperTestSuite) TestExecute_PrimaryFailsUsesFunctionFallback() {
	t := suite.T()
	t.Parallel()

	primaryErr := errors.New("primary operation failed")
	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	fallbackData := &domain.WebPageContent{
		URL:        "https://example.com",
		StatusCode: 200,
		HTML:       "<html><body>Cached Content</body></html>",
	}

	functionFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return fallbackData, nil
	})

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: functionFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, fallbackData, result)
}

func (suite *FallbackWrapperTestSuite) TestExecute_BothPrimaryAndFallbackFail() {
	t := suite.T()
	t.Parallel()

	primaryErr := errors.New("primary operation failed")
	fallbackErr := errors.New("fallback also failed")

	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	functionFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return nil, fallbackErr
	})

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: functionFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "fallback failed")
}

func (suite *FallbackWrapperTestSuite) TestExecute_FallbackDisabled() {
	t := suite.T()
	t.Parallel()

	primaryErr := errors.New("primary operation failed")
	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	staticFallback := resilience.NewStaticFallbackStrategy(&domain.WebPageContent{})

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  false,
		Fallback: staticFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, primaryErr, err)
}

func (suite *FallbackWrapperTestSuite) TestExecute_NoFallbackConfigured() {
	t := suite.T()
	t.Parallel()

	primaryErr := errors.New("primary operation failed")
	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: nil,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, primaryErr, err)
}

func (suite *FallbackWrapperTestSuite) TestExecute_ContextCanceled() {
	t := suite.T()
	t.Parallel()

	ctx, cancel := context.WithCancel(suite.ctx)
	cancel()

	primaryFn := func() (any, error) {
		return nil, ctx.Err()
	}

	staticFallback := resilience.NewStaticFallbackStrategy(&domain.WebPageContent{})

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: staticFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(ctx, primaryFn)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorIs(t, err, context.Canceled)
}

func (suite *FallbackWrapperTestSuite) TestExecute_ContextTimeout() {
	t := suite.T()
	t.Parallel()

	ctx, cancel := context.WithTimeout(suite.ctx, 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	primaryFn := func() (any, error) {
		return nil, ctx.Err()
	}

	staticFallback := resilience.NewStaticFallbackStrategy(&domain.WebPageContent{})

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: staticFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(ctx, primaryFn)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func (suite *FallbackWrapperTestSuite) TestExecute_RecordsMetrics() {
	t := suite.T()
	t.Parallel()

	// This test would be expanded with actual metricsClient mocking
	// For now, just ensure it doesn't panic
	primaryErr := errors.New("primary operation failed")
	primaryFn := func() (any, error) {
		return nil, primaryErr
	}

	fallbackData := &domain.WebPageContent{
		URL: "https://example.com",
	}

	staticFallback := resilience.NewStaticFallbackStrategy(fallbackData)

	config := resilience.FallbackConfig{
		Name:     "test-fallback",
		Enabled:  true,
		Fallback: staticFallback,
	}

	wrapper := resilience.NewFallbackWrapper(config, suite.logger, suite.metrics)
	result, err := wrapper.Execute(suite.ctx, primaryFn)

	require.NoError(t, err)
	require.NotNil(t, result)
}
