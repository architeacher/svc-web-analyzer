package infrastructure

import (
	"context"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewOTELMeterProvider(
	ctx context.Context,
	cfg config.ServiceConfig,
	logger appLogger.Logger,
) (*sdkmetric.MeterProvider, error) {
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

	logger.Info().
		Str("otel_endpoint", endpoint).
		Msg("OTEL meter provider initialized successfully")

	return meterProvider, nil
}
