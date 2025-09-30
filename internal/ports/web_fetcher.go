//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

//counterfeiter:generate -o ../mocks/web_fetcher.go . WebFetcher

type WebFetcher interface {
	Fetch(ctx context.Context, url string, timeout time.Duration) (*domain.WebPageContent, error)
}
