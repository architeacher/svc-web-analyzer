package queries

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/metrics"
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
	logger appLogger.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient metrics.Client,
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
