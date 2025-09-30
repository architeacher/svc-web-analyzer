//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"aidanwoods.dev/go-paseto/v2"
)

//counterfeiter:generate -o ../mocks/key_service.go . KeyService

// KeyService defines the interface for managing PASETO keys.
type KeyService interface {
	// GetPublicKey retrieves the PASETO public key, using cache when valid or loading from the source.
	GetPublicKey(ctx context.Context) (paseto.V4AsymmetricPublicKey, error)

	// RefreshKey forces a refresh of the cached key.
	RefreshKey(ctx context.Context) error
}
