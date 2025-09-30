package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
	"github.com/architeacher/svc-web-analyzer/pkg/metrics/noop"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	metricsNamespace = "web_analyzer"
)

type Adapter struct {
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	logger        appLogger.Logger
}

func NewMetricsAdapter(
	ctx context.Context,
	cfg config.ServiceConfig,
	logger appLogger.Logger,
) (ports.MetricsClient, error) {
	if !cfg.Telemetry.Metrics.Enabled {
		logger.Info().Msg("metrics disabled, using NoOp implementation")

		return noop.NewMetricsClient(), nil
	}

	meterProvider, err := infrastructure.NewOTELMeterProvider(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OTEL meter provider: %w", err)
	}

	meter := meterProvider.Meter(
		metricsNamespace,
		metric.WithInstrumentationVersion(cfg.AppConfig.ServiceVersion),
	)

	logger.With().Str("component", "metrics")

	provider := &Adapter{
		meterProvider: meterProvider,
		meter:         meter,
		logger:        logger,
	}

	if err := provider.initializeMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return provider, nil
}

func (a *Adapter) Inc(ctx context.Context, key string, value any, attributes ...attribute.KeyValue) {
	var instrument any

	if entry, ok := _metrics.Int64Counters[key]; ok && entry.data != nil {
		instrument = *entry.data
	} else if entry, ok := _metrics.Float64Histograms[key]; ok && entry.data != nil {
		instrument = *entry.data
	} else if entry, ok := _metrics.Int64Histograms[key]; ok && entry.data != nil {
		instrument = *entry.data
	} else if entry, ok := _metrics.Int64Gauges[key]; ok && entry.data != nil {
		instrument = *entry.data
	}

	if instrument == nil {
		a.logger.Warn().
			Str("metric_key", key).
			Msg("Inc called with unknown or uninitialized metric")

		return
	}

	switch inst := instrument.(type) {
	case metric.Int64Counter:
		v, ok := value.(int64)
		if !ok {
			a.logger.Warn().
				Str("metric_key", key).
				Msg("Inc called with non-int64 value for counter")

			return
		}

		inst.Add(ctx, v, metric.WithAttributes(attributes...))
	case metric.Float64Histogram:
		switch v := value.(type) {
		case float64:
			inst.Record(ctx, v, metric.WithAttributes(attributes...))
		case int:
			inst.Record(ctx, float64(v), metric.WithAttributes(attributes...))
		case int32:
			inst.Record(ctx, float64(v), metric.WithAttributes(attributes...))
		case int64:
			inst.Record(ctx, float64(v), metric.WithAttributes(attributes...))
		default:
			a.logger.Warn().
				Str("metric_key", key).
				Msg("Inc called with non-numeric value for float histogram")
		}
	case metric.Int64Histogram:
		v, ok := value.(int64)
		if !ok {
			a.logger.Warn().
				Str("metric_key", key).
				Msg("Inc called with non-int64 value for histogram")

			return
		}

		inst.Record(ctx, v, metric.WithAttributes(attributes...))
	case metric.Int64Gauge:
		v, ok := value.(int64)
		if !ok {
			a.logger.Warn().
				Str("metric_key", key).
				Msg("Inc called with non-int64 value for gauge")

			return
		}

		inst.Record(ctx, v, metric.WithAttributes(attributes...))
	default:
		a.logger.Warn().
			Str("metric_key", key).
			Str("metric_type", fmt.Sprintf("%T", inst)).
			Msg("Inc called with unsupported metric type")
	}
}

func (a *Adapter) Handler() http.Handler {
	return promhttp.Handler()
}

func (a *Adapter) Shutdown(ctx context.Context) error {
	if err := a.meterProvider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}

	return nil
}

func (a *Adapter) initializeMetrics() error {
	for name, target := range _metrics.Float64Histograms {
		histogram, err := metricsKit.RegisterFloat64Histogram(a.meter, target.descriptor, name)
		if err != nil {
			return err
		}

		target.data = &histogram

		_metrics.Float64Histograms[name] = target
	}

	for name, target := range _metrics.Int64Counters {
		counter, err := metricsKit.RegisterInt64Counter(a.meter, target.descriptor, name)
		if err != nil {
			return err
		}

		target.data = &counter

		_metrics.Int64Counters[name] = target
	}

	for name, target := range _metrics.Int64Gauges {
		gauge, err := metricsKit.RegisterInt64Gauge(a.meter, target.descriptor, name)
		if err != nil {
			return err
		}

		target.data = &gauge

		_metrics.Int64Gauges[name] = target
	}

	for name, target := range _metrics.Int64Histograms {
		histogram, err := metricsKit.RegisterInt64Histogram(a.meter, target.descriptor, name)
		if err != nil {
			return err
		}

		target.data = &histogram

		_metrics.Int64Histograms[name] = target
	}

	return nil
}
