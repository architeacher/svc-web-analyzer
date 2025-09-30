package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

			mock := new(mocks.FakeMetricsClient)
			mock.RecordHTTPRequestCalls(func(ctx context.Context, method, path string, statusCode uint, duration time.Duration, requestSize, responseSize uint64) {

			})
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
			_, recordedMethod, recordedPath, recordedStatus, recordedDuration, _, recordedResponseSize := mock.RecordHTTPRequestArgsForCall(0)
			assert.Equal(t, tc.method, recordedMethod, "recorded method mismatch")
			assert.Equal(t, tc.path, recordedPath, "recorded path mismatch")
			assert.Equal(t, uint(tc.statusCode), recordedStatus, "recorded status code mismatch")
			assert.Greater(t, recordedDuration, time.Duration(0), "recorded duration should be positive")
			assert.Equal(t, uint64(len(tc.responseBody)), recordedResponseSize, "recorded response size mismatch")
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
