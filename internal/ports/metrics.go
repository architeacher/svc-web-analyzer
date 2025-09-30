//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
)

const (
	AnalysisRequestDuration = "analysis_duration_seconds"
	FetchTimeDuration       = "fetch_time_seconds"
	HttpRequestDuration     = "http_request_duration_seconds"
	ProcessingTimeDuration  = "processing_time_seconds"
	AnalysisErrorTotal      = "analysis_errors_total"
	AnalysisRequestTotal    = "analysis_requests_total"
	FallbackAttemptTotal    = "fallback_attempts_total"
	FallbackFailureTotal    = "fallback_failures_total"
	FallbackSuccessTotal    = "fallback_success_total"
	HttpRequestTotal        = "http_requests_total"
	LinkCheckErrorTotal     = "link_check_errors_total"
	LinkCheckTotal          = "link_checks_total"
	OutboxErrorTotal        = "outbox_errors_total"
	OutboxProcessedTotal    = "outbox_processed_total"
	RetryAttemptTotal       = "retry_attempts_total"
	RetryFailureTotal       = "retry_failures_total"
	RetrySuccessTotal       = "retry_success_total"
	CircuitBreakerState     = "circuit_breaker_state"
	HttpRequestSize         = "http_request_size_bytes"
	HttpResponseSize        = "http_response_size_bytes"
)

//counterfeiter:generate -o ../mocks/metrics.go . MetricsClient
type (
	MetricsClient interface {
		metricsKit.Client
	}
)
