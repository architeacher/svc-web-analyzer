package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WebFetcherTestSuite implements a custom test suite pattern for web fetcher tests
type WebFetcherTestSuite struct {
	fetcher    *TestWebPageFetcher
	logger     infrastructure.Logger
	config     config.WebFetcherConfig
	testServer *httptest.Server
	t          *testing.T
}

// TestWebPageFetcher is a test version of WebFetcher that doesn't block local URLs
type TestWebPageFetcher struct {
	*WebFetcher
}

// Fetch overrides the normal Fetch to use the test validation
func (f *TestWebPageFetcher) Fetch(ctx context.Context, targetURL string, timeout time.Duration) (*domain.WebPageContent, error) {
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
				http.StatusServiceUnavailable,
				err,
			)
		}
		return nil, err
	}

	return result.(*domain.WebPageContent), nil
}

// validateURL overrides the normal validation to allow local URLs for testing
func (f *TestWebPageFetcher) validateURL(targetURL string) error {
	if targetURL == "" {
		return fmt.Errorf("URL cannot be empty")
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

	// For testing, we don't block local networks
	return nil
}

// newWebFetcherTestSuite creates a new test suite instance
func newWebFetcherTestSuite(t *testing.T) *WebFetcherTestSuite {
	cfg := config.WebFetcherConfig{
		MaxRetries:           3,
		RetryWaitTime:        100 * time.Millisecond,
		MaxRetryWaitTime:     1 * time.Second,
		MaxRedirects:         10,
		MaxResponseSizeBytes: 10 * 1024 * 1024, // 10MB
		UserAgent:            "WebAnalyzer/1.0",
		CircuitBreaker: config.CircuitBreakerConfig{
			MaxRequests: 3,
			Interval:    10 * time.Second,
			Timeout:     60 * time.Second,
		},
	}

	return &WebFetcherTestSuite{
		logger: infrastructure.Logger{Logger: zerolog.Nop()},
		config: cfg,
		t:      t,
	}
}

// SetupTest sets up resources before each test
func (suite *WebFetcherTestSuite) SetupTest() {
	baseFetcher := NewWebFetcher(suite.config, suite.logger)
	suite.fetcher = &TestWebPageFetcher{WebFetcher: baseFetcher}
}

// TearDownTest cleans up resources after each test
func (suite *WebFetcherTestSuite) TearDownTest() {
	if suite.testServer != nil {
		suite.testServer.Close()
		suite.testServer = nil
	}
}

// createTestServer creates a test HTTP server with custom handler
func (suite *WebFetcherTestSuite) createTestServer(handler http.HandlerFunc) {
	suite.testServer = httptest.NewServer(handler)
}

// ServerOption defines a functional option for configuring test servers
type ServerOption func(*serverConfig)

// serverConfig holds configuration for creating test servers (internal struct)
type serverConfig struct {
	StatusCode   int
	ContentType  string
	ResponseBody string
	Headers      map[string]string
	Delay        time.Duration
	ResponseSize int // For generating large responses
}

// WithStatusCode sets the HTTP status code for the server response
func WithStatusCode(code int) ServerOption {
	return func(config *serverConfig) {
		config.StatusCode = code
	}
}

// WithContentType sets the Content-Type header for the server response
func WithContentType(contentType string) ServerOption {
	return func(config *serverConfig) {
		config.ContentType = contentType
	}
}

// WithResponseBody sets the response body content
func WithResponseBody(body string) ServerOption {
	return func(config *serverConfig) {
		config.ResponseBody = body
	}
}

// WithHeaders sets custom headers for the server response
func WithHeaders(headers map[string]string) ServerOption {
	return func(config *serverConfig) {
		if config.Headers == nil {
			config.Headers = make(map[string]string)
		}
		for key, value := range headers {
			config.Headers[key] = value
		}
	}
}

// WithDelay adds a delay before responding
func WithDelay(delay time.Duration) ServerOption {
	return func(config *serverConfig) {
		config.Delay = delay
	}
}

// WithResponseSize generates a response of specified size (useful for testing large responses)
func WithResponseSize(size int) ServerOption {
	return func(config *serverConfig) {
		config.ResponseSize = size
	}
}

// createSimpleServer creates a server with functional options configuration
func (suite *WebFetcherTestSuite) createSimpleServer(options ...ServerOption) *httptest.Server {
	// Apply default configuration
	config := &serverConfig{
		StatusCode: http.StatusOK,
	}

	// Apply all options
	for _, option := range options {
		option(config)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add delay if specified
		if config.Delay > 0 {
			time.Sleep(config.Delay)
		}

		// Middleware content type
		if config.ContentType != "" {
			w.Header().Set("Content-Type", config.ContentType)
		}

		// Middleware custom headers
		for key, value := range config.Headers {
			w.Header().Set(key, value)
		}

		// Middleware status code
		w.WriteHeader(config.StatusCode)

		// Write response body
		var responseBody string
		if config.ResponseSize > 0 {
			// Generate large response
			responseBody = strings.Repeat("a", config.ResponseSize)
		} else {
			responseBody = config.ResponseBody
		}

		if responseBody != "" {
			_, err := w.Write([]byte(responseBody))
			if err != nil {
				panic(err) // Use panic in test server handlers since we can't access testing.T
			}
		}
	}))
}

// createRedirectServer creates a server that redirects to another URL
func (suite *WebFetcherTestSuite) createRedirectServer(targetURL string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode == 0 {
			statusCode = http.StatusFound
		}
		http.Redirect(w, r, targetURL, statusCode)
	}))
}

// TestNewWebPageFetcher tests the constructor
func (suite *WebFetcherTestSuite) TestNewWebPageFetcher() {
	require.NotNil(suite.t, suite.fetcher)
	require.NotNil(suite.t, suite.fetcher.client)
	require.NotNil(suite.t, suite.fetcher.circuitBreaker)
	require.NotNil(suite.t, suite.fetcher.logger)
	assert.Equal(suite.t, suite.config, suite.fetcher.config)
}

// TestFetch_SuccessfulRequest tests successful HTTP requests
func (suite *WebFetcherTestSuite) TestFetch_SuccessfulRequest() {
	cases := []struct {
		name           string
		responseBody   string
		responseCode   int
		contentType    string
		customHeaders  map[string]string
		expectedStatus int
	}{
		{
			name:           "HTML content",
			responseBody:   "<html><head><title>Test</title></head><body><h1>Hello World</h1></body></html>",
			responseCode:   http.StatusOK,
			contentType:    "text/html; charset=utf-8",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "XHTML content",
			responseBody:   "<?xml version=\"1.0\"?><html><head><title>Test</title></head></html>",
			responseCode:   http.StatusOK,
			contentType:    "application/xhtml+xml",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Non-HTML content",
			responseBody:   `{"message": "Hello World"}`,
			responseCode:   http.StatusOK,
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Custom headers",
			responseBody:   "<html><body>Test</body></html>",
			responseCode:   http.StatusOK,
			contentType:    "text/html",
			customHeaders:  map[string]string{"X-Custom-Header": "test-value"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create test server for this specific test using helper method
			subSuite := newWebFetcherTestSuite(t)
			server := subSuite.createSimpleServer(
				WithStatusCode(tc.responseCode),
				WithContentType(tc.contentType),
				WithResponseBody(tc.responseBody),
				WithHeaders(tc.customHeaders),
			)
			defer server.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			// Create a fresh suite for each sub-test
			subSuite = newWebFetcherTestSuite(t)
			subSuite.SetupTest()
			defer subSuite.TearDownTest()

			result, err := subSuite.fetcher.Fetch(ctx, server.URL, 0)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.expectedStatus, result.StatusCode)
			assert.Equal(t, tc.responseBody, result.HTML)
			assert.Equal(t, tc.contentType, result.ContentType)
			assert.Equal(t, server.URL, result.URL)

			// Check custom headers
			for key, expectedValue := range tc.customHeaders {
				assert.Equal(t, expectedValue, result.Headers[key])
			}
		})
	}
}

// TestFetch_ErrorCases tests various error scenarios
func (suite *WebFetcherTestSuite) TestFetch_ErrorCases() {
	cases := []struct {
		name           string
		setupServer    func() *httptest.Server
		url            string
		timeout        time.Duration
		expectedErrMsg string
		expectedCode   string
	}{
		{
			name: "HTTP 404 Not Found",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusNotFound),
					WithResponseBody("Not Found"),
				)
			},
			expectedErrMsg: "URL .* is not reachable",
			expectedCode:   "URL_NOT_REACHABLE",
		},
		{
			name: "HTTP 500 Internal Server Error",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusInternalServerError),
					WithResponseBody("Internal Server Error"),
				)
			},
			expectedErrMsg: "URL .* is not reachable",
			expectedCode:   "URL_NOT_REACHABLE",
		},
		{
			name: "Response too large",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithContentType("text/html"),
					WithResponseSize(11*1024*1024), // Write more than 10MB
				)
			},
			expectedErrMsg: "Response size .* bytes exceeds maximum allowed",
			expectedCode:   "RESPONSE_TOO_LARGE",
		},
		{
			name: "Context timeout",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithContentType("text/html"),
					WithResponseBody("OK"),
					WithDelay(2*time.Second),
				)
			},
			timeout:        100 * time.Millisecond,
			expectedErrMsg: "URL .* is not reachable",
			expectedCode:   "URL_NOT_REACHABLE",
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh fetcher for each sub-test to avoid circuit breaker interference
			subSuite := newWebFetcherTestSuite(t)
			subSuite.SetupTest()
			defer subSuite.TearDownTest()

			var testURL string
			if tc.setupServer != nil {
				server := tc.setupServer()
				defer server.Close()
				testURL = server.URL
			} else {
				testURL = tc.url
			}

			ctx := t.Context()
			if tc.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.timeout)
				defer cancel()
			}

			result, err := subSuite.fetcher.Fetch(ctx, testURL, tc.timeout)

			require.Nil(t, result)
			require.Error(t, err)

			var domainErr *domain.DomainError
			require.True(t, errors.As(err, &domainErr), "Expected domain error, got %T", err)
			assert.Equal(t, tc.expectedCode, domainErr.Code)
			assert.Regexp(t, tc.expectedErrMsg, domainErr.Message)
		})
	}
}

// TestFetch_InvalidURLs tests URL validation
func (suite *WebFetcherTestSuite) TestFetch_InvalidURLs() {
	// Note: HTTP URLs in test cases are intentional for testing URL validation logic
	cases := []struct {
		name           string
		url            string
		expectedErrMsg string
		expectedCode   string
		useRealFetcher bool // Use real fetcher instead of TestWebPageFetcher for validation tests
	}{
		{
			name:           "Empty URL",
			url:            "",
			expectedErrMsg: "Invalid URL: ",
			expectedCode:   "INVALID_URL",
			useRealFetcher: false,
		},
		{
			name:           "URL without scheme",
			url:            "example.com",
			expectedErrMsg: "Invalid URL: example.com",
			expectedCode:   "INVALID_URL",
			useRealFetcher: false,
		},
		{
			name:           "Invalid scheme",
			url:            "ftp://example.com",
			expectedErrMsg: "Invalid URL: ftp://example.com",
			expectedCode:   "INVALID_URL",
			useRealFetcher: false,
		},
		{
			name:           "URL without host",
			url:            "http://", // HTTP URL is intentional for testing validation
			expectedErrMsg: "Invalid URL: http://",
			expectedCode:   "INVALID_URL",
			useRealFetcher: false,
		},
		{
			name:           "Localhost access blocked",
			url:            "http://localhost:8080", // HTTP URL is intentional for testing local network blocking
			expectedErrMsg: "Invalid URL: http://localhost:8080",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "127.0.0.1 access blocked",
			url:            "http://127.0.0.1:8080",
			expectedErrMsg: "Invalid URL: http://127.0.0.1:8080",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "Private IP 192.168.x.x blocked",
			url:            "http://192.168.1.1",
			expectedErrMsg: "Invalid URL: http://192.168.1.1",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "Private IP 10.x.x.x blocked",
			url:            "http://10.0.0.1",
			expectedErrMsg: "Invalid URL: http://10.0.0.1",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "Private IP 172.16.x.x blocked",
			url:            "http://172.16.0.1",
			expectedErrMsg: "Invalid URL: http://172.16.0.1",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "URL too short (2 chars)",
			url:            "ab",
			expectedErrMsg: "Invalid URL: ab",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
		{
			name:           "URL exactly at minimum length (3 chars)",
			url:            "a.b",
			expectedErrMsg: "Invalid URL: a.b",
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation (will fail on other validation)
		},
		{
			name:           "URL too long (>10000 chars)",
			url:            "https://example.com/" + strings.Repeat("a", 10000),
			expectedErrMsg: "Invalid URL: https://example.com/" + strings.Repeat("a", 10000),
			expectedCode:   "INVALID_URL",
			useRealFetcher: true, // Use real fetcher to test actual validation
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			var result *domain.WebPageContent
			var err error

			if tc.useRealFetcher {
				cfg := config.WebFetcherConfig{
					MaxRetries:           3,
					RetryWaitTime:        100 * time.Millisecond,
					MaxRetryWaitTime:     1 * time.Second,
					MaxRedirects:         10,
					MaxResponseSizeBytes: 10 * 1024 * 1024,
					UserAgent:            "WebAnalyzer/1.0",
					CircuitBreaker: config.CircuitBreakerConfig{
						MaxRequests: 3,
						Interval:    10 * time.Second,
						Timeout:     60 * time.Second,
					},
				}
				realFetcher := NewWebFetcher(cfg, infrastructure.Logger{Logger: zerolog.Nop()})
				result, err = realFetcher.Fetch(ctx, tc.url, 0)
			} else {
				// Use TestWebPageFetcher for other tests
				subSuite := newWebFetcherTestSuite(t)
				subSuite.SetupTest()
				defer subSuite.TearDownTest()
				result, err = subSuite.fetcher.Fetch(ctx, tc.url, 0)
			}

			require.Nil(t, result)
			require.Error(t, err)

			var domainErr *domain.DomainError
			require.True(t, errors.As(err, &domainErr), "Expected domain error, got %T", err)
			assert.Equal(t, tc.expectedCode, domainErr.Code)
			assert.Equal(t, tc.expectedErrMsg, domainErr.Message)
		})
	}
}

// TestFetch_CircuitBreaker tests circuit breaker functionality
func (suite *WebFetcherTestSuite) TestFetch_CircuitBreaker() {
	// Configure circuit breaker to open quickly for testing
	suite.config.CircuitBreaker.MaxRequests = 1
	suite.config.CircuitBreaker.Interval = 100 * time.Millisecond
	suite.config.CircuitBreaker.Timeout = 200 * time.Millisecond

	// Create a server that always returns 500
	server := suite.createSimpleServer(
		WithStatusCode(http.StatusInternalServerError),
		WithResponseBody("Internal Server Error"),
	)
	defer server.Close()

	ctx := suite.t.Context()

	// First few requests should fail and trigger circuit breaker
	for i := 0; i < 4; i++ {
		result, err := suite.fetcher.Fetch(ctx, server.URL, 0)
		require.Nil(suite.t, result)
		require.Error(suite.t, err)
	}

	// Wait a bit for circuit breaker to open
	time.Sleep(150 * time.Millisecond)

	// Next request should fail with circuit breaker open error
	result, err := suite.fetcher.Fetch(ctx, server.URL, 0)
	require.Nil(suite.t, result)
	require.Error(suite.t, err)

	var domainErr *domain.DomainError
	require.True(suite.t, errors.As(err, &domainErr), "Expected domain error with CIRCUIT_BREAKER_OPEN, got %T", err)
	assert.Equal(suite.t, "CIRCUIT_BREAKER_OPEN", domainErr.Code)
	assert.Contains(suite.t, domainErr.Message, "Service temporarily unavailable")
}

// TestFetch_TimeoutSettings tests timeout configuration
func (suite *WebFetcherTestSuite) TestFetch_TimeoutSettings() {
	cases := []struct {
		name        string
		timeout     time.Duration
		serverDelay time.Duration
		shouldFail  bool
	}{
		{
			name:        "Request completes within timeout",
			timeout:     1 * time.Second,
			serverDelay: 100 * time.Millisecond,
			shouldFail:  false,
		},
		{
			name:        "Request exceeds timeout",
			timeout:     100 * time.Millisecond,
			serverDelay: 1 * time.Second,
			shouldFail:  true,
		},
		{
			name:        "Zero timeout uses default",
			timeout:     0,
			serverDelay: 100 * time.Millisecond,
			shouldFail:  false,
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subSuite := newWebFetcherTestSuite(t)
			server := subSuite.createSimpleServer(
				WithStatusCode(http.StatusOK),
				WithContentType("text/html"),
				WithResponseBody("<html><body>OK</body></html>"),
				WithDelay(tc.serverDelay),
			)
			defer server.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			// Create a fresh fetcher for each sub-test to avoid issues
			subSuite = newWebFetcherTestSuite(t)
			subSuite.SetupTest()
			defer subSuite.TearDownTest()

			result, err := subSuite.fetcher.Fetch(ctx, server.URL, tc.timeout)

			if tc.shouldFail {
				require.Nil(t, result)
				require.Error(t, err)
			} else {
				require.NotNil(t, result)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, result.StatusCode)
			}
		})
	}
}

// TestFetch_Redirects tests redirect handling
func (suite *WebFetcherTestSuite) TestFetch_Redirects() {
	// Create servers for redirect testing
	finalServer := suite.createSimpleServer(
		WithStatusCode(http.StatusOK),
		WithContentType("text/html; charset=utf-8"),
		WithResponseBody("<html><body>Final destination</body></html>"),
	)
	defer finalServer.Close()

	redirectServer := suite.createRedirectServer(finalServer.URL, http.StatusFound)
	defer redirectServer.Close()

	ctx, cancel := context.WithTimeout(suite.t.Context(), 5*time.Second)
	defer cancel()

	result, err := suite.fetcher.Fetch(ctx, redirectServer.URL, 0)

	require.NoError(suite.t, err)
	require.NotNil(suite.t, result)
	assert.Equal(suite.t, http.StatusOK, result.StatusCode)
	assert.Contains(suite.t, result.HTML, "Final destination")

	// Parse both URLs to compare without dynamic port numbers
	expectedURL, err := url.Parse(finalServer.URL)
	require.NoError(suite.t, err)
	actualURL, err := url.Parse(result.URL)
	require.NoError(suite.t, err)

	// Verify the final URL is the expected final server URL
	// Compare scheme, hostname, and path, but not port since ports are dynamic
	assert.Equal(suite.t, expectedURL.Scheme, actualURL.Scheme)
	assert.Equal(suite.t, expectedURL.Hostname(), actualURL.Hostname())
	assert.Equal(suite.t, expectedURL.Path, actualURL.Path)

	// Verify that both URLs actually point to the same test server
	// by checking that they both use localhost/127.0.0.1
	assert.Equal(suite.t, "127.0.0.1", actualURL.Hostname())
	assert.Equal(suite.t, "127.0.0.1", expectedURL.Hostname())
}

// TestValidateURL tests URL validation logic separately
func (suite *WebFetcherTestSuite) TestValidateURL() {
	// Note: HTTP URLs in test cases are intentional for testing URL validation logic
	cases := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid HTTP URL",
			url:     "http://example.com", // HTTP URL is intentional for testing URL validation
			wantErr: false,
		},
		{
			name:    "Valid HTTPS URL",
			url:     "https://example.com/path?query=value",
			wantErr: false,
		},
		{
			name:    "Empty URL",
			url:     "",
			wantErr: true,
			errMsg:  "URL cannot be empty",
		},
		{
			name:    "URL without scheme",
			url:     "example.com",
			wantErr: true,
			errMsg:  "URL must include a scheme",
		},
		{
			name:    "Invalid scheme",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "URL scheme must be http or https",
		},
		{
			name:    "URL without host",
			url:     "http://", // HTTP URL is intentional for testing URL validation
			wantErr: true,
			errMsg:  "URL must include a host",
		},
		{
			name:    "Localhost blocked",
			url:     "http://localhost",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
		{
			name:    "127.0.0.1 blocked",
			url:     "http://127.0.0.1",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
		{
			name:    "IPv6 localhost blocked",
			url:     "http://[::1]:8080",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
		{
			name:    "Private IP 192.168.x.x blocked",
			url:     "http://192.168.1.1",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
		{
			name:    "Private IP 10.x.x.x blocked",
			url:     "http://10.0.0.1",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
		{
			name:    "Private IP 172.16.x.x blocked",
			url:     "http://172.16.0.1",
			wantErr: true,
			errMsg:  "access to private or local networks is not allowed",
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			realFetcher := NewWebFetcher(config.WebFetcherConfig{}, infrastructure.Logger{Logger: zerolog.Nop()})

			err := realFetcher.validateURL(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestIsHTMLContent tests HTML content type detection
func (suite *WebFetcherTestSuite) TestIsHTMLContent() {
	cases := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "text/html",
			contentType: "text/html",
			expected:    true,
		},
		{
			name:        "text/html with charset",
			contentType: "text/html; charset=utf-8",
			expected:    true,
		},
		{
			name:        "application/xhtml+xml",
			contentType: "application/xhtml+xml",
			expected:    true,
		},
		{
			name:        "application/xhtml with charset",
			contentType: "application/xhtml+xml; charset=utf-8",
			expected:    true,
		},
		{
			name:        "Case insensitive HTML",
			contentType: "TEXT/HTML",
			expected:    true,
		},
		{
			name:        "Case insensitive XHTML",
			contentType: "APPLICATION/XHTML+XML",
			expected:    true,
		},
		{
			name:        "application/json",
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "text/plain",
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "image/png",
			contentType: "image/png",
			expected:    false,
		},
		{
			name:        "Empty content type",
			contentType: "",
			expected:    false,
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isHTMLContent(tc.contentType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestIsPrivateOrLocalURL tests private/local URL detection
func (suite *WebFetcherTestSuite) TestIsPrivateOrLocalURL() {
	cases := []struct {
		name     string
		host     string
		expected bool
	}{
		{
			name:     "localhost",
			host:     "localhost",
			expected: true,
		},
		{
			name:     "Localhost uppercase",
			host:     "LOCALHOST",
			expected: true,
		},
		{
			name:     "127.0.0.1",
			host:     "127.0.0.1",
			expected: true,
		},
		{
			name:     "IPv6 localhost",
			host:     "::1",
			expected: true,
		},
		{
			name:     "IPv6 localhost with brackets",
			host:     "[::1]",
			expected: false, // brackets are stripped by URL parsing, so this should be false
		},
		{
			name:     "0.0.0.0",
			host:     "0.0.0.0",
			expected: true,
		},
		{
			name:     "Private IP 192.168.1.1",
			host:     "192.168.1.1",
			expected: true,
		},
		{
			name:     "Private IP 10.0.0.1",
			host:     "10.0.0.1",
			expected: true,
		},
		{
			name:     "Private IP 172.16.0.1",
			host:     "172.16.0.1",
			expected: true,
		},
		{
			name:     "Private IP 172.31.255.255",
			host:     "172.31.255.255",
			expected: true,
		},
		{
			name:     "Public IP 8.8.8.8",
			host:     "8.8.8.8",
			expected: false,
		},
		{
			name:     "Public domain",
			host:     "example.com",
			expected: false,
		},
		{
			name:     "Subdomain localhost",
			host:     "api.localhost",
			expected: true,
		},
		{
			name:     "Not private 172.32.0.1",
			host:     "172.32.0.1",
			expected: false,
		},
		{
			name:     "Not private 172.15.0.1",
			host:     "172.15.0.1",
			expected: false,
		},
		{
			name:     "Edge case 192.169.0.1",
			host:     "192.169.0.1",
			expected: false,
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isPrivateOrLocalURL(tc.host)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestFetch_EdgeCases tests edge cases and boundary conditions
func (suite *WebFetcherTestSuite) TestFetch_EdgeCases() {
	cases := []struct {
		name         string
		setupServer  func() *httptest.Server
		expectedErr  bool
		expectedCode string
	}{
		{
			name: "Empty response body",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithContentType("text/html; charset=utf-8"),
					WithResponseBody(""),
				)
			},
			expectedErr: false,
		},
		{
			name: "Response with only whitespace",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithContentType("text/html; charset=utf-8"),
					WithResponseBody("   \n\t   "),
				)
			},
			expectedErr: false,
		},
		{
			name: "Response exactly at size limit",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithContentType("text/html"),
					WithResponseSize(10*1024*1024), // Exactly 10MB
				)
			},
			expectedErr: false,
		},
		{
			name: "Missing Content-Type header",
			setupServer: func() *httptest.Server {
				subSuite := newWebFetcherTestSuite(nil)
				// No Content-Type header set
				return subSuite.createSimpleServer(
					WithStatusCode(http.StatusOK),
					WithResponseBody("<html><body>Test</body></html>"),
				)
			},
			expectedErr: false,
		},
	}

	for _, tc := range cases {
		suite.t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := tc.setupServer()
			defer server.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			// Create a fresh fetcher for each sub-test to avoid issues
			subSuite := newWebFetcherTestSuite(t)
			subSuite.SetupTest()
			defer subSuite.TearDownTest()

			result, err := subSuite.fetcher.Fetch(ctx, server.URL, 0)

			if tc.expectedErr {
				require.Nil(t, result)
				require.Error(t, err)
				if tc.expectedCode != "" {
					var domainErr *domain.DomainError
					require.True(t, errors.As(err, &domainErr), "Expected domain error, got %T", err)
					assert.Equal(t, tc.expectedCode, domainErr.Code)
				}
			} else {
				require.NotNil(t, result)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, result.StatusCode)
			}
		})
	}
}

// TestFetch_ConcurrentRequests tests concurrent request handling
func (suite *WebFetcherTestSuite) TestFetch_ConcurrentRequests() {
	server := suite.createSimpleServer(
		WithStatusCode(http.StatusOK),
		WithContentType("text/html"),
		WithResponseBody(fmt.Sprintf("<html><body>Response at %s</body></html>", time.Now().Format(time.RFC3339Nano))),
		WithDelay(10*time.Millisecond), // Add a small delay to simulate real network latency
	)
	defer server.Close()

	const numRequests = 10
	results := make(chan error, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			// Create a fresh fetcher for each goroutine to avoid race conditions
			subSuite := newWebFetcherTestSuite(suite.t)
			subSuite.SetupTest()
			defer subSuite.TearDownTest()

			ctx, cancel := context.WithTimeout(suite.t.Context(), 5*time.Second)
			defer cancel()

			result, err := subSuite.fetcher.Fetch(ctx, server.URL, 0)
			if err != nil {
				results <- err
				return
			}

			if result == nil || result.StatusCode != http.StatusOK {
				results <- fmt.Errorf("unexpected result for request %d", id)
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		require.NoError(suite.t, err, "Request %d failed", i)
	}
}

// Custom test suite runner that discovers and executes all test methods
func runWebFetcherSuite(t *testing.T, suite *WebFetcherTestSuite) {
	// Use reflection to find all methods starting with "Test"
	suiteType := reflect.TypeOf(suite)

	var testMethods []reflect.Method
	for i := 0; i < suiteType.NumMethod(); i++ {
		method := suiteType.Method(i)
		if strings.HasPrefix(method.Name, "Test") {
			testMethods = append(testMethods, method)
		}
	}

	// Run each test method as a subtest
	for _, method := range testMethods {
		t.Run(method.Name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh suite instance for each test
			testSuite := newWebFetcherTestSuite(t)
			testSuite.SetupTest()
			defer testSuite.TearDownTest()

			// Call the test method
			methodValue := reflect.ValueOf(testSuite).MethodByName(method.Name)
			if methodValue.IsValid() {
				methodValue.Call([]reflect.Value{})
			}
		})
	}
}

// TestWebFetcherSuite is the main test entry point that runs all test methods
func TestWebFetcherSuite(t *testing.T) {
	suite := newWebFetcherTestSuite(t)

	runWebFetcherSuite(t, suite)
}
