package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/resilience"
	"github.com/stretchr/testify/suite"
)

const (
	testOperation = "test-operation"
)

var (
	errRetryable    = errors.New("retryable error")
	errNonRetryable = errors.New("non-retryable error")
)

type RetryTestSuite struct {
	suite.Suite
	logger  appLogger.Logger
	metrics *mocks.FakeMetrics
}

func TestRetrySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RetryTestSuite))
}

func (s *RetryTestSuite) SetupTest() {
	s.logger = appLogger.NewTestLogger()
	s.metrics = &mocks.FakeMetrics{}
}

func (s *RetryTestSuite) TestNewRetryWrapper() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	config := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	wrapper := resilience.NewRetryWrapper(config, s.logger, s.metrics)

	s.Require().NotNil(wrapper)
}

func (s *RetryTestSuite) TestExecute_SuccessOnFirstAttempt() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	metrics := &mocks.FakeMetrics{}
	wrapper := resilience.NewRetryWrapper(cfg, s.logger, metrics)
	ctx := context.Background()

	expectedResult := "success"
	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		return expectedResult, nil
	})

	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
	s.Require().Equal(0, metrics.RecordRetryAttemptCallCount())
}

func (s *RetryTestSuite) TestExecute_SuccessAfterRetries() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	metrics := &mocks.FakeMetrics{}
	wrapper := resilience.NewRetryWrapper(cfg, s.logger, metrics)
	ctx := context.Background()

	attemptCount := 0
	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, errRetryable
		}

		return "success", nil
	})

	s.Require().NoError(err)
	s.Require().Equal("success", result)
	s.Require().Equal(3, attemptCount)
	s.Require().Equal(1, metrics.RecordRetrySuccessCallCount())
}

func (s *RetryTestSuite) TestExecute_FailureAfterMaxRetries() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	wrapper := resilience.NewRetryWrapper(cfg, s.logger, s.metrics)
	ctx := context.Background()

	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		return nil, errRetryable
	})

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().Contains(err.Error(), "max retries exceeded")
	s.Require().Equal(1, s.metrics.RecordRetryFailureCallCount())
}

func (s *RetryTestSuite) TestExecute_NonRetryableError() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	metrics := &mocks.FakeMetrics{}
	wrapper := resilience.NewRetryWrapper(cfg, s.logger, metrics)
	ctx := context.Background()

	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		return nil, errNonRetryable
	})

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().Equal(errNonRetryable, err)
	s.Require().Equal(0, metrics.RecordRetryAttemptCallCount())
}

func (s *RetryTestSuite) TestExecute_ContextCancellation() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      5,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	wrapper := resilience.NewRetryWrapper(cfg, s.logger, s.metrics)
	ctx, cancel := context.WithCancel(context.Background())

	attemptCount := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		attemptCount++

		return nil, errRetryable
	})

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().True(errors.Is(err, context.Canceled))
	s.Require().LessOrEqual(attemptCount, 2)
}

func (s *RetryTestSuite) TestExecute_DefaultRetryableErrors() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      2,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: nil,
	}

	wrapper := resilience.NewRetryWrapper(cfg, s.logger, s.metrics)
	ctx := context.Background()

	attemptCount := 0
	result, err := wrapper.Execute(ctx, testOperation, func() (any, error) {
		attemptCount++
		if attemptCount < 2 {
			return nil, errors.New("any error")
		}

		return "success", nil
	})

	s.Require().NoError(err)
	s.Require().Equal("success", result)
	s.Require().Equal(2, attemptCount)
}

func (s *RetryTestSuite) TestExecute_MetricsRecorded() {
	s.T().Parallel()

	backoffStrategy := backoff.NewExponentialStrategy(config.BackoffConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0.1,
	})

	cfg := resilience.RetryConfig{
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		RetryableErrors: []error{errRetryable},
	}

	metrics := &mocks.FakeMetrics{}
	wrapper := resilience.NewRetryWrapper(cfg, s.logger, metrics)
	ctx := context.Background()

	attemptCount := 0
	wrapper.Execute(ctx, testOperation, func() (any, error) {
		attemptCount++
		if attemptCount < 2 {
			return nil, errRetryable
		}

		return "success", nil
	})

	s.Require().Equal(1, metrics.RecordRetrySuccessCallCount())
	_, operation, attempts := metrics.RecordRetrySuccessArgsForCall(0)
	s.Require().Equal(testOperation, operation)
	s.Require().Equal(uint(1), attempts)
}
