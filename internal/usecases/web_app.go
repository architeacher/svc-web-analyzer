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
	WebApplication struct {
		Commands Commands
		Queries  Queries
	}

	Commands struct {
		AnalyzeCommandHandler commands.AnalyzeCommandHandler
	}

	Queries struct {
		FetchAnalysisQueryHandler        queries.FetchAnalysisQueryHandler
		FetchAnalysisEventsQueryHandler  queries.FetchAnalysisEventsQueryHandler
		FetchReadinessReportQueryHandler queries.FetchReadinessReportQueryHandler
		FetchLivenessReportQueryHandler  queries.FetchLivenessReportQueryHandler
		FetchHealthReportQueryHandler    queries.FetchHealthReportQueryHandler
	}
)

func NewWebApplication(
	appService service.ApplicationService,
	logger infrastructure.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient decorator.MetricsClient,
) *WebApplication {
	return &WebApplication{
		Commands: Commands{
			AnalyzeCommandHandler: commands.NewAnalyzeCommandHandler(appService, logger, tracerProvider, metricsClient),
		},
		Queries: Queries{
			FetchAnalysisQueryHandler: queries.NewFetchAnalysisQueryHandler(
				appService, logger, tracerProvider, metricsClient,
			),
			FetchAnalysisEventsQueryHandler: queries.NewFetchAnalysisEventsQueryHandler(
				appService, logger, tracerProvider, metricsClient,
			),
			FetchReadinessReportQueryHandler: queries.NewFetchReadinessReportQueryHandler(
				appService, logger, tracerProvider, metricsClient,
			),
			FetchLivenessReportQueryHandler: queries.NewFetchLivenessReportQueryHandler(
				appService, logger, tracerProvider, metricsClient,
			),
			FetchHealthReportQueryHandler: queries.NewFetchHealthReportQueryHandler(
				appService, logger, tracerProvider, metricsClient,
			),
		},
	}
}
