package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
)

const (
	responseCacheKeyPrefix = "svc-web-analyzer:response:v1:"
)

var (
	excludedResponseHeaders = []string{
		"Set-Cookie",
		"Authorization",
		"X-PASETO-Token",
		"WWW-Authenticate",
		"Proxy-Authenticate",
		"Proxy-Authorization",
	}
)

type (
	ResponseCacheConfig struct {
		Enabled             bool
		DefaultTTL          time.Duration
		MaxCacheSize        int64
		CacheableStatuses   []int
		CacheableMethods    []string
		SkipPaths           []string
		RespectCacheControl bool
		EnableETag          bool
		VaryHeaders         []string
	}

	CachedResponse struct {
		StatusCode int                 `json:"status_code"`
		Headers    map[string][]string `json:"headers"`
		Body       []byte              `json:"body"`
		ETag       string              `json:"etag"`
		CreatedAt  time.Time           `json:"created_at"`
	}

	ResponseCacheMiddleware struct {
		cache  *infrastructure.KeydbClient
		logger appLogger.Logger
		config ResponseCacheConfig
	}

	bufferedResponseWriter struct {
		http.ResponseWriter
		statusCode     int
		buffer         *bytes.Buffer
		bytesWritten   int64
		headerWritten  bool
		capturedHeader http.Header
	}
)

func NewResponseCacheMiddleware(
	cache *infrastructure.KeydbClient,
	logger appLogger.Logger,
	config ResponseCacheConfig,
) *ResponseCacheMiddleware {
	return &ResponseCacheMiddleware{
		cache:  cache,
		logger: logger,
		config: config,
	}
}

func (m *ResponseCacheMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if caching is disabled
		if !m.config.Enabled {
			next.ServeHTTP(w, r)

			return
		}

		// Only cache configured methods
		if !m.isCacheableMethod(r.Method) {
			next.ServeHTTP(w, r)

			return
		}

		// Skip configured paths
		if m.isSkipPath(r.URL.Path) {
			next.ServeHTTP(w, r)

			return
		}

		cacheKey := m.generateCacheKey(r)

		// Check cache for existing response
		if cachedResp, err := m.getCachedResponse(r, cacheKey); err == nil {
			m.logger.Debug().
				Str("cache_key", cacheKey).
				Str("path", r.URL.Path).
				Msg("cache hit")

			m.writeCachedResponse(w, r, cachedResp)

			return
		}

		// Cache miss - create buffering response writer
		buffered := newBufferedResponseWriter(w)

		next.ServeHTTP(buffered, r)

		// Generate ETag and set it on headers before writing (if enabled and cacheable)
		if m.config.EnableETag && m.isCacheableStatus(buffered.statusCode) {
			etag := m.generateETag(buffered.buffer.Bytes())
			buffered.capturedHeader.Set("ETag", etag)
		}

		// Flush buffered response to actual writer
		buffered.Flush()

		// Cache response if it's cacheable
		m.cacheResponse(r, cacheKey, buffered)
	})
}

func (m *ResponseCacheMiddleware) isCacheableMethod(method string) bool {
	for _, allowed := range m.config.CacheableMethods {
		if strings.EqualFold(method, allowed) {
			return true
		}
	}

	return false
}

func (m *ResponseCacheMiddleware) isSkipPath(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

func (m *ResponseCacheMiddleware) isCacheableStatus(status int) bool {
	for _, allowed := range m.config.CacheableStatuses {
		if status == allowed {
			return true
		}
	}

	return false
}

func (m *ResponseCacheMiddleware) generateCacheKey(r *http.Request) string {
	var keyParts []string

	// Method
	keyParts = append(keyParts, r.Method)

	// Path
	keyParts = append(keyParts, r.URL.Path)

	// Query parameters (sorted for consistency)
	if r.URL.RawQuery != "" {
		query := r.URL.Query()
		var queryKeys []string
		for key := range query {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)

		var queryParts []string
		for _, key := range queryKeys {
			values := query[key]
			sort.Strings(values)
			for _, value := range values {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, value))
			}
		}
		keyParts = append(keyParts, strings.Join(queryParts, "&"))
	}

	// Vary headers (if configured)
	for _, header := range m.config.VaryHeaders {
		if value := r.Header.Get(header); value != "" {
			keyParts = append(keyParts, fmt.Sprintf("%s:%s", header, value))
		}
	}

	// Auth context (if present)
	if subject := r.Context().Value("subject"); subject != nil {
		keyParts = append(keyParts, fmt.Sprintf("subject:%v", subject))
	}

	// Generate SHA-256 hash of key parts
	keyData := strings.Join(keyParts, "|")
	hash := sha256.Sum256([]byte(keyData))

	return fmt.Sprintf("%s%x", responseCacheKeyPrefix, hash)
}

func (m *ResponseCacheMiddleware) getCachedResponse(r *http.Request, cacheKey string) (*CachedResponse, error) {
	data, err := m.cache.Get(r.Context(), cacheKey)
	if err != nil {
		return nil, err
	}

	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		m.logger.Error().
			Err(err).
			Str("cache_key", cacheKey).
			Msg("failed to unmarshal cached response")

		return nil, err
	}

	return &cached, nil
}

func (m *ResponseCacheMiddleware) writeCachedResponse(w http.ResponseWriter, r *http.Request, cached *CachedResponse) {
	// Check if client supports conditional requests (ETag)
	if m.config.EnableETag && cached.ETag != "" {
		if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch == cached.ETag {
			w.WriteHeader(http.StatusNotModified)

			return
		}
	}

	for key, values := range cached.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set ETag if enabled
	if m.config.EnableETag && cached.ETag != "" {
		w.Header().Set("ETag", cached.ETag)
	}

	// Add cache header to indicate response was served from cache
	w.Header().Set("X-Cache", "HIT")

	// Write status code
	w.WriteHeader(cached.StatusCode)

	// Write body
	if r.Method != http.MethodHead {
		_, _ = w.Write(cached.Body)
	}
}

func (m *ResponseCacheMiddleware) cacheResponse(r *http.Request, cacheKey string, buffered *bufferedResponseWriter) {
	// Only cache successful responses
	if !m.isCacheableStatus(buffered.statusCode) {
		m.logger.Debug().
			Int("status_code", buffered.statusCode).
			Str("path", r.URL.Path).
			Msg("response not cacheable (status code)")

		return
	}

	// Check response size
	if buffered.bytesWritten > m.config.MaxCacheSize {
		m.logger.Debug().
			Int64("size", buffered.bytesWritten).
			Int64("max_size", m.config.MaxCacheSize).
			Str("path", r.URL.Path).
			Msg("response too large to cache")

		return
	}

	// Check Cache-Control header if configured
	if m.config.RespectCacheControl {
		cacheControl := buffered.capturedHeader.Get("Cache-Control")
		if m.shouldSkipCacheControl(cacheControl) {
			m.logger.Debug().
				Str("cache_control", cacheControl).
				Str("path", r.URL.Path).
				Msg("response not cacheable (Cache-Control)")

			return
		}
	}

	// Filter sensitive headers
	headers := m.filterHeaders(buffered.capturedHeader)

	// Get ETag from headers if it was set
	var etag string
	if m.config.EnableETag {
		etag = buffered.capturedHeader.Get("ETag")
	}

	cached := &CachedResponse{
		StatusCode: buffered.statusCode,
		Headers:    headers,
		Body:       buffered.buffer.Bytes(),
		ETag:       etag,
		CreatedAt:  time.Now().UTC(),
	}

	data, err := json.Marshal(cached)
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("cache_key", cacheKey).
			Msg("failed to marshal response for caching")

		return
	}

	ttl := m.config.DefaultTTL
	if customTTL := m.parseCacheControlMaxAge(buffered.capturedHeader.Get("Cache-Control")); customTTL > 0 {
		ttl = customTTL
	}

	if err := m.cache.Set(r.Context(), cacheKey, data, ttl); err != nil {
		m.logger.Error().
			Err(err).
			Str("cache_key", cacheKey).
			Msg("failed to cache response")

		return
	}

	m.logger.Debug().
		Str("cache_key", cacheKey).
		Str("path", r.URL.Path).
		Int64("size", buffered.bytesWritten).
		Str("ttl", ttl.String()).
		Msg("response cached")
}

func (m *ResponseCacheMiddleware) filterHeaders(headers http.Header) map[string][]string {
	filtered := make(map[string][]string)

	for key, values := range headers {
		// Skip excluded headers
		if m.isExcludedHeader(key) {
			continue
		}

		filtered[key] = values
	}

	return filtered
}

func (m *ResponseCacheMiddleware) isExcludedHeader(header string) bool {
	headerLower := strings.ToLower(header)
	for _, excluded := range excludedResponseHeaders {
		if strings.ToLower(excluded) == headerLower {
			return true
		}
	}

	return false
}

func (m *ResponseCacheMiddleware) shouldSkipCacheControl(cacheControl string) bool {
	if cacheControl == "" {
		return false
	}

	directives := strings.Split(strings.ToLower(cacheControl), ",")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive == "no-cache" || directive == "no-store" || directive == "private" {
			return true
		}
	}

	return false
}

func (m *ResponseCacheMiddleware) parseCacheControlMaxAge(cacheControl string) time.Duration {
	if cacheControl == "" {
		return 0
	}

	directives := strings.Split(cacheControl, ",")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "max-age=") {
			maxAgeStr := strings.TrimPrefix(directive, "max-age=")
			var maxAge int
			if _, err := fmt.Sscanf(maxAgeStr, "%d", &maxAge); err == nil && maxAge > 0 {
				return time.Duration(maxAge) * time.Second
			}
		}
	}

	return 0
}

func (m *ResponseCacheMiddleware) generateETag(body []byte) string {
	hash := sha256.Sum256(body)

	return fmt.Sprintf(`"%x"`, hash[:16])
}

func newBufferedResponseWriter(w http.ResponseWriter) *bufferedResponseWriter {
	buffer := new(bytes.Buffer)
	capturedHeader := make(http.Header)

	return &bufferedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		buffer:         buffer,
		bytesWritten:   0,
		headerWritten:  false,
		capturedHeader: capturedHeader,
	}
}

func (b *bufferedResponseWriter) Header() http.Header {
	return b.capturedHeader
}

func (b *bufferedResponseWriter) WriteHeader(code int) {
	if b.headerWritten {
		return
	}

	b.statusCode = code
	b.headerWritten = true
}

func (b *bufferedResponseWriter) Write(p []byte) (int, error) {
	if !b.headerWritten {
		b.WriteHeader(b.statusCode)
	}

	n, err := b.buffer.Write(p)
	b.bytesWritten += int64(n)

	return n, err
}

func (b *bufferedResponseWriter) Flush() {
	// Copy captured headers to actual response
	for key, values := range b.capturedHeader {
		for _, value := range values {
			b.ResponseWriter.Header().Add(key, value)
		}
	}

	b.ResponseWriter.WriteHeader(b.statusCode)

	if b.buffer.Len() > 0 {
		_, _ = b.ResponseWriter.Write(b.buffer.Bytes())
	}
}
