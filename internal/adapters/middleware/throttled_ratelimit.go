package middleware

import (
	"net/http"
	"strings"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
)

type ThrottledRateLimitMiddleware struct {
	config      config.ThrottledRateLimitingConfig
	httpLimiter *throttled.HTTPRateLimiterCtx
	logger      *infrastructure.Logger
}

func NewThrottledRateLimitingMiddleware(config config.ThrottledRateLimitingConfig, logger *infrastructure.Logger) *ThrottledRateLimitMiddleware {
	store, err := memstore.NewCtx(config.MaxKeys)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create memory store for rate limiter")
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(config.RequestsPerSecond),
		MaxBurst: config.BurstSize,
	}

	rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create rate limiter")
	}

	httpLimiter := &throttled.HTTPRateLimiterCtx{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{RemoteAddr: config.EnableIPLimiting},
	}

	return &ThrottledRateLimitMiddleware{
		config:      config,
		httpLimiter: httpLimiter,
		logger:      logger,
	}
}

func (m *ThrottledRateLimitMiddleware) Middleware(next http.Handler) http.Handler {
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

func (m *ThrottledRateLimitMiddleware) shouldSkipRateLimit(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}
