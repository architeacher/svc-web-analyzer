package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const (
	skipAccessLogKey = "skip_access_log"
)

type AccessLogger struct {
	logger zerolog.Logger
}

func NewAccessLogger(logger zerolog.Logger) *AccessLogger {
	return &AccessLogger{
		logger: logger.With().Str("component", "http_access").Logger(),
	}
}

func newResponseWriter(w http.ResponseWriter) *FlushableResponseWriter {
	return NewFlushableResponseWriter(w)
}

func (a *AccessLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skip, ok := r.Context().Value(skipAccessLogKey).(bool); ok && skip {
			next.ServeHTTP(w, r)

			return
		}

		startTime := time.Now()
		wrapped := newResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(startTime)

		logEvent := a.logger.Info()

		logEvent.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("proto", r.Proto).
			Str("host", r.Host).
			Int("status_code", wrapped.StatusCode()).
			Int64("response_size_bytes", wrapped.BytesWritten()).
			Dur("duration", duration).
			Float64("duration_ms", float64(duration.Milliseconds()))

		if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			logEvent.Str("request_id", requestID)
		}

		if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
			logEvent.Str("trace_id", traceID)
		}

		if referer := r.Referer(); referer != "" {
			logEvent.Str("referer", referer)
		}

		if wrapped.StatusCode() >= http.StatusInternalServerError {
			logEvent = a.logger.Error().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("proto", r.Proto).
				Str("host", r.Host).
				Int("status_code", wrapped.StatusCode()).
				Int64("response_size_bytes", wrapped.BytesWritten()).
				Dur("duration", duration).
				Float64("duration_ms", float64(duration.Milliseconds()))

			if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
				logEvent.Str("request_id", requestID)
			}

			if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
				logEvent.Str("trace_id", traceID)
			}
		} else if wrapped.StatusCode() >= http.StatusBadRequest {
			logEvent = a.logger.Warn().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("proto", r.Proto).
				Str("host", r.Host).
				Int("status_code", wrapped.StatusCode()).
				Int64("response_size_bytes", wrapped.BytesWritten()).
				Dur("duration", duration).
				Float64("duration_ms", float64(duration.Milliseconds()))

			if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
				logEvent.Str("request_id", requestID)
			}

			if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
				logEvent.Str("trace_id", traceID)
			}
		}

		logEvent.Msg("HTTP request completed")
	})
}
