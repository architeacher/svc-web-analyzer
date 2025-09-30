package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

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

	appService struct {
		analysisRepo  ports.AnalysisRepository
		outboxRepo    ports.OutboxRepository
		cacheRepo     ports.CacheRepository
		healthChecker ports.HealthChecker
		db            *sqlx.DB
		sseConfig     config.SSEConfig
		outboxConfig  config.OutboxConfig
		logger        infrastructure.Logger
	}
)

func NewApplicationService(
	analysisRepo ports.AnalysisRepository,
	outboxRepo ports.OutboxRepository,
	cacheRepo ports.CacheRepository,
	healthChecker ports.HealthChecker,
	db *sqlx.DB,
	sseConfig config.SSEConfig,
	outboxConfig config.OutboxConfig,
	logger infrastructure.Logger,
) ApplicationService {
	return &appService{
		analysisRepo:  analysisRepo,
		outboxRepo:    outboxRepo,
		cacheRepo:     cacheRepo,
		healthChecker: healthChecker,
		db:            db,
		sseConfig:     sseConfig,
		outboxConfig:  outboxConfig,
		logger:        logger,
	}
}

func (s *appService) StartAnalysis(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.logger.Error().Err(rollbackErr).Msg("failed to rollback transaction")
		}
	}()

	analysis, err := s.analysisRepo.SaveInTx(ctx, tx, url, options)
	if err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	priority := domain.PriorityNormal
	maxRetries := s.outboxConfig.GetMaxRetriesForPriority(string(priority))

	outboxEvent := &domain.OutboxEvent{
		ID:            uuid.Nil,
		AggregateID:   analysis.ID,
		AggregateType: "analysis",
		EventType:     domain.OutboxEventAnalysisRequested,
		Priority:      priority,
		RetryCount:    0,
		MaxRetries:    maxRetries,
		Status:        domain.OutboxStatusPending,
		Payload: domain.AnalysisRequestPayload{
			AnalysisID: analysis.ID,
			URL:        url,
			Options:    options,
			Priority:   priority,
			CreatedAt:  analysis.CreatedAt,
		},
		CreatedAt: analysis.CreatedAt,
	}

	if err := s.outboxRepo.SaveInTx(ctx, tx, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to save outbox event: %w", err)
	}

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

func (s *appService) FetchAnalysis(ctx context.Context, analysisID string) (*domain.Analysis, error) {
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

func (s *appService) FetchAnalysisEvents(ctx context.Context, analysisID string) (<-chan domain.AnalysisEvent, error) {
	events := make(chan domain.AnalysisEvent, 10)
	checkAnalysisChan := make(chan struct{}, 1)

	go func() {
		defer close(events)
		defer close(checkAnalysisChan)

		keepAliveTicker := time.NewTicker(s.sseConfig.HeartbeatInterval)
		eventsTicker := time.NewTicker(s.sseConfig.EventsInterval)
		defer func() {
			keepAliveTicker.Stop()
			eventsTicker.Stop()
		}()

		// Send the initial event if it has been processed and don't wait for the ticker.
		checkAnalysisChan <- struct{}{}

		for {
			select {
			case <-ctx.Done():
				s.logger.Debug().Str("analysis_id", analysisID).Msg("SSE connection closed by client")

				return
			case <-keepAliveTicker.C:
				const eventType = "heartbeat"
				s.sendHeartEvent(eventType, map[string]any{}, events)

			case <-eventsTicker.C:
				checkAnalysisChan <- struct{}{}

			case <-checkAnalysisChan:
				analysis, err := s.FetchAnalysis(ctx, analysisID)
				if err != nil {
					return
				}

				eventType := s.getEventStatus(analysis.Status)
				if !s.shouldWait(s.getEventStatus(analysis.Status)) {
					// Analysis completed, give the client more time to receive the event.
					const parsingDuration = 500 * time.Millisecond

					<-time.After(parsingDuration)
					s.sendAnalysisEvent(eventType, analysis, events)

					return
				}
			}
		}
	}()

	return events, nil
}

func (s *appService) shouldWait(eventType domain.Event) bool {
	return eventType != domain.EventTypeCompleted && eventType != domain.EventTypeFailed
}

func (s *appService) sendAnalysisEvent(
	eventType domain.Event,
	analysis *domain.Analysis,
	events chan<- domain.AnalysisEvent,
) {
	events <- domain.AnalysisEvent{
		Type:    eventType,
		Payload: analysis,
		EventID: analysis.ID.String(),
	}
}

func (s *appService) sendHeartEvent(
	eventType domain.Event,
	payload any,
	events chan<- domain.AnalysisEvent,
) {
	events <- domain.AnalysisEvent{
		Type:    eventType,
		Payload: payload,
	}
}

func (s *appService) getEventStatus(status domain.AnalysisStatus) domain.Event {
	analysisStatusEventsMap := map[domain.AnalysisStatus]domain.Event{
		domain.StatusRequested:  domain.EventTypeStarted,
		domain.StatusInProgress: domain.EventTypeProgress,
		domain.StatusCompleted:  domain.EventTypeCompleted,
		domain.StatusFailed:     domain.EventTypeFailed,
	}

	return analysisStatusEventsMap[status]
}

func (s *appService) FetchReadinessReport(ctx context.Context) (*domain.ReadinessResult, error) {
	return s.healthChecker.CheckReadiness(ctx), nil
}

func (s *appService) FetchLivenessReport(ctx context.Context) (*domain.LivenessResult, error) {
	return s.healthChecker.CheckLiveness(ctx), nil
}

func (s *appService) FetchHealthReport(ctx context.Context) (*domain.HealthResult, error) {
	return s.healthChecker.CheckHealth(ctx), nil
}
