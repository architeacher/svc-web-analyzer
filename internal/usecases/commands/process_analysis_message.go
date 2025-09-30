package commands

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	"go.opentelemetry.io/otel/trace"
)

type (
	ProcessAnalysisMessageCommand struct {
		Payload domain.AnalysisRequestPayload
	}

	ProcessAnalysisMessageHandler decorator.CommandHandler[ProcessAnalysisMessageCommand, *domain.ProcessAnalysisMessageResult]

	processAnalysisMessageHandler struct {
		subscriberService service.SubscriberService
		logger            infrastructure.Logger
	}
)

func NewProcessAnalysisMessageHandler(
	subscriberService service.SubscriberService,
	logger infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) ProcessAnalysisMessageHandler {
	return decorator.ApplyCommandDecorators[ProcessAnalysisMessageCommand, *domain.ProcessAnalysisMessageResult](
		processAnalysisMessageHandler{
			subscriberService: subscriberService,
			logger:            logger,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h processAnalysisMessageHandler) Handle(
	ctx context.Context,
	cmd ProcessAnalysisMessageCommand,
) (*domain.ProcessAnalysisMessageResult, error) {
	return h.subscriberService.ProcessAnalysisRequest(ctx, cmd.Payload)
}
