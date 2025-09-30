package middleware

import (
	"net/http"
	"strings"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
)

type ThrottledRateLimitingMiddleware struct {
	config      config.ThrottledRateLimitingConfig
	httpLimiter *throttled.HTTPRateLimiterCtx
	logger      appLogger.Logger
}

func NewThrottledRateLimitingMiddleware(config config.ThrottledRateLimitingConfig, logger appLogger.Logger) *ThrottledRateLimitingMiddleware {
	store, err := memstore.NewCtx(int(config.MaxKeys))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create memory store for rate limiter")
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(int(config.RequestsPerSecond)),
		MaxBurst: int(config.BurstSize),
	}

	rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create rate limiter")
	}

	httpLimiter := &throttled.HTTPRateLimiterCtx{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{RemoteAddr: config.EnableIPLimiting},
	}

	return &ThrottledRateLimitingMiddleware{
		config:      config,
		httpLimiter: httpLimiter,
		logger:      logger,
	}
}

func (m *ThrottledRateLimitingMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for certain paths
		if m.shouldSkipRateLimit(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Use the HTTPRateLimiter middleware
		m.httpLimiter.RateLimit(next).ServeHTTP(w, r)
	})
}

func (m *ThrottledRateLimitingMiddleware) shouldSkipRateLimit(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}
