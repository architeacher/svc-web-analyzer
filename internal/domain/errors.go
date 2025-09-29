package domain

import (
	"errors"
	"fmt"
)

var (
	ErrAnalysisNotFound    = errors.New("analysis not found")
	ErrInvalidURL          = errors.New("invalid URL")
	ErrURLNotReachable     = errors.New("URL not reachable")
	ErrTimeoutExceeded     = errors.New("analysis timeout exceeded")
	ErrInvalidRequest      = errors.New("invalid request")
	ErrInternalServerError = errors.New("internal server error")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrCircuitBreakerOpen  = errors.New("circuit breaker open")
	ErrCacheUnavailable    = errors.New("cache service unavailable")
)

type DomainError struct {
	Code       string
	Message    string
	StatusCode int
	Cause      error
	Details    map[string]interface{}
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Cause.Error())
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Cause
}

func NewDomainError(code, message string, statusCode int, cause error) *DomainError {
	return &DomainError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
		Details:    make(map[string]interface{}),
	}
}

func (e *DomainError) WithDetails(key string, value interface{}) *DomainError {
	e.Details[key] = value
	return e
}

func NewURLNotReachableError(url string, statusCode int, cause error) *DomainError {
	return NewDomainError(
		"URL_NOT_REACHABLE",
		fmt.Sprintf("URL %s is not reachable", url),
		statusCode,
		cause,
	).WithDetails("url", url).WithDetails("status_code", statusCode)
}

func NewInvalidURLError(url string, cause error) *DomainError {
	return NewDomainError(
		"INVALID_URL",
		fmt.Sprintf("Invalid URL: %s", url),
		400,
		cause,
	).WithDetails("url", url)
}

func NewTimeoutError(url string, timeout interface{}) *DomainError {
	return NewDomainError(
		"TIMEOUT_EXCEEDED",
		fmt.Sprintf("Analysis timeout exceeded for URL: %s", url),
		408,
		ErrTimeoutExceeded,
	).WithDetails("url", url).WithDetails("timeout", timeout)
}

func NewRateLimitError(message string) *DomainError {
	return NewDomainError(
		"RATE_LIMITING_EXCEEDED",
		message,
		429,
		ErrRateLimitExceeded,
	)
}

func NewUnauthorizedError(message string) *DomainError {
	return NewDomainError(
		"UNAUTHORIZED",
		message,
		401,
		ErrUnauthorized,
	)
}

func NewInternalServerError(message string, cause error) *DomainError {
	return NewDomainError(
		"INTERNAL_SERVER_ERROR",
		message,
		500,
		cause,
	)
}
