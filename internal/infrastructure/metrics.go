//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	metricsNamespace = "web_analyzer"
)

type (
	//counterfeiter:generate -o ../mocks/metrics.go . Metrics

	Metrics interface {
		RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64)
		RecordAnalysisRequest(ctx context.Context, duration time.Duration, success bool, errorType string)
		RecordOutboxEvent(ctx context.Context, success bool, priority string)
		RecordLinkCheck(ctx context.Context, success bool, linkType string)
		RecordFetchTime(ctx context.Context, duration time.Duration)
		RecordProcessingTime(ctx context.Context, duration time.Duration)
		Handler() http.Handler
		Shutdown(ctx context.Context) error
	}

	OTELMetrics struct {
		meterProvider *sdkmetric.MeterProvider
		meter         metric.Meter
		logger        Logger

		httpRequestTotal        metric.Int64Counter
		httpRequestDuration     metric.Float64Histogram
		httpRequestSize         metric.Int64Histogram
		httpResponseSize        metric.Int64Histogram
		analysisRequestTotal    metric.Int64Counter
		analysisRequestDuration metric.Float64Histogram
		analysisErrorTotal      metric.Int64Counter
		outboxProcessedTotal    metric.Int64Counter
		outboxErrorTotal        metric.Int64Counter
		linkCheckTotal          metric.Int64Counter
		linkCheckErrorTotal     metric.Int64Counter
		fetchTimeDuration       metric.Float64Histogram
		processingTimeDuration  metric.Float64Histogram
	}
)

func NewMetrics(ctx context.Context, cfg config.ServiceConfig, logger Logger) (Metrics, error) {
	if !cfg.Telemetry.Metrics.Enabled {
		logger.Info().Msg("metrics disabled, using NoOp implementation")

		return &NoOpMetrics{}, nil
	}

	return NewOTELMetrics(ctx, cfg, logger)
}

func NewOTELMetrics(ctx context.Context, cfg config.ServiceConfig, logger Logger) (*OTELMetrics, error) {
	endpoint := fmt.Sprintf("%s:%s", cfg.Telemetry.OtelGRPCHost, cfg.Telemetry.OtelGRPCPort)

	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to OTEL collector: %w", err)
	}

	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.AppConfig.ServiceName),
			semconv.ServiceVersionKey.String(cfg.AppConfig.ServiceVersion),
			semconv.ServiceInstanceIDKey.String(cfg.AppConfig.CommitSHA),
			semconv.DeploymentEnvironmentKey.String(cfg.AppConfig.Env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)

	meter := meterProvider.Meter(
		metricsNamespace,
		metric.WithInstrumentationVersion(cfg.AppConfig.ServiceVersion),
	)

	logger.With().Str("component", "metrics")

	provider := &OTELMetrics{
		meterProvider: meterProvider,
		meter:         meter,
		logger:        logger,
	}

	if err := provider.initializeMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	logger.Info().
		Str("otel_endpoint", endpoint).
		Msg("OTEL metrics provider initialized successfully")

	return provider, nil
}

func (om *OTELMetrics) initializeMetrics() error {
	var err error

	om.httpRequestTotal, err = om.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_requests_total counter: %w", err)
	}

	om.httpRequestDuration, err = om.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_duration_seconds histogram: %w", err)
	}

	om.httpRequestSize, err = om.meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_size_bytes histogram: %w", err)
	}

	om.httpResponseSize, err = om.meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_response_size_bytes histogram: %w", err)
	}

	om.analysisRequestTotal, err = om.meter.Int64Counter(
		"analysis_requests_total",
		metric.WithDescription("Total number of analysis requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create analysis_requests_total counter: %w", err)
	}

	om.analysisRequestDuration, err = om.meter.Float64Histogram(
		"analysis_duration_seconds",
		metric.WithDescription("Analysis processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create analysis_duration_seconds histogram: %w", err)
	}

	om.analysisErrorTotal, err = om.meter.Int64Counter(
		"analysis_errors_total",
		metric.WithDescription("Total number of analysis errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create analysis_errors_total counter: %w", err)
	}

	om.outboxProcessedTotal, err = om.meter.Int64Counter(
		"outbox_processed_total",
		metric.WithDescription("Total number of outbox events processed"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create outbox_processed_total counter: %w", err)
	}

	om.outboxErrorTotal, err = om.meter.Int64Counter(
		"outbox_errors_total",
		metric.WithDescription("Total number of outbox processing errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create outbox_errors_total counter: %w", err)
	}

	om.linkCheckTotal, err = om.meter.Int64Counter(
		"link_checks_total",
		metric.WithDescription("Total number of link checks performed"),
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create link_checks_total counter: %w", err)
	}

	om.linkCheckErrorTotal, err = om.meter.Int64Counter(
		"link_check_errors_total",
		metric.WithDescription("Total number of link check errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create link_check_errors_total counter: %w", err)
	}

	om.fetchTimeDuration, err = om.meter.Float64Histogram(
		"fetch_time_seconds",
		metric.WithDescription("Time spent fetching web page content in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create fetch_time_seconds histogram: %w", err)
	}

	om.processingTimeDuration, err = om.meter.Float64Histogram(
		"processing_time_seconds",
		metric.WithDescription("Time spent processing and analyzing content in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create processing_time_seconds histogram: %w", err)
	}

	return nil
}

func (om *OTELMetrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	om.httpRequestTotal.Add(ctx, 1,
		metric.WithAttributes(
			HTTPMethodAttr(method),
			HTTPPathAttr(path),
			HTTPStatusCodeAttr(statusCode),
		),
	)

	om.httpRequestDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			HTTPMethodAttr(method),
			HTTPPathAttr(path),
			HTTPStatusCodeAttr(statusCode),
		),
	)

	om.httpRequestSize.Record(ctx, requestSize,
		metric.WithAttributes(
			HTTPMethodAttr(method),
			HTTPPathAttr(path),
		),
	)

	om.httpResponseSize.Record(ctx, responseSize,
		metric.WithAttributes(
			HTTPMethodAttr(method),
			HTTPPathAttr(path),
			HTTPStatusCodeAttr(statusCode),
		),
	)
}

func (om *OTELMetrics) RecordAnalysisRequest(ctx context.Context, duration time.Duration, success bool, errorType string) {
	status := "success"
	if !success {
		status = "error"
	}

	om.analysisRequestTotal.Add(ctx, 1,
		metric.WithAttributes(
			StatusAttr(status),
		),
	)

	om.analysisRequestDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			StatusAttr(status),
		),
	)

	if !success && errorType != "" {
		om.analysisErrorTotal.Add(ctx, 1,
			metric.WithAttributes(
				ErrorTypeAttr(errorType),
			),
		)
	}
}

func (om *OTELMetrics) RecordOutboxEvent(ctx context.Context, success bool, priority string) {
	if success {
		om.outboxProcessedTotal.Add(ctx, 1,
			metric.WithAttributes(
				PriorityAttr(priority),
			),
		)

		return
	}

	om.outboxErrorTotal.Add(ctx, 1,
		metric.WithAttributes(
			PriorityAttr(priority),
		),
	)
}

func (om *OTELMetrics) RecordLinkCheck(ctx context.Context, success bool, linkType string) {
	om.linkCheckTotal.Add(ctx, 1,
		metric.WithAttributes(
			LinkTypeAttr(linkType),
		),
	)

	if !success {
		om.linkCheckErrorTotal.Add(ctx, 1,
			metric.WithAttributes(
				LinkTypeAttr(linkType),
			),
		)
	}
}

func (om *OTELMetrics) RecordFetchTime(ctx context.Context, duration time.Duration) {
	om.fetchTimeDuration.Record(ctx, duration.Seconds())
}

func (om *OTELMetrics) RecordProcessingTime(ctx context.Context, duration time.Duration) {
	om.processingTimeDuration.Record(ctx, duration.Seconds())
}

func (om *OTELMetrics) Handler() http.Handler {
	return promhttp.Handler()
}

func (om *OTELMetrics) Shutdown(ctx context.Context) error {
	if err := om.meterProvider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}

	return nil
}
