package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/architeacher/svc-web-analyzer/internal/adapters/http/handlers"
	"github.com/architeacher/svc-web-analyzer/internal/adapters/middleware"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/vault/api"
)

type (
	Applications struct {
		Web        *usecases.WebApplication
		Publisher  *usecases.PublisherApplication
		Subscriber *usecases.SubscriberApplication
	}

	ApplicationWorkers struct {
		OutboxProcessor ports.BackgroundProcessor
		AnalysisWorker  ports.MessageHandler
	}

	TracerShutdownFunc func(ctx context.Context) error

	InfrastructureDeps struct {
		SecretStorageClient ports.SecretsRepository
		HTTPServer          *http.Server
		StorageClient       *infrastructure.Storage
		QueueClient         infrastructure.Queue
		CacheClient         *infrastructure.KeydbClient
	}

	DomainServices struct {
		WebFetcher   ports.WebFetcher
		HTMLAnalyzer domain.HTMLAnalyzer
		LinkChecker  ports.LinkChecker
	}

	Dependencies struct {
		Apps    Applications
		Workers ApplicationWorkers

		cfg    *config.ServiceConfig
		logger *infrastructure.Logger

		Infra          InfrastructureDeps
		DomainServices DomainServices

		tracerShutdownFunc TracerShutdownFunc
		secretVersion      int
	}
)

func initializeDependencies(ctx context.Context, opts ...DependencyOption) (*Dependencies, error) {
	cfg, err := config.Init()
	if err != nil {
		return nil, fmt.Errorf("unable to load service configuration: %w", err)
	}

	appLogger := infrastructure.New(config.LoggingConfig{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	appLogger.Info().Msg("initializing dependencies...")

	deps := &Dependencies{
		cfg:    cfg,
		logger: appLogger,
	}

	// Start with default options and append any additional options.
	options := defaultOptions(ctx)
	options = append(options, opts...)

	for _, opt := range options {
		if err := opt(deps); err != nil {
			return nil, fmt.Errorf("failed to apply dependency option: %w", err)
		}
	}

	deps.logger.Info().Msg("dependencies initialized successfully")

	return deps, nil
}

func initHTTPServer(cfg *config.ServiceConfig, logger *infrastructure.Logger, reqHandler ports.RequestHandler) *http.Server {
	logger.Info().Msg("creating HTTP server...")

	router := chi.NewRouter()

	middlewares := initMiddlewares(cfg, logger)

	// Add global CORS middleware to handle preflight requests
	router.Use(middleware.NewSecurityHeadersMiddleware().Middleware)

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
		middleware.NewAPIVersionMiddleware(cfg.AppConfig.APIVersion).Middleware,
		requestValidator,
		middleware.NewSecurityHeadersMiddleware().Middleware,
		middleware.Tracer(),
	}

	// Add rate limiting middleware
	if cfg.ThrottledRateLimiting.Enabled {
		rateLimitMiddleware := middleware.NewThrottledRateLimitingMiddleware(cfg.ThrottledRateLimiting, logger)

		middlewares = append(middlewares, rateLimitMiddleware.Middleware)
		logger.Info().Msg("rate limiting enabled")
	}

	// Authentication is handled by the OpenAPI request validator
	if cfg.Auth.Enabled {
		logger.Info().Msg("authentication is enabled")
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
