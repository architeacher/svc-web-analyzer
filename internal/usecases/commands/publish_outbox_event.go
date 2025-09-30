package commands

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	PublishOutboxEventCommand struct {
		Event *domain.OutboxEvent
	}

	PublishOutboxEventHandler decorator.CommandHandler[PublishOutboxEventCommand, *domain.PublishOutboxEventResult]

	publishOutboxEventHandler struct {
		publisherService service.PublisherService
		logger           infrastructure.Logger
	}
)

func NewPublishOutboxEventHandler(
	publisherService service.PublisherService,
	logger infrastructure.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient decorator.MetricsClient,
) PublishOutboxEventHandler {
	return decorator.ApplyCommandDecorators[PublishOutboxEventCommand, *domain.PublishOutboxEventResult](
		publishOutboxEventHandler{
			publisherService: publisherService,
			logger:           logger,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h publishOutboxEventHandler) Handle(
	ctx context.Context,
	cmd PublishOutboxEventCommand,
) (*domain.PublishOutboxEventResult, error) {
	return h.publisherService.PublishEvent(ctx, cmd.Event)
}
