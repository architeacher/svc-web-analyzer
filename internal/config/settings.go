package config

import (
	"time"
)

// Compile time variables are set by -ldflags.
var (
	ServiceVersion string
	CommitSHA      string
	APIVersion     string
)

const (
	Development = 1 << iota
	Sandbox
	Staging
	Production
)

type (
	ServiceConfig struct {
		AppConfig             AppConfig
		Logging               LoggingConfig
		Telemetry             Telemetry
		SecretStorage         SecretStorageConfig
		HTTPServer            HTTPServerConfig
		SSE                   SSEConfig
		Cache                 CacheConfig
		Storage               StorageConfig
		Queue                 QueueConfig
		ThrottledRateLimiting ThrottledRateLimitingConfig
		Auth                  AuthConfig
		WebFetcher            WebFetcherConfig
		LinkChecker           LinkCheckerConfig
	}

	AppConfig struct {
		ServiceName    string `envconfig:"APP_SERVICE_NAME" default:"svc-web-analyzer"`
		ServiceVersion string `envconfig:"APP_SERVICE_VERSION" default:"0.0.0"`
		CommitSHA      string `envconfig:"APP_COMMIT_SHA" default:"unknown"`
		APIVersion     string `envconfig:"APP_API_VERSION" default:"v1"`
		Env            string `envconfig:"APP_ENVIRONMENT" default:"unknown"`
	}

	LoggingConfig struct {
		Level  string `envconfig:"LOGGING_LEVEL" default:"info"`
		Format string `envconfig:"LOGGING_FORMAT" default:"json"`
	}

	Telemetry struct {
		ExporterType string `envconfig:"OTEL_EXPORTER" default:"grpc"`

		OtelGRPCHost       string `envconfig:"OTEL_HOST"`
		OtelGRPCPort       string `envconfig:"OTEL_PORT" default:"4317"`
		OtelProductCluster string `envconfig:"OTEL_PRODUCT_CLUSTER"`

		Metrics Metrics
		Traces  Traces
	}

	Metrics struct {
		Enabled bool `envconfig:"METRICS_ENABLED" default:"false"`
	}

	Traces struct {
		Enabled      bool    `envconfig:"TRACES_ENABLED" default:"false"`
		SamplerRatio float64 `envconfig:"TRACES_SAMPLER_RATIO" default:"1"`
	}

	SecretStorageConfig struct {
		Enabled       bool          `envconfig:"VAULT_ENABLED" default:"true"`
		Address       string        `envconfig:"VAULT_ADDRESS" default:"http://vault:8200"`
		Token         string        `envconfig:"VAULT_TOKEN" default:"bottom-Secret"`
		RoleID        string        `envconfig:"VAULT_ROLE_ID" default:""`
		SecretID      string        `envconfig:"VAULT_SECRET_ID" default:""`
		AuthMethod    string        `envconfig:"VAULT_AUTH_METHOD" default:"token"`
		MountPath     string        `envconfig:"VAULT_MOUNT_PATH" default:"svc-web-analyzer"`
		Namespace     string        `envconfig:"VAULT_NAMESPACE" default:""`
		Timeout       time.Duration `envconfig:"VAULT_TIMEOUT" default:"30s"`
		MaxRetries    int           `envconfig:"VAULT_MAX_RETRIES" default:"3"`
		TLSSkipVerify bool          `envconfig:"VAULT_TLS_SKIP_VERIFY" default:"false"`
	}

	HTTPServerConfig struct {
		Port            int           `envconfig:"HTTP_SERVER_PORT" default:"8088"`
		Host            string        `envconfig:"HTTP_SERVER_HOST" default:"0.0.0.0"`
		ReadTimeout     time.Duration `envconfig:"HTTP_SERVER_READ_TIMEOUT" default:"30s"`
		WriteTimeout    time.Duration `envconfig:"HTTP_SERVER_WRITE_TIMEOUT" default:"30s"`
		IdleTimeout     time.Duration `envconfig:"HTTP_SERVER_IDLE_TIMEOUT" default:"120s"`
		ShutdownTimeout time.Duration `envconfig:"HTTP_SERVER_SHUTDOWN_TIMEOUT" default:"10s"`
	}

	SSEConfig struct {
		EventsInterval time.Duration `envconfig:"SSE_EVENTS_INTERVAL" default:"500ms"`
	}

	StorageConfig struct {
		Host            string        `envconfig:"POSTGRES_HOST" default:"postgres"`
		Port            int           `envconfig:"POSTGRES_PORT" default:"5432"`
		Database        string        `envconfig:"POSTGRES_DATABASE" default:"web_analyzer"`
		Username        string        `envconfig:"POSTGRES_USERNAME" default:"postgres"`
		Password        string        `envconfig:"POSTGRES_PASSWORD" default:""`
		SSLMode         string        `envconfig:"POSTGRES_SSL_MODE" default:"disable"`
		MaxOpenConns    int           `envconfig:"POSTGRES_MAX_OPEN_CONNS" default:"25"`
		MaxIdleConns    int           `envconfig:"POSTGRES_MAX_IDLE_CONNS" default:"5"`
		ConnMaxLifetime time.Duration `envconfig:"POSTGRES_CONN_MAX_LIFETIME" default:"5m"`
		ConnMaxIdleTime time.Duration `envconfig:"POSTGRES_CONN_MAX_IDLE_TIME" default:"5m"`
		ConnectTimeout  time.Duration `envconfig:"POSTGRES_CONNECT_TIMEOUT" default:"10s"`
		QueryTimeout    time.Duration `envconfig:"POSTGRES_QUERY_TIMEOUT" default:"30s"`
	}

	QueueConfig struct {
		Host           string        `envconfig:"RABBITMQ_HOST" default:"rabbitmq"`
		Port           int           `envconfig:"RABBITMQ_PORT" default:"5672"`
		Username       string        `envconfig:"RABBITMQ_USERNAME" default:"admin"`
		Password       string        `envconfig:"RABBITMQ_PASSWORD" default:"bottom.Secret"`
		VirtualHost    string        `envconfig:"RABBITMQ_VIRTUAL_HOST" default:"/"`
		ExchangeName   string        `envconfig:"RABBITMQ_EXCHANGE_NAME" default:"web-analyzer"`
		RoutingKey     string        `envconfig:"RABBITMQ_ROUTING_KEY" default:"analysis.*"`
		QueueName      string        `envconfig:"RABBITMQ_NAME" default:"analysis_queue"`
		ConnectTimeout time.Duration `envconfig:"RABBITMQ_CONNECT_TIMEOUT" default:"10s"`
		Heartbeat      time.Duration `envconfig:"RABBITMQ_HEARTBEAT" default:"10s"`
		PrefetchCount  int           `envconfig:"RABBITMQ_PREFETCH_COUNT" default:"10"`
		Durable        bool          `envconfig:"RABBITMQ_DURABLE" default:"true"`
		AutoDelete     bool          `envconfig:"RABBITMQ_AUTO_DELETE" default:"false"`
	}
	CacheConfig struct {
		Addr          string        `envconfig:"KEYDB_ADDR" default:"keydb:6379"`
		Password      string        `envconfig:"KEYDB_PASSWORD" default:"bottom.Secret"`
		DB            int           `envconfig:"KEYDB_DB" default:"0"`
		PoolSize      int           `envconfig:"KEYDB_POOL_SIZE" default:"10"`
		MinIdleConns  int           `envconfig:"KEYDB_MIN_IDLE_CONNS" default:"3"`
		DialTimeout   time.Duration `envconfig:"KEYDB_DIAL_TIMEOUT" default:"5s"`
		ReadTimeout   time.Duration `envconfig:"KEYDB_READ_TIMEOUT" default:"3s"`
		WriteTimeout  time.Duration `envconfig:"KEYDB_WRITE_TIMEOUT" default:"3s"`
		PoolTimeout   time.Duration `envconfig:"KEYDB_POOL_TIMEOUT" default:"5s"`
		MaxRetries    int           `envconfig:"KEYDB_MAX_RETRIES" default:"3"`
		DefaultExpiry time.Duration `envconfig:"KEYDB_DEFAULT_EXPIRY" default:"24h"`
	}

	ThrottledRateLimitingConfig struct {
		Enabled            bool          `envconfig:"RATE_LIMITING_ENABLED" default:"true"`
		RequestsPerSecond  int           `envconfig:"RATE_LIMITING_REQUESTS_PER_SECOND" default:"10"`
		BurstSize          int           `envconfig:"RATE_LIMITING_BURST_SIZE" default:"20"`
		WindowDuration     time.Duration `envconfig:"RATE_LIMITING_WINDOW_DURATION" default:"5m"`
		EnableIPLimiting   bool          `envconfig:"RATE_LIMITING_ENABLE_IP_LIMITING" default:"true"`
		EnableUserLimiting bool          `envconfig:"RATE_LIMITING_ENABLE_USER_LIMITING" default:"true"`
		CleanupInterval    time.Duration `envconfig:"RATE_LIMITING_CLEANUP_INTERVAL" default:"1m"`
		MaxKeys            int           `envconfig:"RATE_LIMITING_MAX_KEYS" default:"1000"`
		SkipPaths          []string      `envconfig:"RATE_LIMITING_SKIP_PATHS" default:"/health"`
	}

	AuthConfig struct {
		Enabled      bool          `envconfig:"AUTH_ENABLED" default:"true"`
		SecretKey    string        `envconfig:"AUTH_SECRET_KEY" default:"default-secret-key-change-in-production"`
		ValidIssuers []string      `envconfig:"AUTH_VALID_ISSUERS" default:"web-analyzer-service,auth-service"`
		TokenExpiry  time.Duration `envconfig:"AUTH_TOKEN_EXPIRY" default:"1h"`
		SkipPaths    []string      `envconfig:"AUTH_SKIP_PATHS" default:"/v1/health"`
	}

	CircuitBreakerConfig struct {
		MaxRequests uint32        `envconfig:"MAX_REQUESTS" default:"3"`
		Interval    time.Duration `envconfig:"INTERVAL" default:"10s"`
		Timeout     time.Duration `envconfig:"TIMEOUT" default:"60s"`
	}

	WebFetcherConfig struct {
		MaxRetries           int                  `envconfig:"WEB_FETCHER_MAX_RETRIES" default:"3"`
		RetryWaitTime        time.Duration        `envconfig:"WEB_FETCHER_RETRY_WAIT_TIME" default:"1s"`
		MaxRetryWaitTime     time.Duration        `envconfig:"WEB_FETCHER_MAX_RETRY_WAIT_TIME" default:"5s"`
		MaxRedirects         int                  `envconfig:"WEB_FETCHER_MAX_REDIRECTS" default:"10"`
		MaxResponseSizeBytes int64                `envconfig:"WEB_FETCHER_MAX_RESPONSE_SIZE_BYTES" default:"10485760"` // 10MB
		UserAgent            string               `envconfig:"WEB_FETCHER_USER_AGENT" default:"WebPageAnalyzer/1.0"`
		CircuitBreaker       CircuitBreakerConfig `envconfig:"WEB_FETCHER_CIRCUIT_BREAKER"`
	}

	LinkCheckerConfig struct {
		Timeout             time.Duration        `envconfig:"LINK_CHECKER_TIMEOUT" default:"10s"`
		MaxConcurrentChecks int                  `envconfig:"LINK_CHECKER_MAX_CONCURRENT_CHECKS" default:"10"`
		MaxLinksToCheck     int                  `envconfig:"LINK_CHECKER_MAX_LINKS_TO_CHECK" default:"100"`
		Retries             int                  `envconfig:"LINK_CHECKER_RETRIES" default:"2"`
		RetryWaitTime       time.Duration        `envconfig:"LINK_CHECKER_RETRY_WAIT_TIME" default:"500ms"`
		MaxRetryWaitTime    time.Duration        `envconfig:"LINK_CHECKER_MAX_RETRY_WAIT_TIME" default:"2s"`
		CircuitBreaker      CircuitBreakerConfig `envconfig:"LINK_CHECKER_CIRCUIT_BREAKER"`
	}
)
