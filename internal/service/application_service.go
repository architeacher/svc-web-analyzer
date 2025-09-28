package service

import (
	"context"
	"fmt"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

type (
	ApplicationService interface {
		StartAnalysis(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error)
		FetchAnalysis(ctx context.Context, analysisID string) (*domain.Analysis, error)
		FetchAnalysisEvents(ctx context.Context, analysisID string) (<-chan domain.AnalysisEvent, error)
		FetchReadinessReport(ctx context.Context) (*domain.ReadinessResult, error)
		FetchLivenessReport(ctx context.Context) (*domain.LivenessResult, error)
		FetchHealthReport(ctx context.Context) (*domain.HealthResult, error)
	}

	analysisService struct {
		analysisRepo  ports.AnalysisRepository
		cacheRepo     ports.CacheRepository
		healthChecker ports.HealthChecker
		sseConfig     config.SSEConfig
		logger        *infrastructure.Logger
	}
)

func NewApplicationService(
	analysisRepo ports.AnalysisRepository,
	cacheRepo ports.CacheRepository,
	healthChecker ports.HealthChecker,
	sseConfig config.SSEConfig,
	logger *infrastructure.Logger,
) ApplicationService {
	return analysisService{
		analysisRepo:  analysisRepo,
		cacheRepo:     cacheRepo,
		healthChecker: healthChecker,
		sseConfig:     sseConfig,
		logger:        logger,
	}
}

func (s analysisService) StartAnalysis(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	analysis, err := s.analysisRepo.Save(ctx, url, options)
	if err != nil {
		return nil, err
	}

	if cacheErr := s.cacheRepo.Set(ctx, analysis); cacheErr != nil {
		s.logger.Error().Err(cacheErr).Msg("failed to save analysis to the cache")
	}

	return analysis, nil
}

func (s analysisService) FetchAnalysis(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	analysis, err := s.cacheRepo.Find(ctx, analysisID)
	if err == nil {
		return analysis, nil
	}

	analysis, err = s.analysisRepo.Find(ctx, analysisID)
	if err != nil {
		return nil, fmt.Errorf("failed to find analysis: %w", err)
	}

	// Cache the result for future requests
	if cacheErr := s.cacheRepo.Set(ctx, analysis); cacheErr != nil {
		s.logger.Error().Err(cacheErr).Msg("failed to save analysis to the cache")
	}

	return analysis, nil
}

func (s analysisService) FetchAnalysisEvents(ctx context.Context, analysisID string) (<-chan domain.AnalysisEvent, error) {
	events := make(chan domain.AnalysisEvent, 10)

	go func() {
		defer close(events)

		// Check the analysis status immediately
		analysis, err := s.FetchAnalysis(ctx, analysisID)
		if err != nil {
			return
		}

		if !s.sendAnalysisEvent(analysis, events) {
			return
		}

		keepAliveTicker := time.NewTicker(s.sseConfig.EventsInterval)
		defer keepAliveTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Debug().Str("analysis_id", analysisID).Msg("SSE connection closed by client")

				return
			case <-keepAliveTicker.C:
				analysis, err := s.FetchAnalysis(ctx, analysisID)
				if err != nil {
					return
				}

				if !s.sendAnalysisEvent(analysis, events) {
					return
				}
			}
		}
	}()

	return events, nil
}

func (s analysisService) sendAnalysisEvent(analysis *domain.Analysis, events chan<- domain.AnalysisEvent) bool {
	analysisStatusEventsMap := map[domain.AnalysisStatus]domain.Event{
		domain.StatusRequested:  domain.EventTypeStarted,
		domain.StatusInProgress: domain.EventTypeProgress,
		domain.StatusCompleted:  domain.EventTypeCompleted,
		domain.StatusFailed:     domain.EventTypeFailed,
	}
	keepWaiting := true

	switch analysis.Status {
	case domain.StatusCompleted, domain.StatusFailed:
		keepWaiting = false
	}

	eventType, ok := analysisStatusEventsMap[analysis.Status]
	if !ok {
		keepWaiting = false
	}

	events <- domain.AnalysisEvent{
		Type:    eventType,
		Data:    analysis,
		EventID: analysis.ID.String(),
	}

	return keepWaiting
}

func (s analysisService) FetchReadinessReport(ctx context.Context) (*domain.ReadinessResult, error) {
	return s.healthChecker.CheckReadiness(ctx), nil
}

func (s analysisService) FetchLivenessReport(ctx context.Context) (*domain.LivenessResult, error) {
	return s.healthChecker.CheckLiveness(ctx), nil
}

func (s analysisService) FetchHealthReport(ctx context.Context) (*domain.HealthResult, error) {
	return s.healthChecker.CheckHealth(ctx), nil
}
