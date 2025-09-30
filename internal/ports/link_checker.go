//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

//counterfeiter:generate -o ../mocks/link_checker.go . LinkChecker

type LinkChecker interface {
	CheckAccessibility(ctx context.Context, links []domain.Link) []domain.InaccessibleLink
}
