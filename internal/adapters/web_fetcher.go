package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/go-resty/resty/v2"
	"github.com/sony/gobreaker"
)

const (
	minInputSize   = 3
	maxInputSize   = 10000
	defaultTimeout = 30 * time.Second
)

type WebPageFetcher struct {
	client         *resty.Client
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *infrastructure.Logger
	config         config.WebFetcherConfig
}

func NewWebPageFetcher(config config.WebFetcherConfig, logger *infrastructure.Logger) *WebPageFetcher {
	client := resty.New()

	client.SetTimeout(defaultTimeout)
	client.SetRetryCount(config.MaxRetries)
	client.SetRetryWaitTime(config.RetryWaitTime)
	client.SetRetryMaxWaitTime(config.MaxRetryWaitTime)
	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(config.MaxRedirects))

	if config.UserAgent != "" {
		client.SetHeader("User-Agent", config.UserAgent)
	} else {
		client.SetHeader("User-Agent", "WebPageAnalyzer/1.0")
	}

	client.SetHeaders(map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.5",
		"Accept-Encoding":           "gzip, deflate",
		"DNT":                       "1",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
	})

	cbSettings := gobreaker.Settings{
		Name:        "web-page-fetcher",
		MaxRequests: config.CircuitBreaker.MaxRequests,
		Interval:    config.CircuitBreaker.Interval,
		Timeout:     config.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info().
				Str("name", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("Circuit breaker state changed")
		},
	}

	circuitBreaker := gobreaker.NewCircuitBreaker(cbSettings)

	return &WebPageFetcher{
		client:         client,
		circuitBreaker: circuitBreaker,
		logger:         logger,
		config:         config,
	}
}

func (f *WebPageFetcher) Fetch(ctx context.Context, targetURL string, timeout time.Duration) (*domain.WebPageContent, error) {
	if err := f.validateURL(targetURL); err != nil {
		return nil, domain.NewInvalidURLError(targetURL, err)
	}

	if timeout > 0 {
		f.client.SetTimeout(timeout)
	}

	result, err := f.circuitBreaker.Execute(func() (any, error) {
		return f.fetchWithRetry(ctx, targetURL)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			f.logger.Warn().Str("url", targetURL).Msg("Circuit breaker is open")
			return nil, domain.NewDomainError(
				"CIRCUIT_BREAKER_OPEN",
				"Service temporarily unavailable due to repeated failures",
				503,
				err,
			)
		}
		return nil, err
	}

	return result.(*domain.WebPageContent), nil
}

func (f *WebPageFetcher) fetchWithRetry(ctx context.Context, targetURL string) (*domain.WebPageContent, error) {
	startTime := time.Now()

	resp, err := f.client.R().
		SetContext(ctx).
		Get(targetURL)

	duration := time.Since(startTime)

	f.logger.Info().
		Str("url", targetURL).
		Int("status_code", resp.StatusCode()).
		Int64("duration_ms", duration.Milliseconds()).
		Int("size_bytes", len(resp.Body())).
		Str("content_type", resp.Header().Get("Content-Type")).
		Msg("HTTP request completed")

	if err != nil {
		f.logger.Error().
			Str("url", targetURL).
			Str("error", err.Error()).
			Msg("Failed to fetch URL")

		return nil, domain.NewURLNotReachableError(targetURL, 0, err)
	}

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		f.logger.Warn().
			Str("url", targetURL).
			Int("status_code", resp.StatusCode()).
			Msg("HTTP request returned non-success status code")

		return nil, domain.NewURLNotReachableError(
			targetURL,
			resp.StatusCode(),
			fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.Status()),
		)
	}

	if len(resp.Body()) > int(f.config.MaxResponseSizeBytes) {
		return nil, domain.NewDomainError(
			"RESPONSE_TOO_LARGE",
			fmt.Sprintf("Response size %d bytes exceeds maximum allowed %d bytes",
				len(resp.Body()), f.config.MaxResponseSizeBytes),
			413,
			fmt.Errorf("response too large"),
		)
	}

	contentType := resp.Header().Get("Content-Type")
	if !isHTMLContent(contentType) {
		f.logger.Warn().
			Str("url", targetURL).
			Str("content_type", contentType).
			Msg("Response is not HTML content")
	}

	headers := make(map[string]string)
	for key, values := range resp.Header() {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &domain.WebPageContent{
		URL:         resp.Request.URL,
		StatusCode:  resp.StatusCode(),
		HTML:        string(resp.Body()),
		ContentType: contentType,
		Headers:     headers,
	}, nil
}

func (f *WebPageFetcher) validateURL(targetURL string) error {
	if targetURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Validate URL length constraints (matching OpenAPI spec)
	if len(targetURL) < minInputSize {
		return fmt.Errorf("URL must be at least %d characters long", minInputSize)
	}
	if len(targetURL) > maxInputSize {
		return fmt.Errorf("URL must not exceed %d characters", maxInputSize)
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL must include a scheme (http or https)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	// Prevent access to local/private networks for security
	if isPrivateOrLocalURL(parsedURL.Hostname()) {
		return fmt.Errorf("access to private or local networks is not allowed")
	}

	return nil
}

func isHTMLContent(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "application/xhtml")
}

func isPrivateOrLocalURL(host string) bool {
	privateHosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
	}

	hostLower := strings.ToLower(host)
	for _, privateHost := range privateHosts {
		if hostLower == privateHost || strings.HasSuffix(hostLower, "."+privateHost) {
			return true
		}
	}

	// Check for private IP ranges
	if strings.HasPrefix(hostLower, "10.") ||
		strings.HasPrefix(hostLower, "172.16.") ||
		strings.HasPrefix(hostLower, "172.17.") ||
		strings.HasPrefix(hostLower, "172.18.") ||
		strings.HasPrefix(hostLower, "172.19.") ||
		strings.HasPrefix(hostLower, "172.20.") ||
		strings.HasPrefix(hostLower, "172.21.") ||
		strings.HasPrefix(hostLower, "172.22.") ||
		strings.HasPrefix(hostLower, "172.23.") ||
		strings.HasPrefix(hostLower, "172.24.") ||
		strings.HasPrefix(hostLower, "172.25.") ||
		strings.HasPrefix(hostLower, "172.26.") ||
		strings.HasPrefix(hostLower, "172.27.") ||
		strings.HasPrefix(hostLower, "172.28.") ||
		strings.HasPrefix(hostLower, "172.29.") ||
		strings.HasPrefix(hostLower, "172.30.") ||
		strings.HasPrefix(hostLower, "172.31.") ||
		strings.HasPrefix(hostLower, "192.168.") {
		return true
	}

	return false
}
