package adapters

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/go-resty/resty/v2"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel/attribute"
)

const linkTypeKey = "link.type"

type (
	LinkChecker struct {
		client         *resty.Client
		circuitBreaker *gobreaker.CircuitBreaker
		config         config.LinkCheckerConfig
		logger         appLogger.Logger
		metricsClient  ports.MetricsClient
	}

	linkCheckResult struct {
		StatusCode int
		Error      string
	}
)

func NewLinkChecker(
	config config.LinkCheckerConfig,
	logger appLogger.Logger,
	metricsClient ports.MetricsClient,
) *LinkChecker {
	client := resty.New()

	client.SetTimeout(config.Timeout).
		SetRetryCount(int(config.Retries)).
		SetRetryWaitTime(config.RetryWaitTime).
		SetRetryMaxWaitTime(config.MaxRetryWaitTime).
		SetRedirectPolicy(resty.FlexibleRedirectPolicy(5)) // Limit redirects for link checking

	client.SetHeaders(map[string]string{
		"User-Agent": "WebAnalyzer-WebCrawler/1.0",
		"Accept":     "*/*",
	})

	cbSettings := gobreaker.Settings{
		Name:        "link-checker",
		MaxRequests: uint32(config.CircuitBreaker.MaxRequests),
		Interval:    config.CircuitBreaker.Interval,
		Timeout:     config.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.8
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info().
				Str("name", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("Link checker circuit breaker state changed")
		},
	}

	circuitBreaker := gobreaker.NewCircuitBreaker(cbSettings)

	return &LinkChecker{
		client:         client,
		circuitBreaker: circuitBreaker,
		logger:         logger,
		config:         config,
		metricsClient:  metricsClient,
	}
}

func (lc *LinkChecker) CheckAccessibility(ctx context.Context, links []domain.Link) []domain.InaccessibleLink {
	if len(links) == 0 {
		return []domain.InaccessibleLink{}
	}

	// Filter to only external links and limit the number
	externalLinks := lc.filterExternalLinks(links)
	if len(externalLinks) > int(lc.config.MaxLinksToCheck) {
		lc.logger.Warn().
			Int("total_links", len(externalLinks)).
			Uint("max_links", lc.config.MaxLinksToCheck).
			Msg("Too many links to check, limiting to maximum allowed")
		externalLinks = externalLinks[:lc.config.MaxLinksToCheck]
	}

	lc.logger.Info().
		Int("total_links", len(links)).
		Int("external_links", len(externalLinks)).
		Int("links_to_check", len(externalLinks)).
		Msg("Starting link accessibility check")

	inaccessibleLinks := lc.checkLinks(ctx, externalLinks)

	lc.logger.Info().
		Int("total_checked", len(externalLinks)).
		Int("inaccessible", len(inaccessibleLinks)).
		Msg("Link accessibility check completed")

	return inaccessibleLinks
}

func (lc *LinkChecker) filterExternalLinks(links []domain.Link) []domain.Link {
	var externalLinks []domain.Link
	seen := make(map[string]bool)

	for _, link := range links {
		if link.Type == domain.LinkTypeInternal || seen[link.URL] {
			continue
		}

		seen[link.URL] = true

		if _, err := url.Parse(link.URL); err != nil {
			continue
		}

		externalLinks = append(externalLinks, link)
	}

	return externalLinks
}

func (lc *LinkChecker) checkLinks(ctx context.Context, links []domain.Link) []domain.InaccessibleLink {
	var inaccessibleLinks []domain.InaccessibleLink
	var mu sync.Mutex

	semaphore := make(chan struct{}, lc.config.MaxConcurrentChecks)
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Go(func() {
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if inaccessibleLink := lc.checkSingleLink(ctx, link); inaccessibleLink != nil {
				mu.Lock()
				inaccessibleLinks = append(inaccessibleLinks, *inaccessibleLink)
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return inaccessibleLinks
}

func (lc *LinkChecker) checkSingleLink(ctx context.Context, link domain.Link) *domain.InaccessibleLink {
	startTime := time.Now()

	result, err := lc.circuitBreaker.Execute(func() (any, error) {
		return lc.performLinkCheck(ctx, link.URL)
	})

	duration := time.Since(startTime)

	if err != nil {
		lc.logger.Debug().
			Err(err).
			Str("url", link.URL).
			Int64("duration_ms", duration.Milliseconds()).
			Msg("Link check failed")

		lc.recordLinkCheck(ctx, false, string(link.Type))

		if errors.Is(err, gobreaker.ErrOpenState) {
			return &domain.InaccessibleLink{
				URL:        link.URL,
				StatusCode: http.StatusServiceUnavailable,
				Error:      "Service temporarily unavailable (circuit breaker open)",
			}
		}

		return &domain.InaccessibleLink{
			URL:        link.URL,
			StatusCode: 0,
			Error:      err.Error(),
		}
	}

	checkResult := result.(*linkCheckResult)

	lc.logger.Debug().
		Str("url", link.URL).
		Int("status_code", checkResult.StatusCode).
		Int64("duration_ms", duration.Milliseconds()).
		Msg("link check completed")

	if checkResult.StatusCode >= http.StatusBadRequest {
		lc.recordLinkCheck(ctx, false, string(link.Type))

		return &domain.InaccessibleLink{
			URL:        link.URL,
			StatusCode: uint(checkResult.StatusCode),
			Error:      checkResult.Error,
		}
	}

	lc.recordLinkCheck(ctx, true, string(link.Type))

	return nil
}

func (lc *LinkChecker) performLinkCheck(ctx context.Context, linkURL string) (*linkCheckResult, error) {
	// Use HEAD request first for efficiency
	resp, err := lc.client.R().
		SetContext(ctx).
		Head(linkURL)

	if err != nil {
		// If HEAD fails, try the GET request
		resp, err = lc.client.R().
			SetContext(ctx).
			Get(linkURL)
		if err != nil {
			return nil, err
		}
	}

	result := &linkCheckResult{
		StatusCode: resp.StatusCode(),
	}

	if resp.StatusCode() >= http.StatusBadRequest {
		result.Error = resp.Status()
	}

	return result, nil
}

func (lc *LinkChecker) recordLinkCheck(ctx context.Context, success bool, linkType string) {
	attr := attribute.String(linkTypeKey, linkType)
	lc.metricsClient.Inc(ctx, ports.LinkCheckTotal, int64(1), attr)

	if !success {
		lc.metricsClient.Inc(ctx, ports.LinkCheckErrorTotal, int64(1), attr)
	}
}
