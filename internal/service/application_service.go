package service

import (
	"context"
	"fmt"

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
		logger        *infrastructure.Logger
	}
)

func NewApplicationService(
	analysisRepo ports.AnalysisRepository,
	cacheRepo ports.CacheRepository,
	healthChecker ports.HealthChecker,
	logger *infrastructure.Logger,
) ApplicationService {
	return analysisService{
		analysisRepo:  analysisRepo,
		cacheRepo:     cacheRepo,
		healthChecker: healthChecker,
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
	// Create a channel for events
	events := make(chan domain.AnalysisEvent)

	// Start a goroutine to send events
	go func() {
		defer close(events)

		// Check if analysis exists
		analysis, err := s.FetchAnalysis(ctx, analysisID)
		if err != nil {
			return
		}

		// Send appropriate event based on analysis status
		switch analysis.Status {
		case domain.StatusRequested:
			events <- domain.AnalysisEvent{
				Type:    domain.EventTypeStarted,
				Data:    analysis,
				EventID: analysis.ID.String(),
			}
		case domain.StatusInProgress:
			events <- domain.AnalysisEvent{
				Type:    domain.EventTypeProgress,
				Data:    analysis,
				EventID: analysis.ID.String(),
			}
		case domain.StatusCompleted:
			events <- domain.AnalysisEvent{
				Type:    domain.EventTypeCompleted,
				Data:    analysis,
				EventID: analysis.ID.String(),
			}
		case domain.StatusFailed:
			events <- domain.AnalysisEvent{
				Type:    domain.EventTypeFailed,
				Data:    analysis,
				EventID: analysis.ID.String(),
			}
		}
	}()

	return events, nil
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
