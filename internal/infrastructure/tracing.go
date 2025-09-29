package infrastructure

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	exporterTypeGRPC   = "grpc"
	exporterTypeStdOut = "stdout"
)

func InitGlobalTracer(ctx context.Context, cfgTelemetry config.Telemetry, cfgApp config.AppConfig) (shutdown func(context.Context) error, err error) {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	traceExporter, err := createExporter(ctx, cfgTelemetry)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	hostName, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get host name: %w", err)
	}

	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfgApp.ServiceName),
		semconv.ServiceVersion(cfgApp.ServiceVersion),
		attribute.String("env", cfgApp.Env),
		attribute.String("host", hostName),
		attribute.String("commit_sha", cfgApp.CommitSHA),
		attribute.String("X-Product-Cluster", cfgTelemetry.OtelProductCluster),
	)

	sampler := trace.TraceIDRatioBased(cfgTelemetry.Traces.SamplerRatio)
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(resource),
		trace.WithSampler(trace.ParentBased(
			sampler,
			trace.WithRemoteParentSampled(sampler),
		)),
	)
	otel.SetTracerProvider(traceProvider)

	return traceProvider.Shutdown, nil
}

func createExporter(ctx context.Context, cfg config.Telemetry) (exporter trace.SpanExporter, err error) {
	switch strings.ToLower(cfg.ExporterType) {
	case exporterTypeGRPC:
		exporter, err = createGRPCExporter(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC exporter: %w", err)
		}
	case exporterTypeStdOut:
		exporter, err = createStdOutExporter()
		if err != nil {
			return nil, fmt.Errorf("failed to create StdOut exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exporer type %q", cfg.ExporterType)
	}

	return exporter, nil
}

func createGRPCExporter(ctx context.Context, cfg config.Telemetry) (*otlptrace.Exporter, error) {
	conn, err := grpc.NewClient(
		net.JoinHostPort(cfg.OtelGRPCHost, cfg.OtelGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a gRPC client connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create a gRPC trace exporter: %w", err)
	}

	return traceExporter, nil
}

func createStdOutExporter() (*stdouttrace.Exporter, error) {
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("failed to create an StdOut trace exporter: %w", err)
	}

	return traceExporter, nil
}
