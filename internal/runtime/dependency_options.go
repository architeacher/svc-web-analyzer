package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/http"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/outbox"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/queue"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/repos"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/hashicorp/vault/api"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

type (
	DependencyOption func(*Dependencies) error
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithSecretStorage(),
		WithSecretStorageRepo(),
		WithConfigLoader(ctx),
		WithStorage(),
		WithCache(ctx),
		WithDataRepos(),
		WithMetrics(ctx),
		WithTracing(ctx),
		WithDomainServices(),
	}
}

// WithSecretStorage initializes the Vault client using ENV config.
func WithSecretStorage() DependencyOption {
	return func(d *Dependencies) error {
		cfg := d.cfg.SecretStorage

		vaultConfig := api.DefaultConfig()
		vaultConfig.Address = cfg.Address
		vaultConfig.Timeout = cfg.Timeout

		if cfg.TLSSkipVerify {
			tlsConfig := &api.TLSConfig{
				Insecure: true,
			}
			if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to configure TLS: %w", err)
			}
		}

		client, err := api.NewClient(vaultConfig)
		if err != nil {
			return fmt.Errorf("failed to create Vault client: %w", err)
		}

		// Skip namespace configuration for dev mode vault
		if cfg.Namespace != "" {
			client.SetNamespace(cfg.Namespace)
		}

		d.Infra.SecretStorageClient = client

		return nil
	}
}

func WithSecretStorageRepo() DependencyOption {
	return func(d *Dependencies) error {
		d.Repos.SecretStorageRepo = repos.NewVaultRepository(d.Infra.SecretStorageClient)

		return nil
	}
}

func WithConfigLoader(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		d.configLoader = config.NewLoader(d.cfg, d.Repos.SecretStorageRepo, d.secretVersion)

		if !d.cfg.SecretStorage.Enabled {
			d.logger.Info().Msg("secret storage is disabled, skipping vault configuration loading")

			return nil
		}

		version, err := d.configLoader.Load(ctx, d.Repos.SecretStorageRepo, d.cfg)
		if err != nil {
			return fmt.Errorf("unable to load service configuration: %w", err)
		}

		d.secretVersion = version

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

		d.Infra.StorageClient = storage

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

func WithDataRepos() DependencyOption {
	return func(d *Dependencies) error {
		db, err := d.Infra.StorageClient.GetDB()
		if err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		d.Repos.AnalysisRepo = repos.NewAnalysisRepository(db)
		d.Repos.OutboxRepo = repos.NewOutboxRepository(db)
		d.Repos.CacheRepo = repos.NewCacheRepository(
			d.Infra.CacheClient,
			d.cfg.Cache,
			d.logger,
		)

		return nil
	}
}

func WithMetrics(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		metrics, err := infrastructure.NewMetrics(ctx, *d.cfg, d.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize metrics: %w", err)
		}

		d.Infra.Metrics = metrics

		return nil
	}
}

func WithTracing(ctx context.Context) DependencyOption {
	return func(d *Dependencies) error {
		if !d.cfg.Telemetry.Traces.Enabled {
			d.tracerShutdownFunc = func(_ context.Context) error {
				return nil
			}

			return nil
		}

		tracerShutdownFunc, err := infrastructure.InitGlobalTracer(ctx, d.cfg.Telemetry, d.cfg.AppConfig)
		if err != nil {
			d.logger.Error().Err(err).Msg("failed to initialize global tracer")

			return err
		}

		d.tracerShutdownFunc = tracerShutdownFunc

		return nil
	}
}

func WithDomainServices() DependencyOption {
	return func(d *Dependencies) error {
		d.DomainServices = DomainServices{
			WebFetcher:   adapters.NewWebFetcher(d.cfg.WebFetcher, d.logger),
			HTMLAnalyzer: adapters.NewHTMLAnalyzer(d.logger),
			LinkChecker:  adapters.NewLinkChecker(d.cfg.LinkChecker, d.logger, d.Infra.Metrics),
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
			d.Repos.AnalysisRepo,
			d.Repos.OutboxRepo,
			d.Repos.CacheRepo,
			adapters.NewHealthChecker(),
			db,
			d.cfg.SSE,
			d.cfg.Outbox,
			d.logger,
		)

		d.Apps.Web = usecases.NewWebApplication(
			analysisService,
			d.logger,
			otel.GetTracerProvider(),
			adapters.NewMetricsAdapter(d.Infra.Metrics),
		)

		// Create PASETO key service for authentication
		pasetoKeyService := infrastructure.NewPasetoKeyService(
			d.cfg.Auth,
			d.Repos.SecretStorageRepo,
			d.logger,
		)

		requestHandler := http.NewRequestHandler(d.Apps.Web, d.logger)
		httpServer := initHTTPServer(d.cfg, d.logger, d.Infra.Metrics, requestHandler, pasetoKeyService)

		d.Infra.HTTPServer = httpServer

		return nil
	}
}

func WithQueue() DependencyOption {
	return func(d *Dependencies) error {
		queueClient, err := infrastructure.NewQueue(d.cfg.Queue, d.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize queue: %w", err)
		}

		if err := queueClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect to queue: %w", err)
		}

		d.Infra.QueueClient = queueClient

		return nil
	}
}

func WithPublisher() DependencyOption {
	return func(d *Dependencies) error {
		if err := WithQueue()(d); err != nil {
			return err
		}

		if err := d.Infra.QueueClient.DeclareExchange(d.cfg.Queue.ExchangeName, amqp.ExchangeTopic, true, false); err != nil {
			return fmt.Errorf("failed to declare exchange: %w", err)
		}

		if _, err := d.Infra.QueueClient.DeclareQueue(d.cfg.Queue.QueueName, true, false); err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		if err := d.Infra.QueueClient.BindQueue(d.cfg.Queue.QueueName, d.cfg.Queue.RoutingKey, d.cfg.Queue.ExchangeName); err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}

		publisherService := service.NewPublisherService(
			d.Repos.OutboxRepo,
			d.Infra.QueueClient,
			d.cfg.Queue,
			backoff.NewExponentialStrategy(d.cfg.Backoff),
			d.logger,
			d.Infra.Metrics,
		)

		d.Apps.Publisher = usecases.NewPublisherApplication(
			publisherService,
			d.logger,
			otel.GetTracerProvider(),
			adapters.NewMetricsAdapter(d.Infra.Metrics),
		)

		d.Workers.OutboxProcessor = outbox.NewProcessor(
			d.Apps.Publisher,
			d.logger,
		)

		return nil
	}
}

func WithSubscriber() DependencyOption {
	return func(d *Dependencies) error {
		if err := WithQueue()(d); err != nil {
			return err
		}

		subscriberService := service.NewSubscriberService(
			d.Repos.AnalysisRepo,
			d.Repos.OutboxRepo,
			d.Repos.CacheRepo,
			d.DomainServices.WebFetcher,
			d.DomainServices.HTMLAnalyzer,
			d.DomainServices.LinkChecker,
			d.logger,
			d.Infra.Metrics,
		)

		d.Apps.Subscriber = usecases.NewSubscriberApplication(
			subscriberService,
			d.logger,
			otel.GetTracerProvider(),
			adapters.NewMetricsAdapter(d.Infra.Metrics),
		)

		d.Workers.AnalysisWorker = queue.NewAnalysisWorker(
			d.Apps.Subscriber,
			d.logger,
		)

		return nil
	}
}
