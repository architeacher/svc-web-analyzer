package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetrics struct {
	recordedMethod       string
	recordedPath         string
	recordedStatusCode   int
	recordedDuration     time.Duration
	recordedRequestSize  int64
	recordedResponseSize int64
}

func (m *mockMetrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	m.recordedMethod = method
	m.recordedPath = path
	m.recordedStatusCode = statusCode
	m.recordedDuration = duration
	m.recordedRequestSize = requestSize
	m.recordedResponseSize = responseSize
}

func (m *mockMetrics) RecordAnalysisRequest(ctx context.Context, duration time.Duration, success bool, errorType string) {
}

func (m *mockMetrics) RecordOutboxEvent(ctx context.Context, success bool, priority string) {
}

func (m *mockMetrics) RecordLinkCheck(ctx context.Context, success bool, linkType string) {
}

func (m *mockMetrics) RecordFetchTime(ctx context.Context, duration time.Duration) {
}

func (m *mockMetrics) RecordProcessingTime(ctx context.Context, duration time.Duration) {
}

func (m *mockMetrics) IncreaseQueueDepth(ctx context.Context, delta int64) {
}

func (m *mockMetrics) DecreaseQueueDepth(ctx context.Context, delta int64) {
}

func (m *mockMetrics) Handler() http.Handler {
	return http.NotFoundHandler()
}

func (m *mockMetrics) Shutdown(ctx context.Context) error {
	return nil
}

func TestMetricsMiddleware_Middleware(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		method             string
		path               string
		statusCode         int
		responseBody       string
		expectedStatusCode int
	}{
		{
			name:               "records successful GET request",
			method:             "GET",
			path:               "/v1/analyze",
			statusCode:         http.StatusOK,
			responseBody:       "success response",
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "records POST request with created status",
			method:             "POST",
			path:               "/v1/analyze",
			statusCode:         http.StatusCreated,
			responseBody:       "{\"id\":\"123\"}",
			expectedStatusCode: http.StatusCreated,
		},
		{
			name:               "records bad request error",
			method:             "POST",
			path:               "/v1/analyze",
			statusCode:         http.StatusBadRequest,
			responseBody:       "bad request",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "records internal server error",
			method:             "GET",
			path:               "/v1/analysis/123",
			statusCode:         http.StatusInternalServerError,
			responseBody:       "error",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name:               "records not found error",
			method:             "GET",
			path:               "/v1/analysis/nonexistent",
			statusCode:         http.StatusNotFound,
			responseBody:       "not found",
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockMetrics{}
			metricsMiddleware := NewMetricsMiddleware(mock)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			})

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			middleware := metricsMiddleware.Middleware(handler)
			middleware.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatusCode, rec.Code, "response status code mismatch")
			assert.Equal(t, tc.method, mock.recordedMethod, "recorded method mismatch")
			assert.Equal(t, tc.path, mock.recordedPath, "recorded path mismatch")
			assert.Equal(t, tc.statusCode, mock.recordedStatusCode, "recorded status code mismatch")
			assert.Greater(t, mock.recordedDuration, time.Duration(0), "recorded duration should be positive")
			assert.Equal(t, int64(len(tc.responseBody)), mock.recordedResponseSize, "recorded response size mismatch")
		})
	}
}

func TestMetricsMiddleware_ResponseWriter(t *testing.T) {
	t.Parallel()

	t.Run("captures status code", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newMetricsResponseWriter(rec)

		rw.WriteHeader(http.StatusNotFound)

		assert.Equal(t, http.StatusNotFound, rw.statusCode)
	})

	t.Run("captures bytes written", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newMetricsResponseWriter(rec)

		testData := []byte("test response data")
		n, err := rw.Write(testData)

		require.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, int64(len(testData)), rw.bytesWritten)
	})

	t.Run("defaults to 200 OK when WriteHeader not called", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newMetricsResponseWriter(rec)

		_, _ = rw.Write([]byte("test"))

		assert.Equal(t, http.StatusOK, rw.statusCode)
	})

	t.Run("accumulates bytes written across multiple writes", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := newMetricsResponseWriter(rec)

		firstWrite := []byte("first ")
		secondWrite := []byte("second")

		n1, err1 := rw.Write(firstWrite)
		require.NoError(t, err1)

		n2, err2 := rw.Write(secondWrite)
		require.NoError(t, err2)

		totalBytes := int64(n1 + n2)
		assert.Equal(t, totalBytes, rw.bytesWritten)
		assert.Equal(t, int64(len(firstWrite)+len(secondWrite)), rw.bytesWritten)
	})
}
