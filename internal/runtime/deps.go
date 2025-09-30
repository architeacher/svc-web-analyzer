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
		HTTPServer          *http.Server
		SecretStorageClient *api.Client
		StorageClient       *infrastructure.Storage
		QueueClient         infrastructure.Queue
		CacheClient         *infrastructure.KeydbClient
		Metrics             infrastructure.Metrics
	}

	DomainServices struct {
		WebFetcher   ports.WebFetcher
		HTMLAnalyzer domain.HTMLAnalyzer
		LinkChecker  ports.LinkChecker
	}

	Repos struct {
		SecretStorageRepo ports.SecretsRepository
		AnalysisRepo      ports.AnalysisRepository
		OutboxRepo        ports.OutboxRepository
		CacheRepo         ports.CacheRepository
	}

	Dependencies struct {
		Apps    Applications
		Workers ApplicationWorkers

		cfg          *config.ServiceConfig
		configLoader *config.Loader

		logger infrastructure.Logger

		Infra          InfrastructureDeps
		DomainServices DomainServices
		Repos          Repos

		tracerShutdownFunc TracerShutdownFunc
		secretVersion      uint
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
	options := append(defaultOptions(ctx), opts...)

	for _, opt := range options {
		if err := opt(deps); err != nil {
			return nil, fmt.Errorf("failed to apply dependency option: %w", err)
		}
	}

	deps.logger.Info().Msg("dependencies initialized successfully")

	return deps, nil
}

func initHTTPServer(
	cfg *config.ServiceConfig,
	logger infrastructure.Logger,
	metrics infrastructure.Metrics,
	reqHandler ports.RequestHandler,
	keyService *infrastructure.PasetoKeyService,
) *http.Server {
	logger.Info().Msg("creating HTTP server...")

	router := chi.NewRouter()

	middlewares := initMiddlewares(cfg, logger, metrics, keyService)

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

func initMiddlewares(
	cfg *config.ServiceConfig,
	logger infrastructure.Logger,
	metrics infrastructure.Metrics,
	keyService *infrastructure.PasetoKeyService,
) []handlers.MiddlewareFunc {
	swagger, err := handlers.GetSwagger()
	if err != nil {
		logger.Fatal().Err(err).Msg("error loading swagger spec")
	}

	swagger.Servers = nil

	requestValidator := middleware.OapiRequestValidatorWithOptions(logger, swagger, &middleware.RequestValidatorOptions{
		Options: openapi3filter.Options{
			MultiError:         false,
			AuthenticationFunc: middleware.NewPasetoAuthenticationFunc(cfg.Auth, logger, keyService),
		},
		ErrorHandler:          middleware.RequestValidationErrHandler,
		SilenceServersWarning: true,
	})

	middlewares := []handlers.MiddlewareFunc{
		chimiddleware.RequestID,
		chimiddleware.RealIP,
		chimiddleware.Recoverer,
		chimiddleware.Timeout(cfg.HTTPServer.WriteTimeout),
		middleware.NewAPIVersionMiddleware(cfg.AppConfig.APIVersion).Middleware,
		requestValidator,
		middleware.NewSecurityHeadersMiddleware().Middleware,
		middleware.Tracer(),
	}

	if cfg.Telemetry.Metrics.Enabled {
		metricsMiddleware := middleware.NewMetricsMiddleware(metrics)
		middlewares = append(middlewares, metricsMiddleware.Middleware)
		logger.Info().Msg("HTTP metrics collection enabled")
	}

	if cfg.Logging.AccessLog.Enabled {
		healthFilter := middleware.NewHealthCheckFilter(cfg.Logging.AccessLog.LogHealthChecks)
		accessLogger := middleware.NewAccessLogger(logger.Logger)

		middlewares = append(middlewares, healthFilter.Middleware, accessLogger.Middleware)
		logger.Info().
			Bool("log_health_checks", cfg.Logging.AccessLog.LogHealthChecks).
			Msg("structured access logging enabled")
	}

	if cfg.ThrottledRateLimiting.Enabled {
		rateLimitMiddleware := middleware.NewThrottledRateLimitingMiddleware(cfg.ThrottledRateLimiting, logger)

		middlewares = append(middlewares, rateLimitMiddleware.Middleware)
		logger.Info().Msg("rate limiting enabled")
	}

	if cfg.Auth.Enabled {
		logger.Info().Msg("authentication is enabled")
	}

	return middlewares
}
