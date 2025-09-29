package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

type LinkChecker interface {
	CheckAccessibility(ctx context.Context, links []domain.Link) []domain.InaccessibleLink
}
