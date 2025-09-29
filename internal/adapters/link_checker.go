package adapters

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/go-resty/resty/v2"
	"github.com/sony/gobreaker"
)

const (
	defaultLinkCheckTimeout   = 10 * time.Second
	maxConcurrentLinkChecks   = 10
	maxLinksToCheck           = 100 // Prevent abuse
	linkCheckRetries          = 2
	linkCheckRetryWaitTime    = 500 * time.Millisecond
	linkCheckMaxRetryWaitTime = 2 * time.Second
)

type LinkChecker struct {
	client         *resty.Client
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *infrastructure.Logger
	config         config.LinkCheckerConfig
}

func NewLinkChecker(config config.LinkCheckerConfig, logger *infrastructure.Logger) *LinkChecker {
	client := resty.New()

	client.SetTimeout(config.Timeout)
	client.SetRetryCount(config.Retries)
	client.SetRetryWaitTime(config.RetryWaitTime)
	client.SetRetryMaxWaitTime(config.MaxRetryWaitTime)
	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(5)) // Limit redirects for link checking

	client.SetHeaders(map[string]string{
		"User-Agent": "WebPageAnalyzer-WebCrawler/1.0",
		"Accept":     "*/*",
	})

	cbSettings := gobreaker.Settings{
		Name:        "link-checker",
		MaxRequests: config.CircuitBreaker.MaxRequests,
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
	}
}

func (lc *LinkChecker) CheckAccessibility(ctx context.Context, links []domain.Link) []domain.InaccessibleLink {
	if len(links) == 0 {
		return []domain.InaccessibleLink{}
	}

	// Filter to only external links and limit the number
	externalLinks := lc.filterExternalLinks(links)
	if len(externalLinks) > lc.config.MaxLinksToCheck {
		lc.logger.Warn().
			Int("total_links", len(externalLinks)).
			Int("max_links", lc.config.MaxLinksToCheck).
			Msg("Too many links to check, limiting to maximum allowed")
		externalLinks = externalLinks[:lc.config.MaxLinksToCheck]
	}

	lc.logger.Info().
		Int("total_links", len(links)).
		Int("external_links", len(externalLinks)).
		Int("links_to_check", len(externalLinks)).
		Msg("Starting link accessibility check")

	inaccessibleLinks := lc.checkLinksWithConcurrency(ctx, externalLinks)

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
		// Skip internal links and duplicates
		if link.Type == domain.LinkTypeInternal {
			continue
		}

		// Skip if we've already seen this URL
		if seen[link.URL] {
			continue
		}
		seen[link.URL] = true

		// Skip invalid URLs
		if _, err := url.Parse(link.URL); err != nil {
			continue
		}

		externalLinks = append(externalLinks, link)
	}

	return externalLinks
}

func (lc *LinkChecker) checkLinksWithConcurrency(ctx context.Context, links []domain.Link) []domain.InaccessibleLink {
	var inaccessibleLinks []domain.InaccessibleLink
	var mu sync.Mutex

	semaphore := make(chan struct{}, lc.config.MaxConcurrentChecks)
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Add(1)
		go func(link domain.Link) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if inaccessibleLink := lc.checkSingleLink(ctx, link); inaccessibleLink != nil {
				mu.Lock()
				inaccessibleLinks = append(inaccessibleLinks, *inaccessibleLink)
				mu.Unlock()
			}
		}(link)
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
			Str("url", link.URL).
			Str("error", err.Error()).
			Int64("duration_ms", duration.Milliseconds()).
			Msg("Link check failed")

		if errors.Is(err, gobreaker.ErrOpenState) {
			return &domain.InaccessibleLink{
				URL:        link.URL,
				StatusCode: 503,
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
		Msg("Link check completed")

	if checkResult.StatusCode >= 400 {
		return &domain.InaccessibleLink{
			URL:        link.URL,
			StatusCode: checkResult.StatusCode,
			Error:      checkResult.Error,
		}
	}

	return nil
}

type linkCheckResult struct {
	StatusCode int
	Error      string
}

func (lc *LinkChecker) performLinkCheck(ctx context.Context, linkURL string) (*linkCheckResult, error) {
	// Use HEAD request first for efficiency
	resp, err := lc.client.R().
		SetContext(ctx).
		Head(linkURL)

	if err != nil {
		// If HEAD fails, try GET request
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

	if resp.StatusCode() >= 400 {
		result.Error = resp.Status()
	}

	return result, nil
}
