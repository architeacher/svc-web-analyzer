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
	FetchReadinessReportQuery struct{}

	FetchReadinessReportQueryHandler decorator.QueryHandler[FetchReadinessReportQuery, *domain.ReadinessResult]

	fetchReadinessReportQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchReadinessReportQueryHandler(appService service.ApplicationService,
	logger infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) decorator.QueryHandler[FetchReadinessReportQuery, *domain.ReadinessResult] {
	return decorator.ApplyQueryDecorators[FetchReadinessReportQuery, *domain.ReadinessResult](
		fetchReadinessReportQueryHandler{
			appService: appService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchReadinessReportQueryHandler) Execute(ctx context.Context, query FetchReadinessReportQuery) (*domain.ReadinessResult, error) {
	return h.appService.FetchReadinessReport(ctx)
}
