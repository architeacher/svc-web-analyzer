package domain

import (
	"errors"
	"fmt"
)

var (
	ErrAnalysisNotFound       = errors.New("analysis not found")
	ErrInvalidURL             = errors.New("invalid URL")
	ErrURLNotReachable        = errors.New("URL not reachable")
	ErrTimeoutExceeded        = errors.New("analysis timeout exceeded")
	ErrInvalidRequest         = errors.New("invalid request")
	ErrInternalServerError    = errors.New("internal server error")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
	ErrCircuitBreakerOpen     = errors.New("circuit breaker open")
	ErrCacheUnavailable       = errors.New("cache service unavailable")
	ErrConcurrentModification = errors.New("concurrent modification detected")
)

type (
	DomainError struct {
		Code       string
		Message    string
		StatusCode int
		Cause      error
		Details    map[string]any
	}

	OptimisticLockError struct {
		Expected int
		Actual   int
	}

	InvalidStateTransitionError struct {
		From string
		To   string
	}

	MaxRetriesExceededError struct {
		EventID    string
		RetryCount int
		MaxRetries int
	}
)

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

func NewConcurrentModificationError(resourceID string, expectedVersion, actualVersion int) *DomainError {
	return NewDomainError(
		"CONCURRENT_MODIFICATION",
		fmt.Sprintf("Resource %s was modified by another process", resourceID),
		409,
		ErrConcurrentModification,
	).WithDetails("resource_id", resourceID).
		WithDetails("expected_version", expectedVersion).
		WithDetails("actual_version", actualVersion)
}

func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock failed: expected version %d, got %d", e.Expected, e.Actual)
}

func (e *InvalidStateTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition from %s to %s", e.From, e.To)
}

func (e *MaxRetriesExceededError) Error() string {
	return fmt.Sprintf("max retries exceeded for event %s: %d/%d", e.EventID, e.RetryCount, e.MaxRetries)
}
