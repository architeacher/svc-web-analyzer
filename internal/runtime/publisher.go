package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/pkg/queue"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

const (
	OutboxProcessorInterval = 5 * time.Second
	BatchSize               = 10
	BaseRetryDelay          = 5 * time.Second
)

type OutboxProcessor struct {
	outboxRepo ports.OutboxRepository
	queue      infrastructure.Queue
	logger     *infrastructure.Logger
	config     config.QueueConfig
}

func NewOutboxProcessor(
	outboxRepo ports.OutboxRepository,
	queue infrastructure.Queue,
	logger *infrastructure.Logger,
	config config.QueueConfig,
) *OutboxProcessor {
	return &OutboxProcessor{
		outboxRepo: outboxRepo,
		queue:      queue,
		logger:     logger,
		config:     config,
	}
}

func (p *OutboxProcessor) Start(ctx context.Context) error {
	p.logger.Info().Msg("Starting outbox processor")

	if err := p.setupInfrastructure(); err != nil {
		return fmt.Errorf("failed to setup queue infrastructure: %w", err)
	}

	ticker := time.NewTicker(OutboxProcessorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("Outbox processor shutting down")
			return ctx.Err()

		case <-ticker.C:
			if err := p.processPendingEvents(ctx); err != nil {
				p.logger.Error().Err(err).Msg("failed to process pending events")
			}

			if err := p.processRetryableEvents(ctx); err != nil {
				p.logger.Error().Err(err).Msg("failed to process retryable events")
			}
		}
	}
}

func (p *OutboxProcessor) setupInfrastructure() error {
	if err := p.queue.DeclareExchange(p.config.ExchangeName, "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	if _, err := p.queue.DeclareQueue(p.config.QueueName, true, false); err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	if err := p.queue.BindQueue(p.config.QueueName, p.config.RoutingKey, p.config.ExchangeName); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	p.logger.Info().
		Str("exchange", p.config.ExchangeName).
		Str("queue", p.config.QueueName).
		Msg("Queue infrastructure setup completed")

	return nil
}

func (p *OutboxProcessor) processPendingEvents(ctx context.Context) error {
	events, err := p.outboxRepo.FindPending(ctx, BatchSize)
	if err != nil {
		return fmt.Errorf("failed to find pending events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	p.logger.Debug().Int("count", len(events)).Msg("Processing pending outbox events")

	for _, event := range events {
		if err := p.processEvent(ctx, event); err != nil {
			p.logger.Error().
				Err(err).
				Str("event_id", event.ID.String()).
				Msg("failed to process pending event")
		}
	}

	return nil
}

func (p *OutboxProcessor) processRetryableEvents(ctx context.Context) error {
	events, err := p.outboxRepo.FindRetryable(ctx, BatchSize)
	if err != nil {
		return fmt.Errorf("failed to find retryable events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	p.logger.Debug().Int("count", len(events)).Msg("Processing retryable outbox events")

	for _, event := range events {
		if err := p.processEvent(ctx, event); err != nil {
			p.logger.Error().
				Err(err).
				Str("event_id", event.ID.String()).
				Msg("failed to process retryable event")
		}
	}

	return nil
}

func (p *OutboxProcessor) processEvent(ctx context.Context, event *domain.OutboxEvent) error {
	claimedEvent, err := p.outboxRepo.ClaimForProcessing(ctx, event.ID.String())
	if err != nil {
		p.logger.Debug().
			Str("event_id", event.ID.String()).
			Msg("failed to claim event (might be claimed by another processor)")
		return nil
	}

	routingKey := string(claimedEvent.EventType)
	if err := p.queue.Publish(ctx, p.config.ExchangeName, routingKey, claimedEvent.Payload); err != nil {
		p.handlePublishFailure(ctx, claimedEvent, err)

		return fmt.Errorf("failed to publish event: %w", err)
	}

	if err := p.outboxRepo.MarkPublished(ctx, claimedEvent.ID.String()); err != nil {
		p.logger.Error().
			Err(err).
			Str("event_id", claimedEvent.ID.String()).
			Msg("failed to mark event as published")

		return fmt.Errorf("failed to mark event as published: %w", err)
	}

	p.logger.Debug().
		Str("event_id", claimedEvent.ID.String()).
		Str("event_type", string(claimedEvent.EventType)).
		Str("routing_key", routingKey).
		Msg("Successfully published outbox event")

	return nil
}

func (p *OutboxProcessor) handlePublishFailure(ctx context.Context, event *domain.OutboxEvent, publishErr error) {
	errorDetails := publishErr.Error()

	if event.RetryCount >= event.MaxRetries {
		if err := p.outboxRepo.MarkPermanentlyFailed(ctx, event.ID.String(), errorDetails); err != nil {
			p.logger.Error().
				Err(err).
				Str("event_id", event.ID.String()).
				Msg("failed to mark event as permanently failed")
		} else {
			p.logger.Warn().
				Str("event_id", event.ID.String()).
				Int("retry_count", event.RetryCount).
				Msg("Event permanently failed after max retries")
		}
		return
	}

	nextRetryAt := adapters.CalculateNextRetryTime(event.RetryCount, BaseRetryDelay)

	if err := p.outboxRepo.MarkFailed(ctx, event.ID.String(), errorDetails, &nextRetryAt); err != nil {
		p.logger.Error().
			Err(err).
			Str("event_id", event.ID.String()).
			Msg("failed to mark event as failed")
	} else {
		p.logger.Debug().
			Str("event_id", event.ID.String()).
			Int("retry_count", event.RetryCount+1).
			Time("next_retry_at", nextRetryAt).
			Msg("Event scheduled for retry")
	}
}

type PublisherCtx struct {
	processor *OutboxProcessor
	logger    *infrastructure.Logger
	queue     infrastructure.Queue
	storage   *infrastructure.Storage

	shutdownChannel chan os.Signal
	ctx             context.Context
	cancelFunc      context.CancelFunc
}

func NewPublisher() *PublisherCtx {
	return &PublisherCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}
}

func (c *PublisherCtx) Run() {
	c.build()
	c.start()
	c.wait()
	c.shutdown()
}

func (c *PublisherCtx) build() {
	c.ctx, c.cancelFunc = context.WithCancel(context.Background())

	cfg, err := config.Init()
	if err != nil {
		panic(fmt.Errorf("failed to load configuration: %w", err))
	}

	c.logger = infrastructure.New(cfg.Logging)

	c.storage, err = infrastructure.NewStorage(cfg.Storage)
	if err != nil {
		c.logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	outboxRepo := adapters.NewOutboxRepository(c.storage)

	queueConfig := queue.Config{
		Scheme:   "amqp",
		Username: cfg.Queue.Username,
		Password: cfg.Queue.Password,
		Host:     cfg.Queue.Host,
		Port:     cfg.Queue.Port,
		Vhost:    cfg.Queue.VirtualHost,
	}

	c.queue = queue.NewRabbitMQQueue(
		queueConfig,
		queue.WithLogger(queue.NewLoggerAdapter(c.logger)),
		queue.WithReconnectDelay(5*time.Second),
	)

	if err := c.queue.Connect(); err != nil {
		c.logger.Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}

	c.processor = NewOutboxProcessor(outboxRepo, c.queue, c.logger, cfg.Queue)
}

func (c *PublisherCtx) start() {
	c.logger.Info().Msg("Starting outbox publisher service")

	go func() {
		if err := c.processor.Start(c.ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Fatal().Err(err).Msg("Outbox processor failed")
		}
	}()
}

func (c *PublisherCtx) wait() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
	<-c.shutdownChannel
}

func (c *PublisherCtx) shutdown() {
	c.logger.Info().Msg("Received shutdown signal")
	defer c.cleanup()

	c.cancelFunc()
	c.logger.Info().Msg("Outbox publisher service stopped")
}

func (c *PublisherCtx) cleanup() {
	c.logger.Info().Msg("cleaning up resources...")

	if c.queue != nil {
		if err := c.queue.Close(); err != nil {
			c.logger.Error().Err(err).Msg("failed to close queue")
		}
	}

	if c.storage != nil {
		if err := c.storage.Close(); err != nil {
			c.logger.Error().Err(err).Msg("failed to close storage")
		}
	}

	c.logger.Info().Msg("cleanup completed")
}
