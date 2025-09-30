package usecases

import (
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/commands"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	SubscriberApplication struct {
		Commands SubscriberCommands
	}

	SubscriberCommands struct {
		ProcessAnalysisMessageHandler commands.ProcessAnalysisMessageHandler
	}
)

func NewSubscriberApplication(
	subscriberService service.SubscriberService,
	logger appLogger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient decorator.MetricsClient,
) *SubscriberApplication {
	return &SubscriberApplication{
		Commands: SubscriberCommands{
			ProcessAnalysisMessageHandler: commands.NewProcessAnalysisMessageHandler(
				subscriberService,
				logger,
				tracerProvider,
				metricsClient,
			),
		},
	}
}
