package queries

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	"go.opentelemetry.io/otel/trace"
)

type (
	FetchPendingOutboxEventsQuery struct {
		BatchSize int
	}

	FetchPendingOutboxEventsQueryHandler decorator.QueryHandler[FetchPendingOutboxEventsQuery, []*domain.OutboxEvent]

	fetchPendingOutboxEventsQueryHandler struct {
		publisherService service.PublisherService
	}
)

func NewFetchPendingOutboxEventsQueryHandler(
	publisherService service.PublisherService,
	logger infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) FetchPendingOutboxEventsQueryHandler {
	return decorator.ApplyQueryDecorators[FetchPendingOutboxEventsQuery, []*domain.OutboxEvent](
		fetchPendingOutboxEventsQueryHandler{
			publisherService: publisherService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchPendingOutboxEventsQueryHandler) Execute(
	ctx context.Context,
	query FetchPendingOutboxEventsQuery,
) ([]*domain.OutboxEvent, error) {
	return h.publisherService.FetchPendingEvents(ctx, query.BatchSize)
}
