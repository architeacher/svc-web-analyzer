package outbox

import (
	"context"
	"sync"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/commands"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/queries"
)

const (
	ProcessorInterval = 5 * time.Second
	BatchSize         = 10
)

// Ensure Processor implements the BackgroundProcessor interface
var _ ports.BackgroundProcessor = (*Processor)(nil)

type Processor struct {
	app    *usecases.PublisherApplication
	logger infrastructure.Logger
}

func NewProcessor(
	app *usecases.PublisherApplication,
	logger infrastructure.Logger,
) *Processor {
	return &Processor{
		app:    app,
		logger: logger,
	}
}

func (p *Processor) Start(ctx context.Context) error {
	p.logger.Info().Msg("starting outbox processor")

	ticker := time.NewTicker(ProcessorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("outbox processor shutting down")

			return ctx.Err()

		case <-ticker.C:
			var wg sync.WaitGroup

			wg.Go(func() {
				if err := p.processPendingEvents(ctx); err != nil {
					p.logger.Error().Err(err).Msg("failed to process pending events")
				}
			})

			wg.Go(func() {
				if err := p.processRetryableEvents(ctx); err != nil {
					p.logger.Error().Err(err).Msg("failed to process retryable events")
				}
			})

			wg.Wait()
		}
	}
}

func (p *Processor) processPendingEvents(ctx context.Context) error {
	events, err := p.app.Queries.FetchPendingOutboxEventsQueryHandler.Execute(ctx, queries.FetchPendingOutboxEventsQuery{
		BatchSize: BatchSize,
	})
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	p.logger.Debug().Int("count", len(events)).Msg("processing pending outbox events")

	var wg sync.WaitGroup

	for _, event := range events {
		wg.Go(func() {
			if _, err := p.app.Commands.PublishOutboxEventHandler.Handle(ctx, commands.PublishOutboxEventCommand{
				Event: event,
			}); err != nil {
				p.logger.Error().
					Err(err).
					Str("event_id", event.ID.String()).
					Msg("failed to process pending event")
			}
		})
	}

	wg.Wait()

	return nil
}

func (p *Processor) processRetryableEvents(ctx context.Context) error {
	events, err := p.app.Queries.FetchRetryableOutboxEventsQueryHandler.Execute(ctx, queries.FetchRetryableOutboxEventsQuery{
		BatchSize: BatchSize,
	})
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	p.logger.Debug().Int("count", len(events)).Msg("processing retryable outbox events")

	var wg sync.WaitGroup

	for _, event := range events {
		wg.Go(func() {
			if _, err := p.app.Commands.PublishOutboxEventHandler.Handle(ctx, commands.PublishOutboxEventCommand{
				Event: event,
			}); err != nil {
				p.logger.Error().
					Err(err).
					Str("event_id", event.ID.String()).
					Msg("failed to process retryable event")
			}
		})
	}

	wg.Wait()

	return nil
}
