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
	FetchHealthReportQuery struct{}

	FetchHealthReportQueryHandler decorator.QueryHandler[FetchHealthReportQuery, *domain.HealthResult]

	fetchHealthReportQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchHealthReportQueryHandler(appService service.ApplicationService,
	logger *infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) decorator.QueryHandler[FetchHealthReportQuery, *domain.HealthResult] {
	return decorator.ApplyQueryDecorators[FetchHealthReportQuery, *domain.HealthResult](
		fetchHealthReportQueryHandler{
			appService: appService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchHealthReportQueryHandler) Execute(ctx context.Context, query FetchHealthReportQuery) (*domain.HealthResult, error) {
	return h.appService.FetchHealthReport(ctx)
}
