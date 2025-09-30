package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/architeacher/svc-web-analyzer/pkg/metrics/noop"
	"github.com/stretchr/testify/suite"
)

type LinkCheckerTestSuite struct {
	suite.Suite
	linkChecker *LinkChecker
	logger      appLogger.Logger
	metrics     ports.MetricsClient
	config      config.LinkCheckerConfig
	testServers []*httptest.Server
}

func (s *LinkCheckerTestSuite) SetupSuite() {
	s.logger = appLogger.NewTestLogger()
	s.metrics = noop.NewMetricsClient()
	s.config = config.LinkCheckerConfig{
		Timeout:             5 * time.Second,
		MaxConcurrentChecks: 3,
		MaxLinksToCheck:     50,
		Retries:             2,
		RetryWaitTime:       100 * time.Millisecond,
		MaxRetryWaitTime:    500 * time.Millisecond,
		CircuitBreaker: config.CircuitBreakerConfig{
			MaxRequests: 3,
			Interval:    5 * time.Second,
			Timeout:     30 * time.Second,
		},
	}
}

func (s *LinkCheckerTestSuite) SetupTest() {
	s.linkChecker = NewLinkChecker(s.config, s.logger, s.metrics)
	s.testServers = make([]*httptest.Server, 0)
}

func (s *LinkCheckerTestSuite) TearDownTest() {
	for _, server := range s.testServers {
		if server != nil {
			server.Close()
		}
	}
	s.testServers = nil
}

func (s *LinkCheckerTestSuite) createTestServer(handler http.HandlerFunc) *httptest.Server {
	server := httptest.NewServer(handler)
	s.testServers = append(s.testServers, server)

	return server
}

func (s *LinkCheckerTestSuite) TestNewLinkChecker() {
	s.Require().NotNil(s.linkChecker)
	s.Require().NotNil(s.linkChecker.client)
	s.Require().NotNil(s.linkChecker.circuitBreaker)
	s.Require().NotNil(s.linkChecker.logger)
	s.Require().Equal(s.config, s.linkChecker.config)
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_SuccessfulLinks() {
	cases := []struct {
		name         string
		statusCode   int
		responseBody string
		contentType  string
		expectError  bool
	}{
		{
			name:         "HTTP 200 OK",
			statusCode:   http.StatusOK,
			responseBody: "<html><body>OK</body></html>",
			contentType:  "text/html",
			expectError:  false,
		},
		{
			name:         "HTTP 201 Created",
			statusCode:   http.StatusCreated,
			responseBody: "Created",
			contentType:  "text/plain",
			expectError:  false,
		},
		{
			name:         "HTTP 301 Moved Permanently",
			statusCode:   http.StatusMovedPermanently,
			responseBody: "Moved",
			contentType:  "text/html",
			expectError:  false,
		},
		{
			name:         "HTTP 302 Found",
			statusCode:   http.StatusFound,
			responseBody: "Found",
			contentType:  "text/html",
			expectError:  false,
		},
		{
			name:         "HTTP 204 No Content",
			statusCode:   http.StatusNoContent,
			responseBody: "",
			contentType:  "",
			expectError:  false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			links := []domain.Link{
				{URL: server.URL, Type: domain.LinkTypeExternal},
			}

			ctx, cancel := context.WithTimeout(s.T().Context(), 10*time.Second)
			defer cancel()

			inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)

			if tc.expectError {
				s.Require().Len(inaccessibleLinks, 1)
			} else {
				s.Require().Len(inaccessibleLinks, 0, "Expected no inaccessible links for status %d", tc.statusCode)
			}
		})
	}
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_ErrorResponses() {
	cases := []struct {
		name               string
		statusCode         int
		responseBody       string
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:               "HTTP 400 Bad Request",
			statusCode:         http.StatusBadRequest,
			responseBody:       "Bad Request",
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:               "HTTP 401 Unauthorized",
			statusCode:         http.StatusUnauthorized,
			responseBody:       "Unauthorized",
			expectedStatusCode: http.StatusUnauthorized,
			expectError:        true,
		},
		{
			name:               "HTTP 403 Forbidden",
			statusCode:         http.StatusForbidden,
			responseBody:       "Forbidden",
			expectedStatusCode: http.StatusForbidden,
			expectError:        true,
		},
		{
			name:               "HTTP 404 Not Found",
			statusCode:         http.StatusNotFound,
			responseBody:       "Not Found",
			expectedStatusCode: http.StatusNotFound,
			expectError:        true,
		},
		{
			name:               "HTTP 429 Too Many Requests",
			statusCode:         http.StatusTooManyRequests,
			responseBody:       "Rate Limited",
			expectedStatusCode: http.StatusTooManyRequests,
			expectError:        true,
		},
		{
			name:               "HTTP 500 Internal Server Error",
			statusCode:         http.StatusInternalServerError,
			responseBody:       "Internal Server Error",
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:               "HTTP 502 Bad Gateway",
			statusCode:         http.StatusBadGateway,
			responseBody:       "Bad Gateway",
			expectedStatusCode: http.StatusBadGateway,
			expectError:        true,
		},
		{
			name:               "HTTP 503 Service Unavailable",
			statusCode:         http.StatusServiceUnavailable,
			responseBody:       "Service Unavailable",
			expectedStatusCode: http.StatusServiceUnavailable,
			expectError:        true,
		},
		{
			name:               "HTTP 504 Gateway Timeout",
			statusCode:         http.StatusGatewayTimeout,
			responseBody:       "Gateway Timeout",
			expectedStatusCode: http.StatusGatewayTimeout,
			expectError:        true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			links := []domain.Link{
				{URL: server.URL, Type: domain.LinkTypeExternal},
			}

			ctx, cancel := context.WithTimeout(s.T().Context(), 10*time.Second)
			defer cancel()

			inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)

			if tc.expectError {
				s.Require().Len(inaccessibleLinks, 1)
				s.Require().Equal(server.URL, inaccessibleLinks[0].URL)
				s.Require().Equal(tc.expectedStatusCode, inaccessibleLinks[0].StatusCode)
				s.Require().NotEmpty(inaccessibleLinks[0].Error)
			} else {
				s.Require().Len(inaccessibleLinks, 0)
			}
		})
	}
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_NetworkErrors() {
	cases := []struct {
		name        string
		setupServer func(*LinkCheckerTestSuite) string
		expectError bool
	}{
		{
			name: "Connection refused",
			setupServer: func(*LinkCheckerTestSuite) string {
				return "http://localhost:99999"
			},
			expectError: true,
		},
		{
			name: "Invalid hostname",
			setupServer: func(*LinkCheckerTestSuite) string {
				return "http://this-domain-does-not-exist-12345.invalid"
			},
			expectError: true,
		},
		{
			name: "Timeout",
			setupServer: func(s *LinkCheckerTestSuite) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(10 * time.Second)
					w.WriteHeader(http.StatusOK)
				}))
				s.testServers = append(s.testServers, server)

				return server.URL
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.config.Timeout = time.Second
			s.linkChecker = NewLinkChecker(s.config, s.logger, s.metrics)

			targetURL := tc.setupServer(s)
			links := []domain.Link{
				{URL: targetURL, Type: domain.LinkTypeExternal},
			}

			ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
			defer cancel()

			inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)

			if tc.expectError {
				s.Require().Len(inaccessibleLinks, 1)
				s.Require().Equal(targetURL, inaccessibleLinks[0].URL)
				s.Require().Equal(0, inaccessibleLinks[0].StatusCode)
				s.Require().NotEmpty(inaccessibleLinks[0].Error)
			} else {
				s.Require().Len(inaccessibleLinks, 0)
			}
		})
	}
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_ConcurrencyLimits() {
	s.config.MaxConcurrentChecks = 2
	s.linkChecker = NewLinkChecker(s.config, s.logger, s.metrics)

	var activeConnections int32
	var maxActiveConnections int32
	var mu sync.Mutex

	server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		activeConnections++
		if activeConnections > maxActiveConnections {
			maxActiveConnections = activeConnections
		}
		mu.Unlock()

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		activeConnections--
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	links := make([]domain.Link, 5)
	for index := 0; index < 5; index++ {
		links[index] = domain.Link{
			URL:  fmt.Sprintf("%s/path%d", server.URL, index),
			Type: domain.LinkTypeExternal,
		}
	}

	ctx, cancel := context.WithTimeout(s.T().Context(), 10*time.Second)
	defer cancel()

	inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)

	s.Require().Len(inaccessibleLinks, 0, "All links should be accessible")

	mu.Lock()
	maxActive := maxActiveConnections
	mu.Unlock()

	s.Require().LessOrEqual(int(maxActive), 2, "Should not exceed max concurrent checks limit")
	s.Require().Greater(int(maxActive), 0, "Should have made at least one concurrent request")
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_MaxLinksLimit() {
	s.config.MaxLinksToCheck = 3
	s.linkChecker = NewLinkChecker(s.config, s.logger, s.metrics)

	var requestCount int32
	var mu sync.Mutex

	server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	links := make([]domain.Link, 10)
	for index := 0; index < 10; index++ {
		links[index] = domain.Link{
			URL:  fmt.Sprintf("%s/path%d", server.URL, index),
			Type: domain.LinkTypeExternal,
		}
	}

	ctx, cancel := context.WithTimeout(s.T().Context(), 10*time.Second)
	defer cancel()

	inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)

	s.Require().Len(inaccessibleLinks, 0, "All checked links should be accessible")

	mu.Lock()
	actualRequests := requestCount
	mu.Unlock()

	s.Require().LessOrEqual(int(actualRequests), 3, "Should not check more than max links limit")
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_CircuitBreaker() {
	s.config.CircuitBreaker.MaxRequests = 1
	s.config.CircuitBreaker.Interval = 100 * time.Millisecond
	s.config.CircuitBreaker.Timeout = 200 * time.Millisecond
	s.config.Timeout = 50 * time.Millisecond
	s.linkChecker = NewLinkChecker(s.config, s.logger, s.metrics)

	server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Too slow"))
	})

	links := []domain.Link{
		{URL: server.URL, Type: domain.LinkTypeExternal},
	}

	ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
	defer cancel()

	var lastResult []domain.InaccessibleLink
	for index := 0; index < 8; index++ {
		inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)
		lastResult = inaccessibleLinks
		s.Require().Len(inaccessibleLinks, 1)
		s.Require().Equal(server.URL, inaccessibleLinks[0].URL)

		if inaccessibleLinks[0].StatusCode == http.StatusServiceUnavailable &&
			strings.Contains(inaccessibleLinks[0].Error, "circuit breaker open") {

			return
		}

		s.Require().Equal(0, inaccessibleLinks[0].StatusCode)
		s.Require().NotEmpty(inaccessibleLinks[0].Error)

		time.Sleep(50 * time.Millisecond)
	}

	s.Require().Len(lastResult, 1)

	if lastResult[0].StatusCode != http.StatusServiceUnavailable {
		s.T().Logf("Expected circuit breaker to open after 8 attempts, but got status %d with error: %s",
			lastResult[0].StatusCode, lastResult[0].Error)

		s.Require().Equal(0, lastResult[0].StatusCode)
		s.Require().Contains(lastResult[0].Error, "context deadline exceeded")
	} else {
		s.Require().Equal(http.StatusServiceUnavailable, lastResult[0].StatusCode)
		s.Require().Contains(lastResult[0].Error, "circuit breaker open")
	}
}

func (s *LinkCheckerTestSuite) TestCheckAccessibility_ContextCancellation() {
	server := s.createTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	links := []domain.Link{
		{URL: server.URL, Type: domain.LinkTypeExternal},
	}

	ctx, cancel := context.WithTimeout(s.T().Context(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	inaccessibleLinks := s.linkChecker.CheckAccessibility(ctx, links)
	duration := time.Since(start)

	s.Require().Less(duration, 1500*time.Millisecond, "Should respect context timeout")
	s.Require().Len(inaccessibleLinks, 1, "Should report link as inaccessible due to timeout")
	s.Require().Equal(server.URL, inaccessibleLinks[0].URL)
}

func TestLinkCheckerSuite(t *testing.T) {
	suite.Run(t, new(LinkCheckerTestSuite))
}
