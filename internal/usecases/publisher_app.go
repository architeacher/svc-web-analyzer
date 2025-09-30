package usecases

import (
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/commands"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/queries"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	PublisherApplication struct {
		Commands PublisherCommands
		Queries  PublisherQueries
	}

	PublisherCommands struct {
		PublishOutboxEventHandler commands.PublishOutboxEventHandler
	}

	PublisherQueries struct {
		FetchPendingOutboxEventsQueryHandler   queries.FetchPendingOutboxEventsQueryHandler
		FetchRetryableOutboxEventsQueryHandler queries.FetchRetryableOutboxEventsQueryHandler
	}
)

func NewPublisherApplication(
	publisherService service.PublisherService,
	logger infrastructure.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient decorator.MetricsClient,
) *PublisherApplication {
	return &PublisherApplication{
		Commands: PublisherCommands{
			PublishOutboxEventHandler: commands.NewPublishOutboxEventHandler(
				publisherService,
				logger,
				tracerProvider,
				metricsClient,
			),
		},
		Queries: PublisherQueries{
			FetchPendingOutboxEventsQueryHandler: queries.NewFetchPendingOutboxEventsQueryHandler(
				publisherService,
				logger,
				tracerProvider,
				metricsClient,
			),
			FetchRetryableOutboxEventsQueryHandler: queries.NewFetchRetryableOutboxEventsQueryHandler(
				publisherService,
				logger,
				tracerProvider,
				metricsClient,
			),
		},
	}
}
