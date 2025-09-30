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

const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

type (
	ServiceConfig struct {
		AppConfig             AppConfig                   `json:"app_config"`
		Logging               LoggingConfig               `json:"logging"`
		Telemetry             Telemetry                   `json:"telemetry"`
		SecretStorage         SecretStorageConfig         `json:"secret_storage"`
		HTTPServer            HTTPServerConfig            `json:"http_server"`
		SSE                   SSEConfig                   `json:"sse"`
		Cache                 CacheConfig                 `json:"cache"`
		Storage               StorageConfig               `json:"storage"`
		Queue                 QueueConfig                 `json:"queue"`
		Outbox                OutboxConfig                `json:"outbox"`
		ThrottledRateLimiting ThrottledRateLimitingConfig `json:"throttled_rate_limiting"`
		Backoff               BackoffConfig               `json:"backoff"`
		Auth                  AuthConfig                  `json:"auth"`
		WebFetcher            WebFetcherConfig            `json:"web_fetcher"`
		LinkChecker           LinkCheckerConfig           `json:"link_checker"`
	}

	AppConfig struct {
		ServiceName    string `envconfig:"APP_SERVICE_NAME" default:"svc-web-analyzer" json:"service_name"`
		ServiceVersion string `envconfig:"APP_SERVICE_VERSION" default:"0.0.0" json:"service_version"`
		CommitSHA      string `envconfig:"APP_COMMIT_SHA" default:"unknown" json:"commit_sha"`
		APIVersion     string `envconfig:"APP_API_VERSION" default:"v1" json:"api_version"`
		Env            string `envconfig:"APP_ENVIRONMENT" default:"unknown" json:"env"`
	}

	LoggingConfig struct {
		Level     string          `envconfig:"LOGGING_LEVEL" default:"info" json:"level"`
		Format    string          `envconfig:"LOGGING_FORMAT" default:"json" json:"format"`
		AccessLog AccessLogConfig `json:"access_log"`
	}

	AccessLogConfig struct {
		Enabled            bool `envconfig:"ACCESS_LOG_ENABLED" default:"true" json:"enabled"`
		LogHealthChecks    bool `envconfig:"ACCESS_LOG_HEALTH_CHECKS" default:"false" json:"log_health_checks"`
		IncludeQueryParams bool `envconfig:"ACCESS_LOG_INCLUDE_QUERY_PARAMS" default:"true" json:"include_query_params"`
	}

	Telemetry struct {
		ExporterType string `envconfig:"OTEL_EXPORTER" default:"grpc" json:"exporter_type"`

		OtelGRPCHost       string `envconfig:"OTEL_HOST" json:"otel_grpc_host"`
		OtelGRPCPort       string `envconfig:"OTEL_PORT" default:"4317" json:"otel_grpc_port"`
		OtelProductCluster string `envconfig:"OTEL_PRODUCT_CLUSTER" json:"otel_product_cluster"`

		Metrics Metrics `json:"metrics"`
		Traces  Traces  `json:"traces"`
	}

	Metrics struct {
		Enabled bool `envconfig:"METRICS_ENABLED" default:"false" json:"enabled"`
	}

	Traces struct {
		Enabled      bool    `envconfig:"TRACES_ENABLED" default:"false" json:"enabled"`
		SamplerRatio float64 `envconfig:"TRACES_SAMPLER_RATIO" default:"1" json:"sampler_ratio"`
	}

	SecretStorageConfig struct {
		Enabled       bool          `envconfig:"VAULT_ENABLED" default:"true" json:"enabled"`
		Address       string        `envconfig:"VAULT_ADDRESS" default:"http://vault:8200" json:"address"`
		Token         string        `envconfig:"VAULT_TOKEN" default:"bottom-Secret" json:"token,omitempty"`
		RoleID        string        `envconfig:"VAULT_ROLE_ID" default:"" json:"role_id,omitempty"`
		SecretID      string        `envconfig:"VAULT_SECRET_ID" default:"" json:"secret_id,omitempty"`
		AuthMethod    string        `envconfig:"VAULT_AUTH_METHOD" default:"token" json:"auth_method"`
		MountPath     string        `envconfig:"VAULT_MOUNT_PATH" default:"svc-web-analyzer" json:"mount_path"`
		Namespace     string        `envconfig:"VAULT_NAMESPACE" default:"" json:"namespace,omitempty"`
		Timeout       time.Duration `envconfig:"VAULT_TIMEOUT" default:"30s" json:"timeout"`
		MaxRetries    int           `envconfig:"VAULT_MAX_RETRIES" default:"3" json:"max_retries"`
		TLSSkipVerify bool          `envconfig:"VAULT_TLS_SKIP_VERIFY" default:"false" json:"tls_skip_verify"`
		PollInterval  time.Duration `envconfig:"VAULT_POLL_INTERVAL" default:"24h" json:"poll_interval"`
	}

	HTTPServerConfig struct {
		Port            int           `envconfig:"HTTP_SERVER_PORT" default:"8088" json:"port"`
		Host            string        `envconfig:"HTTP_SERVER_HOST" default:"0.0.0.0" json:"host"`
		ReadTimeout     time.Duration `envconfig:"HTTP_SERVER_READ_TIMEOUT" default:"30s" json:"read_timeout"`
		WriteTimeout    time.Duration `envconfig:"HTTP_SERVER_WRITE_TIMEOUT" default:"30s" json:"write_timeout"`
		IdleTimeout     time.Duration `envconfig:"HTTP_SERVER_IDLE_TIMEOUT" default:"120s" json:"idle_timeout"`
		ShutdownTimeout time.Duration `envconfig:"HTTP_SERVER_SHUTDOWN_TIMEOUT" default:"30s" json:"shutdown_timeout"`
	}

	SSEConfig struct {
		HeartbeatInterval time.Duration `envconfig:"SSE_HEARTBEAT_INTERVAL" default:"5s" json:"heartbeat_interval"`
		EventsInterval    time.Duration `envconfig:"SSE_EVENTS_INTERVAL" default:"100ms" json:"events_interval"`
	}

	StorageConfig struct {
		Host            string        `envconfig:"POSTGRES_HOST" default:"postgres" json:"host"`
		Port            int           `envconfig:"POSTGRES_PORT" default:"5432" json:"port"`
		Database        string        `envconfig:"POSTGRES_DATABASE" default:"web_analyzer" json:"database"`
		Username        string        `envconfig:"POSTGRES_USERNAME" default:"postgres" json:"username"`
		Password        string        `envconfig:"POSTGRES_PASSWORD" default:"" json:"password,omitempty"`
		SSLMode         string        `envconfig:"POSTGRES_SSL_MODE" default:"disable" json:"ssl_mode"`
		MaxOpenConns    int           `envconfig:"POSTGRES_MAX_OPEN_CONNS" default:"25" json:"max_open_conns"`
		MaxIdleConns    int           `envconfig:"POSTGRES_MAX_IDLE_CONNS" default:"5" json:"max_idle_conns"`
		ConnMaxLifetime time.Duration `envconfig:"POSTGRES_CONN_MAX_LIFETIME" default:"5m" json:"conn_max_lifetime"`
		ConnMaxIdleTime time.Duration `envconfig:"POSTGRES_CONN_MAX_IDLE_TIME" default:"5m" json:"conn_max_idle_time"`
		ConnectTimeout  time.Duration `envconfig:"POSTGRES_CONNECT_TIMEOUT" default:"10s" json:"connect_timeout"`
		QueryTimeout    time.Duration `envconfig:"POSTGRES_QUERY_TIMEOUT" default:"30s" json:"query_timeout"`
	}

	QueueConfig struct {
		Host           string        `envconfig:"RABBITMQ_HOST" default:"rabbitmq" json:"host"`
		Port           int           `envconfig:"RABBITMQ_PORT" default:"5672" json:"port"`
		Username       string        `envconfig:"RABBITMQ_USERNAME" default:"admin" json:"username"`
		Password       string        `envconfig:"RABBITMQ_PASSWORD" default:"bottom.Secret" json:"password,omitempty"`
		VirtualHost    string        `envconfig:"RABBITMQ_VIRTUAL_HOST" default:"/" json:"virtual_host"`
		ExchangeName   string        `envconfig:"RABBITMQ_EXCHANGE_NAME" default:"web-analyzer" json:"exchange_name"`
		RoutingKey     string        `envconfig:"RABBITMQ_ROUTING_KEY" default:"analysis.*" json:"routing_key"`
		QueueName      string        `envconfig:"RABBITMQ_NAME" default:"analysis_queue" json:"queue_name"`
		ConnectTimeout time.Duration `envconfig:"RABBITMQ_CONNECT_TIMEOUT" default:"10s" json:"connect_timeout"`
		Heartbeat      time.Duration `envconfig:"RABBITMQ_HEARTBEAT" default:"10s" json:"heartbeat"`
		PrefetchCount  int           `envconfig:"RABBITMQ_PREFETCH_COUNT" default:"10" json:"prefetch_count"`
		Durable        bool          `envconfig:"RABBITMQ_DURABLE" default:"true" json:"durable"`
		AutoDelete     bool          `envconfig:"RABBITMQ_AUTO_DELETE" default:"false" json:"auto_delete"`
	}

	OutboxConfig struct {
		MaxRetries MaxRetriesByPriority `json:"max_retries"`
	}

	MaxRetriesByPriority struct {
		Low    int `envconfig:"OUTBOX_MAX_RETRIES_LOW" default:"3" json:"low"`
		Normal int `envconfig:"OUTBOX_MAX_RETRIES_NORMAL" default:"5" json:"normal"`
		High   int `envconfig:"OUTBOX_MAX_RETRIES_HIGH" default:"7" json:"high"`
		Urgent int `envconfig:"OUTBOX_MAX_RETRIES_URGENT" default:"10" json:"urgent"`
	}

	CacheConfig struct {
		Addr          string        `envconfig:"KEYDB_ADDR" default:"keydb:6379" json:"addr"`
		Password      string        `envconfig:"KEYDB_PASSWORD" default:"bottom.Secret" json:"password,omitempty"`
		DB            int           `envconfig:"KEYDB_DB" default:"0" json:"db"`
		PoolSize      int           `envconfig:"KEYDB_POOL_SIZE" default:"10" json:"pool_size"`
		MinIdleConns  int           `envconfig:"KEYDB_MIN_IDLE_CONNS" default:"3" json:"min_idle_conns"`
		DialTimeout   time.Duration `envconfig:"KEYDB_DIAL_TIMEOUT" default:"5s" json:"dial_timeout"`
		ReadTimeout   time.Duration `envconfig:"KEYDB_READ_TIMEOUT" default:"3s" json:"read_timeout"`
		WriteTimeout  time.Duration `envconfig:"KEYDB_WRITE_TIMEOUT" default:"3s" json:"write_timeout"`
		PoolTimeout   time.Duration `envconfig:"KEYDB_POOL_TIMEOUT" default:"5s" json:"pool_timeout"`
		MaxRetries    int           `envconfig:"KEYDB_MAX_RETRIES" default:"3" json:"max_retries"`
		DefaultExpiry time.Duration `envconfig:"KEYDB_DEFAULT_EXPIRY" default:"24h" json:"default_expiry"`
	}

	ThrottledRateLimitingConfig struct {
		Enabled            bool          `envconfig:"RATE_LIMITING_ENABLED" default:"true" json:"enabled"`
		RequestsPerSecond  int           `envconfig:"RATE_LIMITING_REQUESTS_PER_SECOND" default:"10" json:"requests_per_second"`
		BurstSize          int           `envconfig:"RATE_LIMITING_BURST_SIZE" default:"20" json:"burst_size"`
		WindowDuration     time.Duration `envconfig:"RATE_LIMITING_WINDOW_DURATION" default:"5m" json:"window_duration"`
		EnableIPLimiting   bool          `envconfig:"RATE_LIMITING_ENABLE_IP_LIMITING" default:"true" json:"enable_ip_limiting"`
		EnableUserLimiting bool          `envconfig:"RATE_LIMITING_ENABLE_USER_LIMITING" default:"true" json:"enable_user_limiting"`
		CleanupInterval    time.Duration `envconfig:"RATE_LIMITING_CLEANUP_INTERVAL" default:"1m" json:"cleanup_interval"`
		MaxKeys            int           `envconfig:"RATE_LIMITING_MAX_KEYS" default:"1000" json:"max_keys"`
		SkipPaths          []string      `envconfig:"RATE_LIMITING_SKIP_PATHS" default:"/health" json:"skip_paths"`
	}

	AuthConfig struct {
		Enabled        bool          `envconfig:"AUTH_ENABLED" default:"true" json:"enabled"`
		SecretKey      string        `envconfig:"AUTH_SECRET_KEY" default:"default-secret-key-change-in-production" json:"secret_key,omitempty"`
		ValidIssuers   []string      `envconfig:"AUTH_VALID_ISSUERS" default:"web-analyzer-service,auth-service" json:"valid_issuers"`
		TokenExpiry    time.Duration `envconfig:"AUTH_TOKEN_EXPIRY" default:"1h" json:"token_expiry"`
		SkipPaths      []string      `envconfig:"AUTH_SKIP_PATHS" default:"/v1/health" json:"skip_paths"`
		PasetoKeyPath  string        `envconfig:"AUTH_PASETO_KEY_PATH" default:"secret/data/paseto/public-key" json:"paseto_key_path"`
		UseVaultKeys   bool          `envconfig:"AUTH_USE_VAULT_KEYS" default:"true" json:"use_vault_keys"`
		KeyCacheTTL    time.Duration `envconfig:"AUTH_KEY_CACHE_TTL" default:"1h" json:"key_cache_ttl"`
		FallbackKeyHex string        `envconfig:"AUTH_FALLBACK_KEY_HEX" default:"01c7981f62c676934dc4acfa7825205ae927960875d09abec497efbe2dba41b7" json:"fallback_key_hex,omitempty"`
	}

	BackoffConfig struct {
		// BaseDelay is the amount of time to backoff after the first failure.
		BaseDelay time.Duration `environment:"BASE_DELAY" default:"1s" json:"base_delay"`
		// Multiplier is the factor with which to multiply backoffs after a
		// failed retry. Should ideally be greater than 1.
		Multiplier float64 `environment:"MULTIPLIER" default:"1.6" json:"multiplier"`
		// Jitter is the factor with which backoffs are randomized.
		Jitter float64 `environment:"JITTER" default:"0.2" json:"jitter"`
		// MaxDelay is the upper bound of backoff delay.
		MaxDelay time.Duration `environment:"MAX_DELAY" default:"10s" json:"max_delay"`
	}

	CircuitBreakerConfig struct {
		MaxRequests uint32        `envconfig:"MAX_REQUESTS" default:"3" json:"max_requests"`
		Interval    time.Duration `envconfig:"INTERVAL" default:"10s" json:"interval"`
		Timeout     time.Duration `envconfig:"TIMEOUT" default:"60s" json:"timeout"`
	}

	WebFetcherConfig struct {
		MaxRetries           int                  `envconfig:"WEB_FETCHER_MAX_RETRIES" default:"3" json:"max_retries"`
		RetryWaitTime        time.Duration        `envconfig:"WEB_FETCHER_RETRY_WAIT_TIME" default:"1s" json:"retry_wait_time"`
		MaxRetryWaitTime     time.Duration        `envconfig:"WEB_FETCHER_MAX_RETRY_WAIT_TIME" default:"5s" json:"max_retry_wait_time"`
		MaxRedirects         int                  `envconfig:"WEB_FETCHER_MAX_REDIRECTS" default:"10" json:"max_redirects"`
		MaxResponseSizeBytes int64                `envconfig:"WEB_FETCHER_MAX_RESPONSE_SIZE_BYTES" default:"10485760" json:"max_response_size_bytes"` // 10MB
		UserAgent            string               `envconfig:"WEB_FETCHER_USER_AGENT" default:"WebAnalyzer/1.0" json:"user_agent"`
		CircuitBreaker       CircuitBreakerConfig `envconfig:"WEB_FETCHER_CIRCUIT_BREAKER" json:"circuit_breaker"`
	}

	LinkCheckerConfig struct {
		Timeout             time.Duration        `envconfig:"LINK_CHECKER_TIMEOUT" default:"10s" json:"timeout"`
		MaxConcurrentChecks int                  `envconfig:"LINK_CHECKER_MAX_CONCURRENT_CHECKS" default:"10" json:"max_concurrent_checks"`
		MaxLinksToCheck     int                  `envconfig:"LINK_CHECKER_MAX_LINKS_TO_CHECK" default:"100" json:"max_links_to_check"`
		Retries             int                  `envconfig:"LINK_CHECKER_RETRIES" default:"2" json:"retries"`
		RetryWaitTime       time.Duration        `envconfig:"LINK_CHECKER_RETRY_WAIT_TIME" default:"500ms" json:"retry_wait_time"`
		MaxRetryWaitTime    time.Duration        `envconfig:"LINK_CHECKER_MAX_RETRY_WAIT_TIME" default:"2s" json:"max_retry_wait_time"`
		CircuitBreaker      CircuitBreakerConfig `envconfig:"LINK_CHECKER_CIRCUIT_BREAKER" json:"circuit_breaker"`
	}
)

func (c OutboxConfig) GetMaxRetriesForPriority(priority string) int {
	switch priority {
	case PriorityLow:
		return c.MaxRetries.Low
	case PriorityNormal:
		return c.MaxRetries.Normal
	case PriorityHigh:
		return c.MaxRetries.High
	case PriorityUrgent:
		return c.MaxRetries.Urgent
	default:
		return c.MaxRetries.Normal
	}
}
