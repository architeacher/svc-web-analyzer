package middleware

import (
	"context"
	"net/http"
	"strings"
)

type HealthCheckFilter struct {
	healthEndpoints []string
	logHealthChecks bool
}

func NewHealthCheckFilter(logHealthChecks bool) *HealthCheckFilter {
	return &HealthCheckFilter{
		healthEndpoints: []string{
			"/v1/health",
			"/v1/ready",
			"/v1/live",
			"/v1/readiness",
			"/health",
			"/ready",
			"/live",
			"/healthz",
			"/readyz",
			"/livez",
		},
		logHealthChecks: logHealthChecks,
	}
}

func (h *HealthCheckFilter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.logHealthChecks {
			next.ServeHTTP(w, r)

			return
		}

		for _, endpoint := range h.healthEndpoints {
			if strings.HasSuffix(r.URL.Path, endpoint) {
				ctx := context.WithValue(r.Context(), skipAccessLogKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
