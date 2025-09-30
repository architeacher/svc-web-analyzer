package decorator

import (
	"context"
	"fmt"
	"strings"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	Command any

	CommandHandler[C Command, R any] interface {
		Handle(context.Context, C) (R, error)
	}
)

func ApplyCommandDecorators[C Command, R any](
	handler CommandHandler[C, R],
	logger appLogger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient ports.MetricsClient,
) CommandHandler[C, R] {
	return commandLoggingDecorator[C, R]{
		base: commandTracingDecorator[C, R]{
			base: commandMetricsDecorator[C, R]{
				base:   handler,
				client: metricsClient,
			},
			tracerProvider: tracerProvider,
		},
		logger: logger,
	}
}

func generateActionName(handler any) string {
	return strings.Split(fmt.Sprintf("%T", handler), ".")[1]
}
