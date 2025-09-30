package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type ServiceCtx struct {
	deps *dependencies

	shutdownChannel chan os.Signal

	serverCtx      context.Context
	serverStopFunc context.CancelFunc

	serverReady chan struct{}
}

func New(opt ...ServiceOption) *ServiceCtx {
	sCtx := ServiceCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}

	for i := range opt {
		opt[i](&sCtx)
	}

	return &sCtx
}

func (c *ServiceCtx) Run() {
	c.build()
	c.startService()
	c.monitorConfigChanges()
	c.shutdownHook()
	c.shutdown()
}

// build initializes the service components
func (c *ServiceCtx) build() {
	c.serverCtx, c.serverStopFunc = context.WithCancel(context.Background())

	deps, err := initializeDependencies(c.serverCtx, WithHTTPServer(), WithProfiler())
	if err != nil {
		log.Fatalf("failed to initialize dependencies: %v\n", err)
	}

	c.deps = deps
}

// startService starts the HTTP server
func (c *ServiceCtx) startService() {
	// Start HTTP server
	go func() {
		c.deps.infra.logger.Info().
			Str("address", net.JoinHostPort(c.deps.cfg.HTTPServer.Host, fmt.Sprintf("%d", c.deps.cfg.HTTPServer.Port))).
			Msg("service starting up")

		if c.serverReady != nil {
			c.serverReady <- struct{}{}
		}

		if err := c.deps.infra.httpServer.ListenAndServe(); err != nil {
			c.deps.infra.logger.Fatal().Err(err).Msg("unable to start http server")
			c.serverStopFunc()

			return
		}
	}()

	if c.deps.infra.profiler == nil {
		return
	}

	if err := c.deps.infra.profiler.Start(); err != nil {
		c.deps.infra.logger.Error().Err(err).Msg("failed to start the profiler")
		os.Exit(1)
	}

	defer func() {
		if err := c.deps.infra.profiler.Stop(); err != nil {
			c.deps.infra.logger.Err(err).Msg("failed to stop the profiler")
		}
	}()
}

func (c *ServiceCtx) shutdownHook() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (c *ServiceCtx) monitorConfigChanges() {
	reloadErrors := c.deps.configLoader.WatchConfigSignals(c.serverCtx)

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

func (c *ServiceCtx) shutdown() {
	// Waits for one of the following shutdown conditions to happen.
	select {
	case <-c.serverCtx.Done():
	case <-c.shutdownChannel:
		defer close(c.shutdownChannel)
	}

	c.deps.infra.logger.Info().Msg("received shutdown signal")

	// Cancel context that underlying processes would start cleanup.
	c.serverStopFunc()

	// Shutdown signal with a grace period of 30 seconds.
	shutdownCtx, cancel := context.WithTimeout(c.serverCtx, c.deps.cfg.HTTPServer.ShutdownTimeout)

	go func() {
		<-shutdownCtx.Done()

		if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
			c.deps.infra.logger.Error().Msg("graceful shutdown timed out.. forcing exit.")
			cancel()
			os.Exit(1)
		}
	}()

	c.cleanup(shutdownCtx)

	c.deps.infra.logger.Info().Msg("HTTP server shutdown completed")
}

// WaitForServer blocks until the http server is running.
// If you want to be notified when the server is running,
// make sure you instantiate your server with WithWaitingForServer.
//
// Example:
//
//	srv := runtime.New(WithWaitingForServer())
//	go func() {
//		srv.Run()
//	}()
//
//	srv.WaitForServer()
func (c *ServiceCtx) WaitForServer() {
	if c.serverReady != nil {
		<-c.serverReady
		close(c.serverReady)
	}
}

func (c *ServiceCtx) cleanup(shutdownCtx context.Context) {
	c.deps.infra.logger.Info().Msg("cleaning up resources...")

	if c.deps.infra.cacheClient != nil {
		if err := c.deps.infra.cacheClient.Close(); err != nil {
			c.deps.infra.logger.Error().Err(err).Msg("failed to close cache connection")
		}
	}

	// Trigger graceful shutdown of the http server
	if err := c.deps.infra.httpServer.Shutdown(shutdownCtx); err != nil {
		c.deps.infra.logger.Error().Err(err).Msg("unable to gracefully shutdown http server")
	}

	c.deps.infra.logger.Info().Msg("cleanup completed")
}
