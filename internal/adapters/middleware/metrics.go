package middleware

import (
	"net/http"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

type MetricsMiddleware struct {
	metrics infrastructure.Metrics
}

func NewMetricsMiddleware(metrics infrastructure.Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: metrics,
	}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		wrapped := newMetricsResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(startTime)

		m.metrics.RecordHTTPRequest(
			r.Context(),
			r.Method,
			r.URL.Path,
			wrapped.StatusCode(),
			duration,
			r.ContentLength,
			wrapped.BytesWritten(),
		)
	})
}

func newMetricsResponseWriter(w http.ResponseWriter) *FlushableResponseWriter {
	return NewFlushableResponseWriter(w)
}
