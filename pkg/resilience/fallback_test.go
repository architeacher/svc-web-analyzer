package resilience_test

import (
	"context"
	"errors"
	"testing"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/resilience"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	errPrimaryFailed  = errors.New("primary operation failed")
	errFallbackFailed = errors.New("fallback failed")
)

type FallbackTestSuite struct {
	suite.Suite
	logger appLogger.Logger
}

func TestFallbackSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(FallbackTestSuite))
}

func (s *FallbackTestSuite) SetupTest() {
	s.logger = appLogger.NewTestLogger()
}

func (s *FallbackTestSuite) TestNewFallbackChain() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return "primary", nil
	}

	chain := resilience.NewFallbackChain(primary, s.logger)

	require.NotNil(t, chain)
}

func (s *FallbackTestSuite) TestExecute_PrimarySuccess() {
	t := s.T()
	t.Parallel()

	expectedResult := "primary success"
	primary := func(ctx context.Context) (any, error) {
		return expectedResult, nil
	}

	chain := resilience.NewFallbackChain(primary, s.logger)
	ctx := context.Background()

	result, err := chain.Execute(ctx)

	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func (s *FallbackTestSuite) TestExecute_PrimaryFailsNoFallback() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return nil, errPrimaryFailed
	}

	chain := resilience.NewFallbackChain(primary, s.logger)
	ctx := context.Background()

	result, err := chain.Execute(ctx)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "all fallback strategies exhausted")
}

func (s *FallbackTestSuite) TestExecute_PrimaryFailsFallbackSucceeds() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return nil, errPrimaryFailed
	}

	expectedFallbackResult := "fallback success"
	fallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return expectedFallbackResult, nil
	})

	chain := resilience.NewFallbackChain(primary, s.logger).AddFallback(fallback)
	ctx := context.Background()

	result, err := chain.Execute(ctx)

	require.NoError(t, err)
	require.Equal(t, expectedFallbackResult, result)
}

func (s *FallbackTestSuite) TestExecute_MultipleFallbacks() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return nil, errPrimaryFailed
	}

	firstFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return nil, errFallbackFailed
	})

	secondFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return "second fallback success", nil
	})

	chain := resilience.NewFallbackChain(primary, s.logger).
		AddFallback(firstFallback).
		AddFallback(secondFallback)

	ctx := context.Background()

	result, err := chain.Execute(ctx)

	require.NoError(t, err)
	require.Equal(t, "second fallback success", result)
}

func (s *FallbackTestSuite) TestExecute_AllFallbacksFail() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return nil, errPrimaryFailed
	}

	firstFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return nil, errors.New("first fallback failed")
	})

	secondFallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return nil, errors.New("second fallback failed")
	})

	chain := resilience.NewFallbackChain(primary, s.logger).
		AddFallback(firstFallback).
		AddFallback(secondFallback)

	ctx := context.Background()

	result, err := chain.Execute(ctx)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "all fallback strategies exhausted")
}

func (s *FallbackTestSuite) TestStaticFallbackStrategy() {
	t := s.T()
	t.Parallel()

	staticData := "static fallback data"
	fallback := resilience.NewStaticFallbackStrategy(staticData)

	ctx := context.Background()

	result, err := fallback.Execute(ctx)

	require.NoError(t, err)
	require.Equal(t, staticData, result)
}

func (s *FallbackTestSuite) TestStaticFallbackStrategy_NilData() {
	t := s.T()
	t.Parallel()

	fallback := resilience.NewStaticFallbackStrategy(nil)

	ctx := context.Background()

	result, err := fallback.Execute(ctx)

	require.NoError(t, err)
	require.Nil(t, result)
}

func (s *FallbackTestSuite) TestFunctionFallbackStrategy() {
	t := s.T()
	t.Parallel()

	expectedResult := "function result"
	fallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return expectedResult, nil
	})

	ctx := context.Background()

	result, err := fallback.Execute(ctx)

	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func (s *FallbackTestSuite) TestFunctionFallbackStrategy_Error() {
	t := s.T()
	t.Parallel()

	expectedError := errors.New("function error")
	fallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		return nil, expectedError
	})

	ctx := context.Background()

	result, err := fallback.Execute(ctx)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, expectedError, err)
}

func (s *FallbackTestSuite) TestFallbackChain_ContextCancellation() {
	t := s.T()
	t.Parallel()

	primary := func(ctx context.Context) (any, error) {
		return nil, errPrimaryFailed
	}

	fallback := resilience.NewFunctionFallbackStrategy(func(ctx context.Context) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return "fallback success", nil
		}
	})

	chain := resilience.NewFallbackChain(primary, s.logger).AddFallback(fallback)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := chain.Execute(ctx)

	require.Error(t, err)
	require.Nil(t, result)
}
