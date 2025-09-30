package commands

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/decorator"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type (
	AnalyzeCommand struct {
		URL     string
		Options domain.AnalysisOptions
	}

	AnalyzeCommandHandler decorator.CommandHandler[AnalyzeCommand, *domain.Analysis]

	analyzeCommandHandler struct {
		analysisService service.ApplicationService
		logger          appLogger.Logger
	}
)

func NewAnalyzeCommandHandler(
	analysisService service.ApplicationService,
	logger appLogger.Logger,
	tracerProvider otelTrace.TracerProvider,
	metricsClient ports.MetricsClient,
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
