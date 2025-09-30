//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/hashicorp/vault/api"
)

//counterfeiter:generate -o ../mocks/secrets_repository.go . SecretsRepository

type (
	SecretsRepository interface {
		SetToken(v string)
		GetSecrets(ctx context.Context, path string) (*api.Secret, error)
		WriteWithContext(ctx context.Context, path string, data map[string]interface{}) (*api.Secret, error)
	}
)
