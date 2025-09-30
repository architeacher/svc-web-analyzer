//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

type Setter interface {
	Set(context.Context, *domain.Analysis) error
}

//counterfeiter:generate -o ../mocks/cache_repository.go . CacheRepository
type CacheRepository interface {
	Finder
	Setter
	Deleter
}
