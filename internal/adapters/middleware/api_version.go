package middleware

import (
	"net/http"
)

type APIVersionMiddleware struct {
	version string
}

func NewAPIVersionMiddleware(version string) APIVersionMiddleware {
	return APIVersionMiddleware{
		version: version,
	}
}

func (mw APIVersionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("API-Version", mw.version)

		next.ServeHTTP(w, r)
	})
}
