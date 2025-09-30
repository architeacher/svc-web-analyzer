package commands

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	AnalyzeCommand struct {
		URL     string                 `json:"url"`
		Options domain.AnalysisOptions `json:"options"`
	}

	AnalyzeCommandHandler decorator.CommandHandler[AnalyzeCommand, *domain.Analysis]

	analyzeCommandHandler struct {
		analysisService service.ApplicationService
		logger          *infrastructure.Logger
	}
)

func NewAnalyzeCommandHandler(
	analysisService service.ApplicationService,
	logger infrastructure.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient decorator.MetricsClient,
) AnalyzeCommandHandler {
	return decorator.ApplyCommandDecorators[AnalyzeCommand, *domain.Analysis](
		analyzeCommandHandler{analysisService: analysisService},
		logger,
		tracerProvider,
		metricsClient,
	)
}

func (h analyzeCommandHandler) Handle(ctx context.Context, cmd AnalyzeCommand) (*domain.Analysis, error) {
	return h.analysisService.StartAnalysis(ctx, cmd.URL, cmd.Options)
}
