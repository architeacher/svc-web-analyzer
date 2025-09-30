package queries

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	metricsKit "github.com/architeacher/svc-web-analyzer/pkg/metrics"
	"go.opentelemetry.io/otel/trace"
)

type (
	FetchAnalysisEventsQuery struct {
		AnalysisID string
	}

	FetchAnalysisEventsQueryHandler decorator.QueryHandler[FetchAnalysisEventsQuery, <-chan domain.AnalysisEvent]

	fetchAnalysisEventsQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchAnalysisEventsQueryHandler(
	appService service.ApplicationService,
	logger appLogger.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient metricsKit.Client,
) decorator.QueryHandler[FetchAnalysisEventsQuery, <-chan domain.AnalysisEvent] {
	return decorator.ApplyQueryDecorators[FetchAnalysisEventsQuery, <-chan domain.AnalysisEvent](
		fetchAnalysisEventsQueryHandler{
			appService: appService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchAnalysisEventsQueryHandler) Execute(ctx context.Context, q FetchAnalysisEventsQuery) (<-chan domain.AnalysisEvent, error) {
	return h.appService.FetchAnalysisEvents(ctx, q.AnalysisID)
}
