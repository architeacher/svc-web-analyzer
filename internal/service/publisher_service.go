package service

import (
	"context"
	"fmt"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
)

type (
	PublisherService interface {
		FetchPendingEvents(ctx context.Context, batchSize int) ([]*domain.OutboxEvent, error)
		FetchRetryableEvents(ctx context.Context, batchSize int) ([]*domain.OutboxEvent, error)
		PublishEvent(ctx context.Context, event *domain.OutboxEvent) (*domain.PublishOutboxEventResult, error)
	}

	publisherService struct {
		outboxRepo      ports.OutboxRepository
		queue           infrastructure.Queue
		queueConfig     config.QueueConfig
		backoffStrategy backoff.Strategy
		logger          infrastructure.Logger
		metrics         infrastructure.Metrics
	}
)

func NewPublisherService(
	outboxRepo ports.OutboxRepository,
	queue infrastructure.Queue,
	queueConfig config.QueueConfig,
	backoffStrategy backoff.Strategy,
	logger infrastructure.Logger,
	metrics infrastructure.Metrics,
) PublisherService {
	return publisherService{
		outboxRepo:      outboxRepo,
		queue:           queue,
		queueConfig:     queueConfig,
		backoffStrategy: backoffStrategy,
		logger:          logger,
		metrics:         metrics,
	}
}

func (s publisherService) FetchPendingEvents(ctx context.Context, batchSize int) ([]*domain.OutboxEvent, error) {
	return s.outboxRepo.FindPending(ctx, batchSize)
}

func (s publisherService) FetchRetryableEvents(ctx context.Context, batchSize int) ([]*domain.OutboxEvent, error) {
	return s.outboxRepo.FindRetryable(ctx, batchSize)
}

func (s publisherService) PublishEvent(ctx context.Context, event *domain.OutboxEvent) (*domain.PublishOutboxEventResult, error) {
	claimedEvent, err := s.outboxRepo.ClaimForProcessing(ctx, event.ID.String())
	if err != nil {
		s.logger.Debug().
			Str("event_id", event.ID.String()).
			Msg("failed to claim event for processing")

		return &domain.PublishOutboxEventResult{
			Published: false,
			Error:     fmt.Sprintf("failed to claim event: %v", err),
		}, nil
	}

	routingKey := string(claimedEvent.EventType)
	if err := s.queue.Publish(ctx, s.queueConfig.ExchangeName, routingKey, claimedEvent.Payload); err != nil {
		if handleErr := s.handlePublishFailure(ctx, claimedEvent, err); handleErr != nil {
			s.logger.Error().
				Err(handleErr).
				Str("event_id", claimedEvent.ID.String()).
				Msg("failed to handle publish failure")
		}

		s.logger.Debug().
			Str("event_id", claimedEvent.ID.String()).
			Msg("failed to publish event to queue")

		return &domain.PublishOutboxEventResult{
			Published: false,
			Error:     fmt.Sprintf("failed to publish to queue: %v", err),
		}, nil
	}

	if err := s.outboxRepo.MarkPublished(ctx, claimedEvent.ID.String()); err != nil {
		return &domain.PublishOutboxEventResult{
			Published: false,
			Error:     fmt.Sprintf("failed to mark as published: %v", err),
		}, nil
	}

	s.metrics.RecordOutboxEvent(ctx, true, string(claimedEvent.Priority))

	s.logger.Debug().
		Str("event_id", claimedEvent.ID.String()).
		Str("event_type", string(claimedEvent.EventType)).
		Msg("successfully published outbox event")

	return &domain.PublishOutboxEventResult{Published: true}, nil
}

func (s publisherService) handlePublishFailure(ctx context.Context, event *domain.OutboxEvent, publishErr error) error {
	errorDetails := publishErr.Error()

	s.metrics.RecordOutboxEvent(ctx, false, string(event.Priority))

	if event.RetryCount >= event.MaxRetries {
		if err := s.outboxRepo.MarkPermanentlyFailed(ctx, event.ID.String(), errorDetails); err != nil {
			return fmt.Errorf("failed to mark event as permanently failed: %w", err)
		}

		s.logger.Warn().
			Str("event_id", event.ID.String()).
			Int("retry_count", event.RetryCount).
			Msg("event permanently failed after max retries")

		return nil
	}

	backoffDuration := s.backoffStrategy.Backoff(event.RetryCount)
	nextRetryAt := time.Now().Add(backoffDuration)

	if err := s.outboxRepo.MarkFailed(ctx, event.ID.String(), errorDetails, &nextRetryAt); err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	s.logger.Debug().
		Str("event_id", event.ID.String()).
		Int("retry_count", event.RetryCount+1).
		Time("next_retry_at", nextRetryAt).
		Msg("event scheduled for retry")

	return nil
}
