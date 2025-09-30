//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import "context"

//counterfeiter:generate -o ../mocks/background_processor.go . BackgroundProcessor

// BackgroundProcessor defines the interface for background task processing
type BackgroundProcessor interface {
	Start(ctx context.Context) error
}
