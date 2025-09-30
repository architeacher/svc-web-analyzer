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
	FetchReadinessReportQuery struct{}

	FetchReadinessReportQueryHandler decorator.QueryHandler[FetchReadinessReportQuery, *domain.ReadinessResult]

	fetchReadinessReportQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchReadinessReportQueryHandler(appService service.ApplicationService,
	logger appLogger.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient metricsKit.Client,
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
