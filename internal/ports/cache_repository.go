//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

//counterfeiter:generate -o ../mocks/cache_repository.go . CacheRepository
type (
	Setter interface {
		Set(context.Context, *domain.Analysis) error
	}

	CacheRepository interface {
		Finder
		Setter
		Deleter
	}
)
