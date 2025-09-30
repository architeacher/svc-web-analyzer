package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
	"github.com/stretchr/testify/suite"
)

const (
	testCacheKeyPrefix = "svc-web-analyzer:response:v1:"
)

type ResponseCacheMiddlewareTestSuite struct {
	suite.Suite
	middleware  *ResponseCacheMiddleware
	cacheClient *infrastructure.KeydbClient
	logger      appLogger.Logger
	config      ResponseCacheConfig
}

func TestResponseCacheMiddlewareTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ResponseCacheMiddlewareTestSuite))
}

func (suite *ResponseCacheMiddlewareTestSuite) SetupTest() {
	suite.logger = appLogger.NewTestLogger()

	cacheConfig := config.CacheConfig{
		Addr:          "localhost:6379",
		Password:      "",
		DB:            0,
		PoolSize:      10,
		MinIdleConns:  3,
		DialTimeout:   5 * time.Second,
		ReadTimeout:   3 * time.Second,
		WriteTimeout:  3 * time.Second,
		PoolTimeout:   5 * time.Second,
		MaxRetries:    3,
		DefaultExpiry: 1 * time.Hour,
	}

	suite.cacheClient = infrastructure.NewKeyDBClient(cacheConfig, suite.logger)

	suite.config = ResponseCacheConfig{
		Enabled:             true,
		DefaultTTL:          5 * time.Minute,
		MaxCacheSize:        1024 * 1024, // 1MB
		CacheableStatuses:   []int{http.StatusOK, http.StatusNotFound},
		CacheableMethods:    []string{http.MethodGet, http.MethodHead},
		SkipPaths:           []string{"/health", "/metricsClient"},
		RespectCacheControl: true,
		EnableETag:          true,
		VaryHeaders:         []string{"Accept", "Accept-Language"},
	}

	suite.middleware = NewResponseCacheMiddleware(suite.cacheClient, suite.logger, suite.config)
}

func (suite *ResponseCacheMiddlewareTestSuite) TearDownTest() {
	if suite.cacheClient != nil {
		_ = suite.cacheClient.Close()
	}
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheKeyGeneration_WithoutAuth() {
	suite.T().Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	req.Header.Set("Accept", "application/json")

	key := suite.middleware.generateCacheKey(req)

	suite.Require().NotEmpty(key)
	suite.Require().Contains(key, testCacheKeyPrefix)
	// Key should be deterministic for the same request
	key2 := suite.middleware.generateCacheKey(req)
	suite.Require().Equal(key, key2)
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheKeyGeneration_WithAuth() {
	suite.T().Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	req.Header.Set("Accept", "application/json")

	// Add auth context
	ctx := context.WithValue(req.Context(), "subject", "user123")
	req = req.WithContext(ctx)

	key := suite.middleware.generateCacheKey(req)

	suite.Require().NotEmpty(key)
	suite.Require().Contains(key, testCacheKeyPrefix)

	// Different user should have different cache key
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	req2.Header.Set("Accept", "application/json")
	ctx2 := context.WithValue(req2.Context(), "subject", "user456")
	req2 = req2.WithContext(ctx2)

	key2 := suite.middleware.generateCacheKey(req2)
	suite.Require().NotEqual(key, key2, "Different users should have different cache keys")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheKeyGeneration_VaryHeaders() {
	suite.T().Parallel()

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("Accept-Language", "en-US")

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("Accept-Language", "fr-FR")

	key1 := suite.middleware.generateCacheKey(req1)
	key2 := suite.middleware.generateCacheKey(req2)

	suite.Require().NotEqual(key1, key2, "Different Accept-Language should result in different cache keys")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheMiss_ExecutesHandler() {
	suite.T().Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})

	middleware := suite.middleware.Middleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	suite.Require().True(handlerCalled, "Handler should be called on cache miss")
	suite.Require().Equal(http.StatusOK, rec.Code)
	suite.Require().Equal(`{"status":"success"}`, rec.Body.String())
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheHit_SkipsHandler() {
	suite.T().Parallel()

	// Skip this test if KeyDB is not available
	if err := suite.cacheClient.Ping(context.Background()); err != nil {
		suite.T().Skip("KeyDB not available")
	}

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","count":` + string(rune(handlerCallCount+'0')) + `}`))
	})

	middleware := suite.middleware.Middleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com&test=cache-hit", nil)

	// First request - should call handler and cache
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req)
	suite.Require().Equal(1, handlerCallCount, "First request should call handler")
	suite.Require().Equal(http.StatusOK, rec1.Code)

	// Second request - should use cache and NOT call handler
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com&test=cache-hit", nil)
	middleware.ServeHTTP(rec2, req2)
	suite.Require().Equal(1, handlerCallCount, "Second request should NOT call handler (cache hit)")
	suite.Require().Equal(http.StatusOK, rec2.Code)
	suite.Require().Equal(rec1.Body.String(), rec2.Body.String(), "Cached response should match original")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestNonCacheableMethod_POST() {
	suite.T().Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	})

	middleware := suite.middleware.Middleware(handler)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analyze", bytes.NewBufferString(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	suite.Require().Equal(http.StatusCreated, rec.Code)
	// POST requests should not be cached
}

func (suite *ResponseCacheMiddlewareTestSuite) TestNonCacheableStatus_500() {
	suite.T().Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	})

	middleware := suite.middleware.Middleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?url=https://example.com", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// 500 responses should not be cached
	suite.Require().Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *ResponseCacheMiddlewareTestSuite) TestSkipPaths() {
	suite.T().Parallel()

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	middleware := suite.middleware.Middleware(handler)

	// Request to /health should always execute handler
	req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)
	suite.Require().Equal(1, handlerCallCount)

	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)
	suite.Require().Equal(2, handlerCallCount, "Skip paths should not be cached")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestETagSupport_Generation() {
	suite.T().Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	})

	middleware := suite.middleware.Middleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/data?id=123", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	etag := rec.Header().Get("ETag")
	suite.Require().NotEmpty(etag, "ETag should be generated when EnableETag is true")
	suite.Require().Contains(etag, `"`, "ETag should be quoted")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestETagSupport_IfNoneMatch() {
	suite.T().Parallel()

	// Skip if KeyDB not available
	if err := suite.cacheClient.Ping(context.Background()); err != nil {
		suite.T().Skip("KeyDB not available")
	}

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	})

	middleware := suite.middleware.Middleware(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/data?id=etag-test", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	suite.Require().NotEmpty(etag)

	// Second request with If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/data?id=etag-test", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)

	suite.Require().Equal(http.StatusNotModified, rec2.Code, "Should return 304 when ETag matches")
	suite.Require().Empty(rec2.Body.String(), "304 response should have no body")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestSensitiveHeadersExcluded() {
	suite.T().Parallel()

	// Skip if KeyDB not available
	if err := suite.cacheClient.Ping(context.Background()); err != nil {
		suite.T().Skip("KeyDB not available")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "session=secret123")
		w.Header().Set("Authorization", "Bearer token123")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	})

	middleware := suite.middleware.Middleware(handler)

	// First request - cache the response
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/sensitive?test=1", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)

	// Second request - should get the cached response
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/sensitive?test=1", nil)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)

	// Sensitive headers should not be in the cached response
	suite.Require().Empty(rec2.Header().Get("Set-Cookie"), "Set-Cookie should not be cached")
	suite.Require().Empty(rec2.Header().Get("Authorization"), "Authorization should not be cached")
	suite.Require().NotEmpty(rec2.Header().Get("Content-Type"), "Content-Type should be cached")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestMaxCacheSize_Enforcement() {
	suite.T().Parallel()

	// Create middleware with a very small max cache size
	smallConfig := suite.config
	smallConfig.MaxCacheSize = 10 // Only 10 bytes
	smallMiddleware := NewResponseCacheMiddleware(suite.cacheClient, suite.logger, smallConfig)

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.WriteHeader(http.StatusOK)
		// Response larger than 10 bytes
		_, _ = w.Write([]byte(`{"data":"this is a very long response that exceeds the max cache size"}`))
	})

	middleware := smallMiddleware.Middleware(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/large", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)
	suite.Require().Equal(1, handlerCallCount)

	// Second request - should call handler again (not cached due to size)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/large", nil)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)
	suite.Require().Equal(2, handlerCallCount, "Large responses should not be cached")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCacheControl_NoCache() {
	suite.T().Parallel()

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	})

	middleware := suite.middleware.Middleware(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/no-cache", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)
	suite.Require().Equal(1, handlerCallCount)

	// Second request - should call handler again (not cached due to Cache-Control: no-cache)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/no-cache", nil)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)
	suite.Require().Equal(2, handlerCallCount, "Responses with Cache-Control: no-cache should not be cached")
}

func (suite *ResponseCacheMiddlewareTestSuite) TestCachedResponse_Serialization() {
	suite.T().Parallel()

	response := &CachedResponse{
		StatusCode: http.StatusOK,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Custom":     {"value1", "value2"},
		},
		Body:      []byte(`{"test":"data"}`),
		ETag:      `"abc123"`,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(response)
	suite.Require().NoError(err)

	var decoded CachedResponse
	err = json.Unmarshal(data, &decoded)
	suite.Require().NoError(err)

	suite.Require().Equal(response.StatusCode, decoded.StatusCode)
	suite.Require().Equal(response.Headers, decoded.Headers)
	suite.Require().Equal(response.Body, decoded.Body)
	suite.Require().Equal(response.ETag, decoded.ETag)
}

func (suite *ResponseCacheMiddlewareTestSuite) TestDisabledCache() {
	suite.T().Parallel()

	disabledConfig := suite.config
	disabledConfig.Enabled = false
	disabledMiddleware := NewResponseCacheMiddleware(suite.cacheClient, suite.logger, disabledConfig)

	handlerCallCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCallCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	})

	middleware := disabledMiddleware.Middleware(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec1 := httptest.NewRecorder()
	middleware.ServeHTTP(rec1, req1)
	suite.Require().Equal(1, handlerCallCount)

	// Second request - should call handler again (caching disabled)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec2 := httptest.NewRecorder()
	middleware.ServeHTTP(rec2, req2)
	suite.Require().Equal(2, handlerCallCount, "Handler should be called every time when caching is disabled")
}
