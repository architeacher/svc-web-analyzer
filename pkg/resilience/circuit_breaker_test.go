package resilience_test

import (
	"errors"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/resilience"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/suite"
)

type CircuitBreakerTestSuite struct {
	suite.Suite
	logger  appLogger.Logger
	metrics *mocks.FakeMetrics
}

func TestCircuitBreakerSuite(t *testing.T) {
	suite.Run(t, new(CircuitBreakerTestSuite))
}

func (s *CircuitBreakerTestSuite) SetupTest() {
	s.logger = appLogger.NewTestLogger()
	s.metrics = &mocks.FakeMetrics{}
}

func (s *CircuitBreakerTestSuite) TestNewCircuitBreaker() {
	tests := []struct {
		name   string
		config resilience.CircuitBreakerConfig
	}{
		{
			name: "creates circuit breaker with valid config",
			config: resilience.CircuitBreakerConfig{
				Name:             "test-breaker",
				MaxRequests:      3,
				Interval:         10 * time.Second,
				Timeout:          60 * time.Second,
				FailureThreshold: 0.6,
			},
		},
		{
			name: "creates circuit breaker with custom state change callback",
			config: resilience.CircuitBreakerConfig{
				Name:             "test-breaker-callback",
				MaxRequests:      5,
				Interval:         5 * time.Second,
				Timeout:          30 * time.Second,
				FailureThreshold: 0.5,
				OnStateChange: func(name string, from, to gobreaker.State) {
					// Custom callback
				},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			cb := resilience.NewCircuitBreaker(tt.config, s.logger, s.metrics)

			s.Require().NotNil(cb)
		})
	}
}

func (s *CircuitBreakerTestSuite) TestExecute_Success() {
	config := resilience.CircuitBreakerConfig{
		Name:             "test-success",
		MaxRequests:      3,
		Interval:         10 * time.Second,
		Timeout:          60 * time.Second,
		FailureThreshold: 0.6,
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)
	ctx := s.T().Context()

	expectedResult := "success"
	result, err := cb.Execute(ctx, func() (any, error) {
		return expectedResult, nil
	})

	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
}

func (s *CircuitBreakerTestSuite) TestExecute_Failure() {
	config := resilience.CircuitBreakerConfig{
		Name:             "test-failure",
		MaxRequests:      3,
		Interval:         10 * time.Second,
		Timeout:          60 * time.Second,
		FailureThreshold: 0.6,
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)
	ctx := s.T().Context()

	expectedError := errors.New("operation failed")
	result, err := cb.Execute(ctx, func() (any, error) {
		return nil, expectedError
	})

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().Equal(expectedError, err)
}

func (s *CircuitBreakerTestSuite) TestExecute_OpenState() {
	config := resilience.CircuitBreakerConfig{
		Name:             "test-open-state",
		MaxRequests:      1,
		Interval:         100 * time.Millisecond,
		Timeout:          1 * time.Second,
		FailureThreshold: 0.5,
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)
	ctx := s.T().Context()

	failureError := errors.New("operation failed")
	for index := 0; index < 10; index++ {
		_, _ = cb.Execute(ctx, func() (any, error) {
			return nil, failureError
		})
	}

	result, err := cb.Execute(ctx, func() (any, error) {
		return "should not execute", nil
	})

	s.Require().Error(err)
	s.Require().Nil(result)
	s.Require().Equal(gobreaker.ErrOpenState, err)
}

func (s *CircuitBreakerTestSuite) TestExecute_StateTransitions() {
	stateChanges := make([]gobreaker.State, 0)
	config := resilience.CircuitBreakerConfig{
		Name:             "test-state-transitions",
		MaxRequests:      1,
		Interval:         100 * time.Millisecond,
		Timeout:          500 * time.Millisecond,
		FailureThreshold: 0.5,
		OnStateChange: func(name string, from, to gobreaker.State) {
			stateChanges = append(stateChanges, to)
		},
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)
	ctx := s.T().Context()

	for index := 0; index < 10; index++ {
		_, _ = cb.Execute(ctx, func() (any, error) {
			return nil, errors.New("failure")
		})
	}

	time.Sleep(600 * time.Millisecond)

	_, _ = cb.Execute(ctx, func() (any, error) {
		return "success", nil
	})

	s.Require().Greater(len(stateChanges), 0)
}

func (s *CircuitBreakerTestSuite) TestExecute_MetricsRecorded() {
	stateChangeCalled := false
	config := resilience.CircuitBreakerConfig{
		Name:             "test-metricsClient",
		MaxRequests:      1,
		Interval:         100 * time.Millisecond,
		Timeout:          1 * time.Second,
		FailureThreshold: 0.5,
		OnStateChange: func(name string, from, to gobreaker.State) {
			stateChangeCalled = true
		},
	}

	metrics := &mocks.FakeMetrics{}
	cb := resilience.NewCircuitBreaker(config, s.logger, metrics)
	ctx := s.T().Context()

	for index := 0; index < 10; index++ {
		_, _ = cb.Execute(ctx, func() (any, error) {
			return nil, errors.New("failure")
		})
	}

	s.Require().True(stateChangeCalled)
	s.Require().Greater(metrics.RecordCircuitBreakerStateCallCount(), 0)
}

func (s *CircuitBreakerTestSuite) TestGetState() {
	config := resilience.CircuitBreakerConfig{
		Name:             "test-get-state",
		MaxRequests:      3,
		Interval:         10 * time.Second,
		Timeout:          60 * time.Second,
		FailureThreshold: 0.6,
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)

	state := cb.State()

	s.Require().Equal(gobreaker.StateClosed, state)
}

func (s *CircuitBreakerTestSuite) TestGetName() {
	expectedName := "test-get-name"
	config := resilience.CircuitBreakerConfig{
		Name:             expectedName,
		MaxRequests:      3,
		Interval:         10 * time.Second,
		Timeout:          60 * time.Second,
		FailureThreshold: 0.6,
	}

	cb := resilience.NewCircuitBreaker(config, s.logger, s.metrics)

	name := cb.Name()

	s.Require().Equal(expectedName, name)
}
