package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type PublisherCtx struct {
	deps *Dependencies

	shutdownChannel chan os.Signal

	backgroundActorCtx      context.Context
	backgroundActorStopFunc context.CancelFunc
}

func NewPublisher(opt ...PublisherOption) *PublisherCtx {
	if len(opt) != 0 {
		pCtx := PublisherCtx{}

		for i := range opt {
			opt[i](&pCtx)
		}

		return &pCtx
	}

	return &PublisherCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}
}

func (c *PublisherCtx) Run() {
	c.build()
	c.start()
	c.monitorConfigChanges()
	c.shutdownHook()
	c.shutdown()
}

func (c *PublisherCtx) build() {
	c.backgroundActorCtx, c.backgroundActorStopFunc = context.WithCancel(context.Background())

	deps, err := initializeDependencies(c.backgroundActorCtx, WithPublisher())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to initialize dependencies: %v\n", err)
		os.Exit(1)
	}

	c.deps = deps
}

func (c *PublisherCtx) start() {
	go func() {
		c.deps.logger.Info().Msg("starting outbox publisher service")

		if err := c.deps.Workers.OutboxProcessor.Start(c.backgroundActorCtx); err != nil && !errors.Is(err, context.Canceled) {
			c.deps.logger.Fatal().Err(err).Msg("outbox processor failed")
		}
	}()
}

func (c *PublisherCtx) shutdownHook() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (c *PublisherCtx) monitorConfigChanges() {
	reloadErrors := c.deps.configLoader.WatchConfigSignals(c.backgroundActorCtx)

	go func() {
		for err := range reloadErrors {
			if err != nil {
				c.deps.logger.Error().Err(err).Msg("failed to reload config")
				continue
			}

			c.deps.logger.Info().Msg("config reloaded successfully")
		}

		c.deps.logger.Info().Msg("stopping config monitor")
	}()
}

func (c *PublisherCtx) shutdown() {
	// Waits for one of the following shutdown conditions to happen.
	select {
	case <-c.backgroundActorCtx.Done():
	case <-c.shutdownChannel:
		defer close(c.shutdownChannel)
	}

	c.deps.logger.Info().Msg("received shutdown signal")

	// Cancel context that underlying processes would start cleanup
	c.backgroundActorStopFunc()

	c.cleanup()

	c.deps.logger.Info().Msg("outbox publisher service stopped")
}

func (c *PublisherCtx) cleanup() {
	c.deps.logger.Info().Msg("cleaning up resources...")

	if c.deps.Infra.QueueClient != nil {
		if err := c.deps.Infra.QueueClient.Close(); err != nil {
			c.deps.logger.Error().Err(err).Msg("failed to close queue")
		}
	}

	if err := c.deps.Infra.StorageClient.Close(); err != nil {
		c.deps.logger.Error().Err(err).Msg("failed to close storage")
	}

	c.deps.logger.Info().Msg("cleanup completed")
}
