package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"go.opentelemetry.io/otel/attribute"
)

const (
	httpMethodKey     = "http.method"
	httpPathKey       = "http.path"
	httpStatusCodeKey = "http.status_code"
)

type MetricsMiddleware struct {
	metricsClient ports.MetricsClient
}

func NewMetricsMiddleware(metricsClient ports.MetricsClient) *MetricsMiddleware {
	return &MetricsMiddleware{
		metricsClient: metricsClient,
	}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		wrapped := newMetricsResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(startTime)

		requestSize := clampToUint64(r.ContentLength)
		responseSize := clampToUint64(wrapped.BytesWritten())

		m.recordHTTPRequest(
			r.Context(),
			r.Method,
			r.URL.Path,
			uint(wrapped.StatusCode()),
			duration,
			requestSize,
			responseSize,
		)
	})
}

func (m *MetricsMiddleware) recordHTTPRequest(
	ctx context.Context,
	method, path string,
	statusCode uint,
	duration time.Duration,
	requestSize, responseSize uint64,
) {
	attrs := []attribute.KeyValue{
		attribute.String(httpMethodKey, method),
		attribute.String(httpPathKey, path),
		attribute.String(httpStatusCodeKey, fmt.Sprintf("%d", statusCode)),
	}

	m.metricsClient.Inc(ctx, ports.HttpRequestTotal, int64(1), attrs...)
	m.metricsClient.Inc(ctx, ports.HttpRequestDuration, duration.Seconds(), attrs...)

	m.metricsClient.Inc(
		ctx,
		ports.HttpRequestSize,
		int64(requestSize),
		attribute.String(httpMethodKey, method),
		attribute.String(httpPathKey, path),
	)

	m.metricsClient.Inc(ctx, ports.HttpResponseSize, int64(responseSize), attrs...)
}

func newMetricsResponseWriter(w http.ResponseWriter) *FlushableResponseWriter {
	return NewFlushableResponseWriter(w)
}

func clampToUint64(value int64) uint64 {
	if value > 0 {
		return uint64(value)
	}

	return 0
}
