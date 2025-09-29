package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/architeacher/svc-web-analyzer/internal/config"
)

type ServiceCtx struct {
	deps *Dependencies

	reloadConfigChannel chan os.Signal
	shutdownChannel     chan os.Signal

	serverCtx      context.Context
	serverStopFunc context.CancelFunc

	serverReady chan struct{}
}

func New(opt ...Option) *ServiceCtx {
	if len(opt) != 0 {
		sCtx := ServiceCtx{}

		for i := range opt {
			opt[i](&sCtx)
		}

		return &sCtx
	}

	return &ServiceCtx{
		shutdownChannel:     make(chan os.Signal, 1),
		reloadConfigChannel: make(chan os.Signal, 1),
	}
}

func (c *ServiceCtx) Run() {
	c.build()
	c.startService()
	c.wait()
	c.monitorConfigChanges()
	c.shutdown()
}

// build initializes the service components
func (c *ServiceCtx) build() {
	c.serverCtx, c.serverStopFunc = context.WithCancel(context.Background())

	deps, err := initializeDependencies(c.serverCtx)
	if err != nil {
		panic(fmt.Errorf("ailed to initialize dependencies: %w", err))
	}

	c.deps = deps
}

// startService starts the HTTP server
func (c *ServiceCtx) startService() {
	c.deps.logger.Info().
		Str("address", net.JoinHostPort(c.deps.cfg.HTTPServer.Host, fmt.Sprintf("%d", c.deps.cfg.HTTPServer.Port))).
		Msg("service starting up")

	if c.serverReady != nil {
		c.serverReady <- struct{}{}
	}

	if err := c.deps.Infra.HTTPServer.ListenAndServe(); err != nil {
		c.deps.logger.Fatal().Err(err).Msg("unable to start http server")
	}

	<-c.serverCtx.Done()
}

func (c *ServiceCtx) wait() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(c.reloadConfigChannel, syscall.SIGHUP)
}

func (c *ServiceCtx) monitorConfigChanges() {
	go func() {
		for {
			select {
			case <-c.serverCtx.Done():
				c.deps.logger.Info().Msg("stopping config monitor")
				return

			case <-c.reloadConfigChannel:
				c.deps.logger.Info().Msg("received config reload signal")

				if err := config.Load(c.serverCtx, c.deps.Infra.SecretStorageClient, c.deps.cfg); err != nil {
					c.deps.logger.Error().Err(err).Msg("failed to reload config")
					continue
				}

				c.deps.logger.Info().Msg("config reloaded successfully")
			}
		}
	}()
}

func (c *ServiceCtx) shutdown() {
	go func() {
		<-c.shutdownChannel
		defer close(c.shutdownChannel)
		defer close(c.reloadConfigChannel)

		c.deps.logger.Info().Msg("received shutdown signal")

		defer c.cleanup()

		// Shutdown signal with a grace period of 30 seconds
		shutdownCtx, cancel := context.WithTimeout(c.serverCtx, c.deps.cfg.HTTPServer.ShutdownTimeout)

		go func() {
			<-shutdownCtx.Done()

			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				c.deps.logger.Error().Msg("graceful shutdown timed out.. forcing exit.")
				cancel()
				os.Exit(1)
			}
		}()

		// Trigger graceful shutdown of the http server
		if err := c.deps.Infra.HTTPServer.Shutdown(shutdownCtx); err != nil {
			c.deps.logger.Error().Err(err).Msg("unable to gracefully shutdown http server")
		}

		if err := c.deps.tracerShutdownFunc(shutdownCtx); err != nil {
			c.deps.logger.Error().Err(err).
				Msg("unable to gracefully shutdown global tracer")
		}

		c.serverStopFunc()

		c.deps.logger.Info().Msg("HTTP server shutdown completed")
	}()
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

func (c *ServiceCtx) cleanup() {
	c.deps.logger.Info().Msg("cleaning up resources...")

	if c.deps.Infra.CacheClient != nil {
		if err := c.deps.Infra.CacheClient.Close(); err != nil {
			c.deps.logger.Error().Err(err).Msg("failed to close cache connection")
		}
	}

	c.deps.logger.Info().Msg("Cleanup completed")
}
