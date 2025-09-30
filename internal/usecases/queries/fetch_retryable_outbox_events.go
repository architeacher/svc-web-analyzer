package queries

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"go.opentelemetry.io/otel/trace"
)

type (
	FetchRetryableOutboxEventsQuery struct {
		BatchSize int
	}

	FetchRetryableOutboxEventsQueryHandler decorator.QueryHandler[FetchRetryableOutboxEventsQuery, []*domain.OutboxEvent]

	fetchRetryableOutboxEventsQueryHandler struct {
		publisherService service.PublisherService
	}
)

func NewFetchRetryableOutboxEventsQueryHandler(
	publisherService service.PublisherService,
	logger appLogger.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) FetchRetryableOutboxEventsQueryHandler {
	return decorator.ApplyQueryDecorators[FetchRetryableOutboxEventsQuery, []*domain.OutboxEvent](
		fetchRetryableOutboxEventsQueryHandler{
			publisherService: publisherService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchRetryableOutboxEventsQueryHandler) Execute(
	ctx context.Context,
	query FetchRetryableOutboxEventsQuery,
) ([]*domain.OutboxEvent, error) {
	return h.publisherService.FetchRetryableEvents(ctx, query.BatchSize)
}
