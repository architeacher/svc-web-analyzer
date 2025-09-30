package metrics

import (
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
)

type (
	metricType[T any] struct {
		data       *T
		descriptor metricsKit.Descriptor
	}

	metrics struct {
		Float64Histograms map[string]metricType[metric.Float64Histogram]
		Int64Counters     map[string]metricType[metric.Int64Counter]
		Int64Gauges       map[string]metricType[metric.Int64Gauge]
		Int64Histograms   map[string]metricType[metric.Int64Histogram]
	}
)

var (
	_metrics = &metrics{
		Float64Histograms: map[string]metricType[metric.Float64Histogram]{
			ports.AnalysisRequestDuration: {
				descriptor: metricsKit.Descriptor{
					Description: "Analysis processing duration in seconds",
					Unit:        "s",
				},
			},
			ports.FetchTimeDuration: {
				descriptor: metricsKit.Descriptor{
					Description: "Time spent fetching web page content in seconds", Unit: "s"},
			},
			ports.HttpRequestDuration: {
				descriptor: metricsKit.Descriptor{
					Description: "HTTP request duration in seconds",
					Unit:        "s",
				},
			},
			ports.ProcessingTimeDuration: {
				descriptor: metricsKit.Descriptor{
					Description: "Time spent processing and analyzing content in seconds",
					Unit:        "s",
				},
			},
		},
		Int64Counters: map[string]metricType[metric.Int64Counter]{
			ports.AnalysisErrorTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of analysis errors",
					Unit:        "{error}",
				},
			},
			ports.AnalysisRequestTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of analysis requests",
					Unit:        "{request}",
				},
			},
			ports.FallbackAttemptTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of fallback attempts",
					Unit:        "{attempt}",
				},
			},
			ports.FallbackFailureTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of failed fallbacks",
					Unit:        "{failure}",
				},
			},
			ports.FallbackSuccessTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of successful fallbacks",
					Unit:        "{success}",
				},
			},
			ports.HttpRequestTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of HTTP requests",
					Unit:        "{request}",
				},
			},
			ports.LinkCheckErrorTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of link check errors",
					Unit:        "{error}",
				},
			},
			ports.LinkCheckTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of link checks performed",
					Unit:        "{check}",
				},
			},
			ports.OutboxErrorTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of outbox processing errors",
					Unit:        "{error}",
				},
			},
			ports.OutboxProcessedTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of outbox events processed",
					Unit:        "{event}",
				},
			},
			ports.RetryAttemptTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of retry attempts",
					Unit:        "{attempt}",
				},
			},
			ports.RetryFailureTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of failed retries after max attempts",
					Unit:        "{failure}",
				},
			},
			ports.RetrySuccessTotal: {
				descriptor: metricsKit.Descriptor{
					Description: "Total number of successful retries",
					Unit:        "{success}",
				},
			},
		},
		Int64Gauges: map[string]metricType[metric.Int64Gauge]{
			ports.CircuitBreakerState: {
				descriptor: metricsKit.Descriptor{
					Description: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
					Unit:        "{state}",
				},
			},
		},
		Int64Histograms: map[string]metricType[metric.Int64Histogram]{
			ports.HttpRequestSize: {
				descriptor: metricsKit.Descriptor{
					Description: "HTTP request size in bytes",
					Unit:        "By",
				},
			},
			ports.HttpResponseSize: {
				descriptor: metricsKit.Descriptor{
					Description: "HTTP response size in bytes",
					Unit:        "By",
				},
			},
		},
	}
)
