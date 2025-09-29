package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

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
		outboxRepo    ports.OutboxRepository
		healthChecker ports.HealthChecker
		storageClient *infrastructure.Storage
		sseConfig     config.SSEConfig
		logger        *infrastructure.Logger
	}
)

func NewApplicationService(
	analysisRepo ports.AnalysisRepository,
	cacheRepo ports.CacheRepository,
	outboxRepo ports.OutboxRepository,
	healthChecker ports.HealthChecker,
	storageClient *infrastructure.Storage,
	sseConfig config.SSEConfig,
	logger *infrastructure.Logger,
) ApplicationService {
	return analysisService{
		analysisRepo:  analysisRepo,
		cacheRepo:     cacheRepo,
		outboxRepo:    outboxRepo,
		healthChecker: healthChecker,
		storageClient: storageClient,
		sseConfig:     sseConfig,
		logger:        logger,
	}
}

func (s analysisService) StartAnalysis(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	if s.storageClient == nil {
		return nil, fmt.Errorf("failed to get database connection: storage client not initialized")
	}

	db, err := s.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	analysis, err := s.analysisRepo.SaveInTx(tx, url, options)
	if err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	outboxEvent := &domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateID:   analysis.ID,
		AggregateType: "analysis",
		EventType:     domain.OutboxEventAnalysisRequested,
		Priority:      domain.PriorityNormal,
		RetryCount:    0,
		MaxRetries:    3,
		Status:        domain.OutboxStatusPending,
		Payload: domain.AnalysisRequestPayload{
			AnalysisID: analysis.ID,
			URL:        url,
			Options:    options,
			Priority:   domain.PriorityNormal,
			CreatedAt:  analysis.CreatedAt,
		},
		CreatedAt: analysis.CreatedAt,
	}

	if err := s.outboxRepo.SaveInTx(tx, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to save outbox event: %w", err)
	}

	// Commit transaction - ensures atomicity of analysis + outbox event
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Cache the analysis (fire-and-forget, not part of transaction)
	if cacheErr := s.cacheRepo.Set(ctx, analysis); cacheErr != nil {
		s.logger.Error().Err(cacheErr).Msg("failed to save analysis to the cache")
	}

	s.logger.Info().
		Str("analysis_id", analysis.ID.String()).
		Str("url", url).
		Str("outbox_event_id", outboxEvent.ID.String()).
		Msg("Successfully created analysis and outbox event")

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

	if cacheErr := s.cacheRepo.Set(ctx, analysis); cacheErr != nil {
		s.logger.Error().Err(cacheErr).Msg("failed to save analysis to the cache")
	}

	return analysis, nil
}

func (s analysisService) FetchAnalysisEvents(ctx context.Context, analysisID string) (<-chan domain.AnalysisEvent, error) {
	events := make(chan domain.AnalysisEvent, 10)

	go func() {
		defer close(events)

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
