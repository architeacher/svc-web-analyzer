package resilience

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
)

type (
	CacheClient interface {
		Get(ctx context.Context, key string) ([]byte, error)
	}

	CacheFallbackStrategy struct {
		url    string
		cache  CacheClient
		logger appLogger.Logger
	}

	WebFetcherStaticFallbackStrategy struct {
		url string
	}
)

func NewCacheFallbackStrategy(url string, cache CacheClient, logger appLogger.Logger) *CacheFallbackStrategy {
	return &CacheFallbackStrategy{
		url:    url,
		cache:  cache,
		logger: logger,
	}
}

func (cfs *CacheFallbackStrategy) Execute(ctx context.Context) (any, error) {
	cacheKey := fmt.Sprintf("webpage:%s", cfs.url)

	cachedData, err := cfs.cache.Get(ctx, cacheKey)
	if err != nil {
		cfs.logger.Error().
			Err(err).
			Str("url", cfs.url).
			Msg("failed to retrieve cached content")

		return nil, fmt.Errorf("failed to retrieve cached content: %w", err)
	}

	if len(cachedData) == 0 {
		cfs.logger.Warn().
			Str("url", cfs.url).
			Msg("no cached content available for fallback")

		return nil, fmt.Errorf("no cached content available for URL: %s", cfs.url)
	}

	var content domain.WebPageContent
	if err := json.Unmarshal(cachedData, &content); err != nil {
		cfs.logger.Error().
			Err(err).
			Str("url", cfs.url).
			Msg("failed to unmarshal cached content")

		return nil, fmt.Errorf("failed to unmarshal cached content: %w", err)
	}

	content.FromFallback = true
	content.FallbackReason = "cache_fallback"

	cfs.logger.Info().
		Str("url", cfs.url).
		Msg("successfully retrieved content from cache fallback")

	return &content, nil
}

func NewWebFetcherStaticFallbackStrategy(url string) *WebFetcherStaticFallbackStrategy {
	return &WebFetcherStaticFallbackStrategy{
		url: url,
	}
}

func (sfs *WebFetcherStaticFallbackStrategy) Execute(_ context.Context) (any, error) {
	return &domain.WebPageContent{
		URL:            sfs.url,
		StatusCode:     503,
		HTML:           fmt.Sprintf("<html><head><title>Service Temporarily Unavailable</title></head><body><h1>Service Temporarily Unavailable</h1><p>The requested page at %s is temporarily unavailable. Please try again later.</p></body></html>", sfs.url),
		ContentType:    "text/html",
		Headers:        make(map[string]string),
		FromFallback:   true,
		FallbackReason: "static_fallback",
	}, nil
}
