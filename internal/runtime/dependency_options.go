package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"go.opentelemetry.io/otel"
)

type (
	DependencyOption func(*Dependencies) error
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithTracing(ctx),
		WithSecretStorage(ctx),
		WithStorage(),
		WithCache(ctx),
		WithDomainServices(),
	}
}

func WithTracing(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		tracerShutdownFunc, err := initGlobalTracing(ctx, d.cfg)
		if err != nil {
			d.logger.Error().Err(err).Msg("failed to initialize global tracer")

			return err
		}

		d.tracerShutdownFunc = tracerShutdownFunc

		return nil
	}
}

func WithSecretStorage(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		secretStorageClient, err := createVaultClient(d.cfg.SecretStorage)
		if err != nil {
			return fmt.Errorf("unable to create vault client: %w", err)
		}

		storageRepo := adapters.NewVaultRepository(secretStorageClient)
		if d.cfg.SecretStorage.Enabled {
			version, err := config.Load(ctx, storageRepo, d.cfg)
			if err != nil {
				return fmt.Errorf("unable to load service configuration: %w", err)
			}
			d.secretVersion = version
		} else {
			d.logger.Info().Msg("secret storage is disabled, skipping vault configuration loading")
		}

		d.Infra.SecretStorageClient = storageRepo

		return nil
	}
}

func WithCache(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		cacheClient := infrastructure.NewKeyDBClient(d.cfg.Cache, d.logger)

		cacheCtx, cancel := context.WithTimeout(ctx, d.cfg.Cache.DialTimeout)
		defer cancel()

		if err := cacheClient.Ping(cacheCtx); err != nil {
			d.logger.Error().Err(err).Msg("failed to connect to cache, continuing without cache")
			d.Infra.CacheClient = nil

			return nil
		}

		d.logger.Info().Msg("cache connection established")
		d.Infra.CacheClient = cacheClient

		return nil
	}
}

func WithStorage() DependencyOption {
	return func(d *Dependencies) error {
		storage, err := infrastructure.NewStorage(d.cfg.Storage)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		if _, err := storage.GetDB(); err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		d.Infra.StorageClient = *storage

		return nil
	}
}

func WithDomainServices() DependencyOption {
	return func(d *Dependencies) error {
		d.DomainServices = DomainServices{
			WebFetcher:   adapters.NewWebPageFetcher(d.cfg.WebFetcher, d.logger),
			HTMLAnalyzer: adapters.NewHTMLAnalyzer(d.logger),
			LinkChecker:  adapters.NewLinkChecker(d.cfg.LinkChecker, d.logger),
		}

		return nil
	}
}

func WithHTTPServer() DependencyOption {
	return func(d *Dependencies) error {
		db, err := d.Infra.StorageClient.GetDB()
		if err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		analysisService := service.NewApplicationService(
			adapters.NewAnalysisRepository(db),
			adapters.NewOutboxRepository(db),
			adapters.NewCacheRepository(
				d.Infra.CacheClient,
				d.cfg.Cache,
				d.logger,
			),
			adapters.NewHealthChecker(),
			db,
			d.cfg.SSE,
			d.cfg.Outbox,
			d.logger,
		)

		app := usecases.NewApplication(
			analysisService,
			d.logger,
			otel.GetTracerProvider(),
			infrastructure.NoOp{},
		)

		requestHandler := adapters.NewRequestHandler(app, d.logger)
		httpServer := initHTTPServer(d.cfg, d.logger, requestHandler)

		d.Infra.HTTPServer = httpServer

		return nil
	}
}
