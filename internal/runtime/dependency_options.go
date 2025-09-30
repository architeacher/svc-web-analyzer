package runtime

import (
	"context"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/http"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/metrics"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/outbox"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/queue"
	reposAdapter "github.com/architeacher/svc-web-analyzer/internal/adapters/repos"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/shared/backoff"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/hashicorp/vault/api"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

type (
	DependencyOption func(*dependencies) error
)

func defaultOptions(ctx context.Context) []DependencyOption {
	return []DependencyOption{
		WithSecretStorage(),
		WithLogger(),
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
	return func(d *dependencies) error {
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

		d.infra.secretStorageClient = client

		return nil
	}
}

func WithLogger() DependencyOption {
	return func(d *dependencies) error {
		d.infra.logger = appLogger.New(d.cfg.Logging.Level, d.cfg.Logging.Format)

		return nil
	}
}

func WithSecretStorageRepo() DependencyOption {
	return func(d *dependencies) error {
		d.repos.secretStorageRepo = reposAdapter.NewVaultRepository(d.infra.secretStorageClient)

		return nil
	}
}

func WithConfigLoader(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		d.configLoader = config.NewLoader(d.cfg, d.repos.secretStorageRepo, d.secretVersion)

		if !d.cfg.SecretStorage.Enabled {
			d.infra.logger.Info().Msg("secret storage is disabled, skipping vault configuration loading")

			return nil
		}

		version, err := d.configLoader.Load(ctx, d.repos.secretStorageRepo, d.cfg)
		if err != nil {
			return fmt.Errorf("unable to load service configuration: %w", err)
		}

		d.secretVersion = version

		return nil
	}
}

func WithStorage() DependencyOption {
	return func(d *dependencies) error {
		storage, err := infrastructure.NewStorage(d.cfg.Storage)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		if _, err := storage.GetDB(); err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		d.infra.storageClient = storage

		return nil
	}
}

func WithCache(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		cacheClient := infrastructure.NewKeyDBClient(d.cfg.Cache, d.infra.logger)

		cacheCtx, cancel := context.WithTimeout(ctx, d.cfg.Cache.DialTimeout)
		defer cancel()

		if err := cacheClient.Ping(cacheCtx); err != nil {
			d.infra.logger.Error().Err(err).Msg("failed to connect to cache, continuing without cache")
			d.infra.cacheClient = nil

			return nil
		}

		d.infra.logger.Info().Msg("cache connection established")
		d.infra.cacheClient = cacheClient

		return nil
	}
}

func WithDataRepos() DependencyOption {
	return func(d *dependencies) error {
		db, err := d.infra.storageClient.GetDB()
		if err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		d.repos.analysisRepo = reposAdapter.NewAnalysisRepository(db)
		d.repos.outboxRepo = reposAdapter.NewOutboxRepository(db)
		d.repos.cacheRepo = reposAdapter.NewCacheRepository(
			d.infra.cacheClient,
			d.cfg.Cache,
			d.infra.logger,
		)

		return nil
	}
}

func WithMetrics(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		metricsClient, err := metrics.NewMetricsAdapter(ctx, *d.cfg, d.infra.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize metricsClient: %w", err)
		}

		d.infra.metricsClient = metricsClient

		return nil
	}
}

func WithTracing(ctx context.Context) DependencyOption {
	return func(d *dependencies) error {
		if !d.cfg.Telemetry.Traces.Enabled {
			d.infra.tracerShutdownFunc = func(_ context.Context) error {
				return nil
			}

			return nil
		}

		tracerShutdownFunc, err := infrastructure.InitGlobalTracer(ctx, d.cfg.Telemetry, d.cfg.AppConfig)
		if err != nil {
			d.infra.logger.Error().Err(err).Msg("failed to initialize global tracer")

			return err
		}

		d.infra.tracer = otel.GetTracerProvider()
		d.infra.tracerShutdownFunc = tracerShutdownFunc

		return nil
	}
}

func WithDomainServices() DependencyOption {
	return func(d *dependencies) error {
		d.domainServices = domainService{
			webFetcher:   adapters.NewWebFetcher(d.cfg.WebFetcher, d.infra.cacheClient, d.infra.logger, d.infra.metricsClient),
			htmlAnalyzer: adapters.NewHTMLAnalyzer(d.infra.logger),
			linkChecker:  adapters.NewLinkChecker(d.cfg.LinkChecker, d.infra.logger, d.infra.metricsClient),
		}

		return nil
	}
}

func WithHTTPServer() DependencyOption {
	return func(d *dependencies) error {
		db, err := d.infra.storageClient.GetDB()
		if err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}

		analysisService := service.NewApplicationService(
			d.repos.analysisRepo,
			d.repos.outboxRepo,
			d.repos.cacheRepo,
			adapters.NewHealthChecker(),
			db,
			d.cfg.SSE,
			d.cfg.Outbox,
			d.infra.logger,
		)

		d.apps.web = usecases.NewWebApplication(
			analysisService,
			d.infra.logger,
			d.infra.tracer,
			d.infra.metricsClient,
		)

		// Create PASETO key service for authentication
		pasetoKeyService := infrastructure.NewPasetoKeyService(
			d.cfg.Auth,
			d.repos.secretStorageRepo,
			d.infra.logger,
		)

		requestHandler := http.NewRequestHandler(d.apps.web, d.infra.logger)
		httpServer := initHTTPServer(d.cfg, d.infra.logger, d.infra.metricsClient, requestHandler, pasetoKeyService)

		d.infra.httpServer = httpServer

		return nil
	}
}

func WithQueue() DependencyOption {
	return func(d *dependencies) error {
		queueClient, err := infrastructure.NewQueue(d.cfg.Queue, d.infra.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize queue: %w", err)
		}

		if err := queueClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect to queue: %w", err)
		}

		d.infra.queueClient = queueClient

		return nil
	}
}

func WithProfiler() DependencyOption {
	return func(d *dependencies) error {
		d.infra.profiler = infrastructure.NewProfiler(d.cfg.Profiler, d.infra.logger)

		return nil
	}
}

func WithPublisher() DependencyOption {
	return func(d *dependencies) error {
		if err := WithQueue()(d); err != nil {
			return err
		}

		if err := d.infra.queueClient.DeclareExchange(d.cfg.Queue.ExchangeName, amqp.ExchangeTopic, true, false); err != nil {
			return fmt.Errorf("failed to declare exchange: %w", err)
		}

		if _, err := d.infra.queueClient.DeclareQueue(d.cfg.Queue.QueueName, true, false); err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		if err := d.infra.queueClient.BindQueue(d.cfg.Queue.QueueName, d.cfg.Queue.RoutingKey, d.cfg.Queue.ExchangeName); err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}

		publisherService := service.NewPublisherService(
			d.repos.outboxRepo,
			d.infra.queueClient,
			d.cfg.Queue,
			backoff.NewExponentialStrategy(d.cfg.Backoff),
			d.infra.logger,
			d.infra.metricsClient,
		)

		d.apps.publisher = usecases.NewPublisherApplication(
			publisherService,
			d.infra.logger,
			d.infra.tracer,
			d.infra.metricsClient,
		)

		d.workers.outboxProcessor = outbox.NewProcessor(
			d.apps.publisher,
			d.infra.logger,
		)

		return nil
	}
}

func WithSubscriber() DependencyOption {
	return func(d *dependencies) error {
		if err := WithQueue()(d); err != nil {
			return err
		}

		subscriberService := service.NewSubscriberService(
			d.repos.analysisRepo,
			d.repos.outboxRepo,
			d.repos.cacheRepo,
			d.domainServices.webFetcher,
			d.domainServices.htmlAnalyzer,
			d.domainServices.linkChecker,
			d.infra.logger,
			d.infra.metricsClient,
		)

		d.apps.subscriber = usecases.NewSubscriberApplication(
			subscriberService,
			d.infra.logger,
			d.infra.tracer,
			d.infra.metricsClient,
		)

		d.workers.analysisWorker = queue.NewAnalysisWorker(
			d.apps.subscriber,
			d.infra.logger,
		)

		return nil
	}
}
