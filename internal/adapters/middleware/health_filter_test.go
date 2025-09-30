package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckFilter_Middleware(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		path                string
		logHealthChecks     bool
		expectSkipAccessLog bool
	}{
		{
			name:                "skips health check endpoint when logging disabled",
			path:                "/v1/health",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips readiness endpoint when logging disabled",
			path:                "/v1/readiness",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips ready endpoint when logging disabled",
			path:                "/v1/ready",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips live endpoint when logging disabled",
			path:                "/v1/live",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips healthz endpoint when logging disabled",
			path:                "/healthz",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips readyz endpoint when logging disabled",
			path:                "/readyz",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "skips livez endpoint when logging disabled",
			path:                "/livez",
			logHealthChecks:     false,
			expectSkipAccessLog: true,
		},
		{
			name:                "does not skip health check when logging enabled",
			path:                "/v1/health",
			logHealthChecks:     true,
			expectSkipAccessLog: false,
		},
		{
			name:                "does not skip non-health endpoint",
			path:                "/v1/analyze",
			logHealthChecks:     false,
			expectSkipAccessLog: false,
		},
		{
			name:                "does not skip endpoint with health in middle",
			path:                "/v1/health/status",
			logHealthChecks:     false,
			expectSkipAccessLog: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			filter := NewHealthCheckFilter(tc.logHealthChecks)

			contextChecked := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contextChecked = true

				skipValue := r.Context().Value(skipAccessLogKey)

				if tc.expectSkipAccessLog {
					assert.NotNil(t, skipValue, "context should have skip value")

					if skipValue != nil {
						assert.True(t, skipValue.(bool), "skip value should be true")
					}
				} else {
					if skipValue != nil {
						assert.False(t, skipValue.(bool), "skip value should be false or nil")
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			middleware := filter.Middleware(handler)
			middleware.ServeHTTP(rec, req)

			assert.True(t, contextChecked, "handler should have been called")
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestHealthCheckFilter_WithMultipleEndpoints(t *testing.T) {
	t.Parallel()

	filter := NewHealthCheckFilter(false)

	healthEndpoints := []string{
		"/v1/health",
		"/v1/ready",
		"/v1/readiness",
		"/health",
		"/healthz",
		"/readyz",
		"/livez",
	}

	for _, endpoint := range healthEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				skipValue := r.Context().Value(skipAccessLogKey)
				assert.NotNil(t, skipValue)
				assert.True(t, skipValue.(bool))

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", endpoint, nil)
			rec := httptest.NewRecorder()

			middleware := filter.Middleware(handler)
			middleware.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}
