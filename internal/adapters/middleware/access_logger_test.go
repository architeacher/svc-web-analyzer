package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessLogger_Middleware(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		method        string
		path          string
		query         string
		statusCode    int
		expectedLevel string
		skipAccessLog bool
		requestID     string
		traceID       string
		referer       string
		shouldLog     bool
	}{
		{
			name:          "successful request logs info level",
			method:        "GET",
			path:          "/v1/analyze",
			query:         "url=https://example.com",
			statusCode:    http.StatusOK,
			expectedLevel: "info",
			shouldLog:     true,
		},
		{
			name:          "client error logs warn level",
			method:        "POST",
			path:          "/v1/analyze",
			statusCode:    http.StatusBadRequest,
			expectedLevel: "warn",
			shouldLog:     true,
		},
		{
			name:          "server error logs error level",
			method:        "GET",
			path:          "/v1/analysis/123",
			statusCode:    http.StatusInternalServerError,
			expectedLevel: "error",
			shouldLog:     true,
		},
		{
			name:          "skipped access log does not log",
			method:        "GET",
			path:          "/v1/health",
			statusCode:    http.StatusOK,
			skipAccessLog: true,
			shouldLog:     false,
		},
		{
			name:          "includes request_id when present",
			method:        "GET",
			path:          "/v1/analyze",
			statusCode:    http.StatusOK,
			requestID:     "req_12345",
			expectedLevel: "info",
			shouldLog:     true,
		},
		{
			name:          "includes trace_id when present",
			method:        "GET",
			path:          "/v1/analyze",
			statusCode:    http.StatusOK,
			traceID:       "trace_67890",
			expectedLevel: "info",
			shouldLog:     true,
		},
		{
			name:          "includes referer when present",
			method:        "GET",
			path:          "/v1/analyze",
			statusCode:    http.StatusOK,
			referer:       "https://example.com",
			expectedLevel: "info",
			shouldLog:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := zerolog.New(&buf)

			accessLogger := NewAccessLogger(logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte("test response"))
			})

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.query != "" {
				req.URL.RawQuery = tc.query
			}

			if tc.skipAccessLog {
				ctx := context.WithValue(req.Context(), skipAccessLogKey, true)
				req = req.WithContext(ctx)
			}

			if tc.requestID != "" {
				req.Header.Set("X-Request-ID", tc.requestID)
			}

			if tc.traceID != "" {
				req.Header.Set("X-Trace-ID", tc.traceID)
			}

			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}

			rec := httptest.NewRecorder()

			middleware := accessLogger.Middleware(handler)
			middleware.ServeHTTP(rec, req)

			if !tc.shouldLog {
				assert.Empty(t, buf.String(), "expected no log output")

				return
			}

			var logEntry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			require.NoError(t, err, "log output should be valid JSON: %s", buf.String())

			assert.Equal(t, tc.expectedLevel, logEntry["level"], "level mismatch")
			assert.Equal(t, "http_access", logEntry["component"], "component mismatch")
			assert.Equal(t, tc.method, logEntry["method"], "method mismatch")
			assert.Equal(t, tc.path, logEntry["path"], "path mismatch")
			assert.Equal(t, tc.query, logEntry["query"], "query mismatch")
			assert.Equal(t, float64(tc.statusCode), logEntry["status_code"], "status_code mismatch")
			assert.Contains(t, logEntry, "duration", "duration field missing")
			assert.Contains(t, logEntry, "duration_ms", "duration_ms field missing")
			assert.Contains(t, logEntry, "response_size_bytes", "response_size_bytes field missing")
			assert.GreaterOrEqual(t, logEntry["response_size_bytes"], float64(0), "response_size_bytes should be >= 0")

			if tc.requestID != "" {
				assert.Equal(t, tc.requestID, logEntry["request_id"])
			}

			if tc.traceID != "" {
				assert.Equal(t, tc.traceID, logEntry["trace_id"])
			}

			if tc.referer != "" {
				assert.Equal(t, tc.referer, logEntry["referer"])
			}
		})
	}
}

func TestAccessLogger_ResponseWriter(t *testing.T) {
	t.Parallel()

	t.Run("captures status code", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		rw.WriteHeader(http.StatusNotFound)

		assert.Equal(t, http.StatusNotFound, rw.statusCode)
	})

	t.Run("captures bytes written", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		testData := []byte("test response data")
		n, err := rw.Write(testData)

		require.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, int64(len(testData)), rw.bytesWritten)
	})

	t.Run("defaults to 200 OK when WriteHeader not called", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		_, _ = rw.Write([]byte("test"))

		assert.Equal(t, http.StatusOK, rw.statusCode)
	})
}
