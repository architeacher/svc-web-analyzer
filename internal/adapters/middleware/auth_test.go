package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto/v2"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/mocks"
	"github.com/stretchr/testify/require"
)

const (
	testLogLevel               = "error"
	testLogFormat              = "json"
	testRequestIDKey           = "request_id"
	testRequestIDValue         = "test-123"
	testIssuer                 = "test-issuer"
	testToken                  = "v4.public.test"
	testHealthPath             = "/health"
	testMetricsPath            = "/metrics"
	testAPIPath                = "/api/test"
	testPasetoKeyPath          = "secret/paseto/keys"
	authHeaderFormat           = "Bearer %s"
	errKeyRetrievalFailed      = "key retrieval failed"
	errContextCanceled         = "context canceled"
	errContextDeadlineExceeded = "context deadline exceeded"
)

func TestPasetoAuthMiddleware_ContextPropagation(t *testing.T) {
	t.Parallel()

	logger := infrastructure.New(config.LoggingConfig{
		Level:  testLogLevel,
		Format: testLogFormat,
	})

	cases := []struct {
		name           string
		setupContext   func() (context.Context, context.CancelFunc)
		setupKeyFunc   func(t *testing.T, ctx context.Context) func(context.Context) (paseto.V4AsymmetricPublicKey, error)
		token          string
		expectError    bool
		errorSubstring string
	}{
		{
			name: "context cancellation propagates to key service",
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately

				return ctx, cancel
			},
			setupKeyFunc: func(t *testing.T, _ context.Context) func(context.Context) (paseto.V4AsymmetricPublicKey, error) {
				return func(callCtx context.Context) (paseto.V4AsymmetricPublicKey, error) {
					require.NotNil(t, callCtx.Err(), "Expected context to be cancelled, but it wasn't")

					return paseto.V4AsymmetricPublicKey{}, callCtx.Err()
				}
			},
			token:          testToken,
			expectError:    true,
			errorSubstring: errContextCanceled,
		},
		{
			name: "context timeout propagates to key service",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Millisecond)
			},
			setupKeyFunc: func(t *testing.T, ctx context.Context) func(context.Context) (paseto.V4AsymmetricPublicKey, error) {
				return func(callCtx context.Context) (paseto.V4AsymmetricPublicKey, error) {
					// Simulate slow operation
					<-time.After(10 * time.Millisecond)

					require.NotNil(t, callCtx.Err(), "Expected context to be timed out, but it wasn't")

					return paseto.V4AsymmetricPublicKey{}, callCtx.Err()
				}
			},
			token:          testToken,
			expectError:    true,
			errorSubstring: errContextDeadlineExceeded,
		},
		{
			name: "context values propagate to key service",
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx := context.WithValue(context.Background(), testRequestIDKey, testRequestIDValue)

				return ctx, func() {}
			},
			setupKeyFunc: func(t *testing.T, ctx context.Context) func(context.Context) (paseto.V4AsymmetricPublicKey, error) {
				return func(callCtx context.Context) (paseto.V4AsymmetricPublicKey, error) {
					requestID := callCtx.Value(testRequestIDKey)
					require.Equal(t, testRequestIDValue, requestID, "Expected request_id to be '%s'", testRequestIDValue)

					return paseto.V4AsymmetricPublicKey{}, errors.New(errKeyRetrievalFailed)
				}
			},
			token:          testToken,
			expectError:    true,
			errorSubstring: errKeyRetrievalFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := tc.setupContext()
			defer cancel()

			fakeKeyService := &mocks.FakeKeyService{}
			fakeKeyService.GetPublicKeyStub = tc.setupKeyFunc(t, ctx)

			cfg := config.AuthConfig{
				Enabled:       true,
				SkipPaths:     []string{testHealthPath},
				ValidIssuers:  []string{testIssuer},
				UseVaultKeys:  true,
				KeyCacheTTL:   5 * time.Minute,
				PasetoKeyPath: testPasetoKeyPath,
			}

			middleware := NewPasetoAuthMiddleware(cfg, logger, fakeKeyService)

			req := httptest.NewRequest(http.MethodGet, testAPIPath, nil)
			req = req.WithContext(ctx)
			req.Header.Set("Authorization", fmt.Sprintf(authHeaderFormat, tc.token))

			rec := httptest.NewRecorder()

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			middleware.Middleware(next).ServeHTTP(rec, req)

			if tc.expectError {
				require.False(t, nextCalled, "Expected next handler not to be called, but it was")
				require.Equal(t, http.StatusUnauthorized, rec.Code, "Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}
		})
	}
}

func TestPasetoAuthMiddleware_SkipAuthPaths(t *testing.T) {
	t.Parallel()

	logger := infrastructure.New(config.LoggingConfig{
		Level:  testLogLevel,
		Format: testLogFormat,
	})

	cfg := config.AuthConfig{
		Enabled:      true,
		SkipPaths:    []string{testHealthPath, testMetricsPath},
		ValidIssuers: []string{testIssuer},
	}

	fakeKeyService := &mocks.FakeKeyService{}
	middleware := NewPasetoAuthMiddleware(cfg, logger, fakeKeyService)

	cases := []struct {
		name       string
		path       string
		shouldSkip bool
	}{
		{
			name:       "health endpoint should skip auth",
			path:       testHealthPath,
			shouldSkip: true,
		},
		{
			name:       "metrics endpoint should skip auth",
			path:       testMetricsPath,
			shouldSkip: true,
		},
		{
			name:       "api endpoint should not skip auth",
			path:       testAPIPath,
			shouldSkip: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			middleware.Middleware(next).ServeHTTP(rec, req)

			if tc.shouldSkip {
				require.True(t, nextCalled, "Expected next handler to be called for skip path, but it wasn't")
				require.Equal(t, http.StatusOK, rec.Code, "Expected status %d for skip path, got %d", http.StatusOK, rec.Code)
			}
		})
	}
}
