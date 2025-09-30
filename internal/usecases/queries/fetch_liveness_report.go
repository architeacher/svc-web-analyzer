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
	FetchLivenessReportQuery struct{}

	FetchLivenessReportQueryHandler decorator.QueryHandler[FetchLivenessReportQuery, *domain.LivenessResult]

	fetchLivenessReportQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchLivenessReportQueryHandler(appService service.ApplicationService,
	logger infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) decorator.QueryHandler[FetchLivenessReportQuery, *domain.LivenessResult] {
	return decorator.ApplyQueryDecorators[FetchLivenessReportQuery, *domain.LivenessResult](
		fetchLivenessReportQueryHandler{
			appService: appService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchLivenessReportQueryHandler) Execute(ctx context.Context, query FetchLivenessReportQuery) (*domain.LivenessResult, error) {
	return h.appService.FetchLivenessReport(ctx)
}
