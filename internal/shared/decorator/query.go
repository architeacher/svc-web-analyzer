package decorator

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	Query  any
	Result any

	QueryHandler[Q Query, R Result] interface {
		Execute(ctx context.Context, query Q) (R, error)
	}
)

func ApplyQueryDecorators[Q Query, R Result](
	handler QueryHandler[Q, R],
	logger infrastructure.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient MetricsClient,
) QueryHandler[Q, R] {
	return queryLoggingDecorator[Q, R]{
		base: queryTracingDecorator[Q, R]{
			base: queryMetricsDecorator[Q, R]{
				base:   handler,
				client: metricsClient,
			},
			tracerProvider: tracerProvider,
		},
		logger: logger,
	}
}
