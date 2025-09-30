package ports

import "context"

// BackgroundProcessor defines the interface for background task processing
type BackgroundProcessor interface {
	Start(ctx context.Context) error
}
