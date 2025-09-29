package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

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
