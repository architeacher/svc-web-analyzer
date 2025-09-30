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
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/vault/api"
)

type (
	application struct {
		web        *usecases.WebApplication
		publisher  *usecases.PublisherApplication
		subscriber *usecases.SubscriberApplication
	}

	applicationWorker struct {
		outboxProcessor ports.BackgroundProcessor
		analysisWorker  ports.MessageHandler
	}

	tracerShutdownFunc func(ctx context.Context) error

	infrastructureDep struct {
		httpServer          *http.Server
		secretStorageClient *api.Client
		storageClient       *infrastructure.Storage
		queueClient         infrastructure.Queue
		cacheClient         *infrastructure.KeydbClient
		tracer              ports.Tracer
		tracerShutdownFunc  tracerShutdownFunc
		metricsClient       ports.MetricsClient
		logger              appLogger.Logger
		profiler            *infrastructure.Profiler
	}

	domainService struct {
		webFetcher   ports.WebFetcher
		htmlAnalyzer domain.HTMLAnalyzer
		linkChecker  ports.LinkChecker
	}

	repos struct {
		secretStorageRepo ports.SecretsRepository
		cacheRepo         ports.CacheRepository

		analysisRepo ports.AnalysisRepository
		outboxRepo   ports.OutboxRepository
	}

	dependencies struct {
		apps    application
		workers applicationWorker

		cfg          *config.ServiceConfig
		configLoader *config.Loader

		infra          infrastructureDep
		domainServices domainService
		repos          repos

		secretVersion uint
	}
)

func initializeDependencies(ctx context.Context, opts ...DependencyOption) (*dependencies, error) {
	cfg, err := config.Init()
	if err != nil {
		return nil, fmt.Errorf("unable to load service configuration: %w", err)
	}

	deps := &dependencies{
		cfg: cfg,
	}

	// Start with default options and append any additional options.
	options := append(defaultOptions(ctx), opts...)

	for _, opt := range options {
		if err := opt(deps); err != nil {
			return nil, fmt.Errorf("failed to apply dependency option: %w", err)
		}
	}

	deps.infra.logger.Info().Msg("dependencies initialized successfully")

	return deps, nil
}

func initHTTPServer(
	cfg *config.ServiceConfig,
	logger appLogger.Logger,
	metricsClient ports.MetricsClient,
	reqHandler ports.RequestHandler,
	keyService *infrastructure.PasetoKeyService,
) *http.Server {
	logger.Info().Msg("creating HTTP server...")

	router := chi.NewRouter()

	middlewares := initMiddlewares(cfg, logger, metricsClient, keyService)

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
	logger appLogger.Logger,
	metricsClient ports.MetricsClient,
	keyService *infrastructure.PasetoKeyService,
) []handlers.MiddlewareFunc {
	swagger, err := handlers.GetSwagger()
	if err != nil {
		logger.Fatal().Err(err).Msg("error loading swagger spec")
	}

	swagger.Servers = nil

	requestValidator := middleware.OapiRequestValidatorWithOptions(
		logger,
		swagger,
		&middleware.RequestValidatorOptions{
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
		metricsMiddleware := middleware.NewMetricsMiddleware(metricsClient)
		middlewares = append(middlewares, metricsMiddleware.Middleware)
		logger.Info().Msg("HTTP metricsClient collection enabled")
	}

	if cfg.Logging.AccessLog.Enabled {
		healthFilter := middleware.NewHealthCheckFilter(cfg.Logging.AccessLog.LogHealthChecks)
		accessLogger := middleware.NewAccessLogger(logger)

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
