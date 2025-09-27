package ports

import (
	"context"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

type WebPageFetcher interface {
	Fetch(ctx context.Context, url string, timeout time.Duration) (*domain.WebPageContent, error)
}
