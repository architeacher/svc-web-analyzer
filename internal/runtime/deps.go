package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/middleware"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/handlers"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/service"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/vault/api"
	"go.opentelemetry.io/otel"
)

type (
	InfrastructureDeps struct {
		SecretStorageClient ports.SecretsRepository
		HTTPServer          *http.Server
		StorageClient       infrastructure.Storage
		QueueClient         infrastructure.Queue
		CacheClient         *infrastructure.KeydbClient
	}

	DomainServices struct {
		WebFetcher   ports.WebPageFetcher
		HTMLAnalyzer domain.HTMLAnalyzer
		LinkChecker  ports.LinkChecker
	}

	Dependencies struct {
		cfg    *config.ServiceConfig
		logger *infrastructure.Logger

		tracerShutdownFunc TracerShutdownFunc

		Infra InfrastructureDeps

		DomainServices DomainServices
	}
)

func initializeDependencies(ctx context.Context) (*Dependencies, error) {
	cfg, err := config.Init()
	if err != nil {
		panic(fmt.Errorf("unable to load service configuration: %w", err))
	}

	appLogger := infrastructure.New(config.LoggingConfig{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	appLogger.Info().Msg("initializing dependencies...")

	tracerShutdownFunc, err := initGlobalTracing(ctx, cfg)
	if err != nil {
		appLogger.Error().Err(err).Msg("failed to initialize global tracer")
	}

	secretStorageClient, err := createVaultClient(cfg.SecretStorage)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("unable to create vault client")
	}

	storageRepo := adapters.NewVaultRepository(secretStorageClient)
	if cfg.SecretStorage.Enabled {
		if err := config.Load(ctx, storageRepo, cfg); err != nil {
			appLogger.Fatal().Err(err).Msg("unable to load service configuration")
		}
	} else {
		appLogger.Info().Msg("secret storage is disabled, skipping vault configuration loading")
	}

	// Initialize cache
	cacheClient := infrastructure.NewKeyDBClient(cfg.Cache, appLogger)

	// Test cache connection
	ctx, cancel := context.WithTimeout(ctx, cfg.Cache.DialTimeout)
	defer cancel()

	if err := cacheClient.Ping(ctx); err != nil {
		appLogger.Fatal().Err(err).Msg("failed to connect to cache, continuing without cache")
		cacheClient = nil
	} else {
		appLogger.Info().Msg("cache connection established")
	}

	storage, err := infrastructure.NewStorage(cfg.Storage)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("failed to initialize storage")
	}

	analysisService := service.NewApplicationService(
		adapters.NewPostgresRepository(storage),
		adapters.NewCacheRepository(
			cacheClient,
			cfg.Cache,
			appLogger,
		),
		adapters.NewHealthChecker(),
		cfg.SSE,
		appLogger,
	)

	app := usecases.NewApplication(
		analysisService,
		appLogger,
		otel.GetTracerProvider(),
		infrastructure.NoOp{},
	)

	requestHandler := adapters.NewRequestHandler(app, appLogger)

	httpServer := initHTTPServer(cfg, appLogger, requestHandler)

	webFetcher := adapters.NewWebPageFetcher(cfg.WebFetcher, appLogger)

	linkChecker := adapters.NewLinkChecker(cfg.LinkChecker, appLogger)

	htmlAnalyzer := adapters.NewHTMLAnalyzer(appLogger)

	appLogger.Info().Msg("dependencies initialized successfully")

	return &Dependencies{
		cfg:                cfg,
		logger:             appLogger,
		tracerShutdownFunc: tracerShutdownFunc,
		Infra: InfrastructureDeps{
			SecretStorageClient: adapters.NewVaultRepository(secretStorageClient),
			HTTPServer:          httpServer,
			CacheClient:         cacheClient,
		},
		DomainServices: DomainServices{
			WebFetcher:   webFetcher,
			HTMLAnalyzer: htmlAnalyzer,
			LinkChecker:  linkChecker,
		},
	}, nil
}

func initHTTPServer(cfg *config.ServiceConfig, logger *infrastructure.Logger, reqHandler ports.RequestHandler) *http.Server {
	logger.Info().Msg("creating HTTP server...")

	router := chi.NewRouter()

	middlewares := initMiddlewares(cfg, logger)

	// Spin up automatic generated routes
	handlers.HandlerWithOptions(reqHandler, handlers.ChiServerOptions{
		BaseURL:          "",
		BaseRouter:       router,
		Middlewares:      middlewares,
		ErrorHandlerFunc: nil,
	})

	server := &http.Server{
		Addr:         net.JoinHostPort(cfg.HTTPServer.Host, fmt.Sprintf("%d", cfg.HTTPServer.Port)),
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.ReadTimeout,
		WriteTimeout: cfg.HTTPServer.WriteTimeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	logger.Info().Str("addr", server.Addr).Msg("HTTP server created")

	return server
}

func initMiddlewares(cfg *config.ServiceConfig, logger *infrastructure.Logger) []handlers.MiddlewareFunc {
	swagger, err := handlers.GetSwagger()
	if err != nil {
		logger.Fatal().Err(err).Msg("error loading swagger spec")
	}

	swagger.Servers = nil

	requestValidator := middleware.OapiRequestValidatorWithOptions(logger, swagger, &middleware.RequestValidatorOptions{
		Options: openapi3filter.Options{
			MultiError:         false,
			AuthenticationFunc: middleware.NewPasetoAuthenticationFunc(cfg.Auth, logger),
		},
		ErrorHandler:          middleware.RequestValidationErrHandler,
		SilenceServersWarning: true,
	})

	// Middlewares only applied to the automatic generated routes
	middlewares := []handlers.MiddlewareFunc{
		// Add basic middleware
		chimiddleware.RequestID,
		chimiddleware.RealIP,
		chimiddleware.Logger,
		chimiddleware.Recoverer,
		chimiddleware.Timeout(cfg.HTTPServer.WriteTimeout),
		middleware.NewSecurityHeadersMiddleware().Middleware,
		middleware.NewAPIVersionMiddleware(cfg.AppConfig.APIVersion).Middleware,
		requestValidator,
		middleware.Tracer(),
	}

	// Add rate limiting middleware
	if cfg.ThrottledRateLimiting.Enabled {
		rateLimitMiddleware := middleware.NewThrottledRateLimitingMiddleware(cfg.ThrottledRateLimiting, logger)

		middlewares = append(middlewares, rateLimitMiddleware.Middleware)
		logger.Info().Msg("Rate limiting enabled")
	}

	// Authentication is handled by the OpenAPI request validator
	if cfg.Auth.Enabled {
		logger.Info().Msg("Authentication enabled")
	}

	return middlewares
}

func initGlobalTracing(ctx context.Context, cfg *config.ServiceConfig) (func(context.Context) error, error) {
	if !cfg.Telemetry.Traces.Enabled {
		return func(_ context.Context) error {
			return nil
		}, nil
	}

	shutdownFunc, err := infrastructure.InitGlobalTracer(ctx, cfg.Telemetry, cfg.AppConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize global tracing: %w", err)
	}

	return shutdownFunc, nil
}

func createVaultClient(config config.SecretStorageConfig) (*api.Client, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = config.Address
	vaultConfig.Timeout = config.Timeout

	if config.TLSSkipVerify {
		tlsConfig := &api.TLSConfig{
			Insecure: true,
		}
		if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Skip namespace configuration for dev mode vault
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	return client, nil
}
