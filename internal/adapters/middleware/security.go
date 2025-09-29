package middleware

import (
	"net/http"
)

type SecurityHeadersMiddleware struct{}

func NewSecurityHeadersMiddleware() SecurityHeadersMiddleware {
	return SecurityHeadersMiddleware{}
}

// Middleware is a middleware that sets a global timeout to the HTTP request.
func (mw SecurityHeadersMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw.addCORSHeaders(w).
			addSecurityHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// addCORSHeaders adds standard CORS headers to all responses.
// That's a requirement for SSE
func (mw SecurityHeadersMiddleware) addCORSHeaders(w http.ResponseWriter) SecurityHeadersMiddleware {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-PASTEO-Token, API-Version")

	return mw
}

// addSecurityHeaders adds standard security headers to all responses.
func (mw SecurityHeadersMiddleware) addSecurityHeaders(w http.ResponseWriter) SecurityHeadersMiddleware {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Content-Security-Policy", "default-src 'self'")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

	return mw
}
