package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

type SubscriberCtx struct {
	deps *dependencies

	shutdownChannel chan os.Signal

	backgroundActorCtx      context.Context
	backgroundActorStopFunc context.CancelFunc
}

func NewSubscriber(opt ...SubscriberOption) *SubscriberCtx {
	if len(opt) != 0 {
		sCtx := SubscriberCtx{}

		for i := range opt {
			opt[i](&sCtx)
		}

		return &sCtx
	}

	return &SubscriberCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}
}

func (c *SubscriberCtx) Run() {
	c.build()
	c.start()
	c.monitorConfigChanges()
	c.shutdownHook()
	c.shutdown()
}

func (c *SubscriberCtx) build() {
	c.backgroundActorCtx, c.backgroundActorStopFunc = context.WithCancel(context.Background())

	deps, err := initializeDependencies(c.backgroundActorCtx, WithSubscriber())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to initialize dependencies: %v\n", err)
		os.Exit(1)
	}

	c.deps = deps
}

func (c *SubscriberCtx) start() {
	go func() {
		c.deps.infra.logger.Info().
			Str("queue", c.deps.cfg.Queue.QueueName).
			Msg("starting outbox subscriber service")

		err := c.deps.infra.queueClient.Consume(
			c.backgroundActorCtx,
			c.deps.cfg.Queue.QueueName,
			"analysis-worker",
			c.deps.workers.analysisWorker.ProcessMessage,
			queue.WithConsumingLogger(queue.NewLoggerAdapter(c.deps.infra.logger)),
			queue.WithErrorHandler(func(err error) {
				c.deps.infra.logger.Error().Err(err).Msg("consumer error")
			}),
		)

		if err != nil && !errors.Is(err, context.Canceled) {
			c.deps.infra.logger.Fatal().Err(err).Msg("analysis worker failed")
		}
	}()
}

func (c *SubscriberCtx) shutdownHook() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (c *SubscriberCtx) monitorConfigChanges() {
	reloadErrors := c.deps.configLoader.WatchConfigSignals(c.backgroundActorCtx)

	go func() {
		for err := range reloadErrors {
			if err != nil {
				c.deps.infra.logger.Error().Err(err).Msg("failed to reload config")
				continue
			}

			c.deps.infra.logger.Info().Msg("config reloaded successfully")
		}

		c.deps.infra.logger.Info().Msg("stopping config monitor")
	}()
}

func (c *SubscriberCtx) shutdown() {
	// Waits for one of the following shutdown conditions to happen.
	select {
	case <-c.backgroundActorCtx.Done():
	case <-c.shutdownChannel:
		defer close(c.shutdownChannel)
	}

	c.deps.infra.logger.Info().Msg("received shutdown signal")

	// Cancel context that underlying processes would start cleanup
	c.backgroundActorStopFunc()

	c.deps.infra.logger.Info().Msg("analysis worker service stopped")
}

func (c *SubscriberCtx) cleanup() {
	c.deps.infra.logger.Info().Msg("cleaning up resources...")

	if err := c.deps.infra.storageClient.Close(); err != nil {
		c.deps.infra.logger.Error().Err(err).Msg("failed to close storage")
	}

	if c.deps.infra.queueClient != nil {
		if err := c.deps.infra.queueClient.Close(); err != nil {
			c.deps.infra.logger.Error().Err(err).Msg("failed to close queue")
		}
	}

	if c.deps.infra.cacheClient != nil {
		if err := c.deps.infra.cacheClient.Close(); err != nil {
			c.deps.infra.logger.Error().Err(err).Msg("failed to close cache")
		}
	}

	c.deps.infra.logger.Info().Msg("cleanup completed")
}
