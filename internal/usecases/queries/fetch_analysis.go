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
	FetchAnalysisQuery struct {
		AnalysisID string
	}

	FetchAnalysisQueryHandler decorator.QueryHandler[FetchAnalysisQuery, *domain.Analysis]

	fetchAnalysisQueryHandler struct {
		appService service.ApplicationService
	}
)

func NewFetchAnalysisQueryHandler(
	appService service.ApplicationService,
	logger infrastructure.Logger,
	tracerProvider trace.TracerProvider,
	metricsClient decorator.MetricsClient,
) decorator.QueryHandler[FetchAnalysisQuery, *domain.Analysis] {
	return decorator.ApplyQueryDecorators[FetchAnalysisQuery, *domain.Analysis](
		fetchAnalysisQueryHandler{
			appService: appService,
		},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h fetchAnalysisQueryHandler) Execute(ctx context.Context, query FetchAnalysisQuery) (*domain.Analysis, error) {
	return h.appService.FetchAnalysis(ctx, query.AnalysisID)
}
