package ports

import (
	"context"

	"github.com/hashicorp/vault/api"
)

type (
	SecretsRepository interface {
		SetToken(v string)
		GetSecrets(ctx context.Context, path string) (*api.Secret, error)
		WriteWithContext(ctx context.Context, path string, data map[string]interface{}) (*api.Secret, error)
	}
)
