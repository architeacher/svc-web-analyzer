package adapters

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/hashicorp/vault/api"
)

type (
	VaultRepository struct {
		vaultClient *api.Client
	}
)

func NewVaultRepository(vaultClient *api.Client) ports.SecretsRepository {
	return VaultRepository{
		vaultClient: vaultClient,
	}
}

func (s VaultRepository) SetToken(v string) {
	s.vaultClient.SetToken(v)
}

func (s VaultRepository) GetSecrets(ctx context.Context, path string) (*api.Secret, error) {
	return s.vaultClient.Logical().ReadWithContext(ctx, path)
}

func (s VaultRepository) WriteWithContext(ctx context.Context, path string, data map[string]any) (*api.Secret, error) {
	return s.vaultClient.Logical().WriteWithContext(ctx, path, data)
}
