package infrastructure_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto/v2"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testPublicKeyHex = "01c7981f62c676934dc4acfa7825205ae927960875d09abec497efbe2dba41b7"
)

type (
	MockSecretsRepository struct {
		mock.Mock
	}
)

func (m *MockSecretsRepository) SetToken(v string) {
	m.Called(v)
}

func (m *MockSecretsRepository) GetSecrets(ctx context.Context, path string) (*api.Secret, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*api.Secret), args.Error(1)
}

func (m *MockSecretsRepository) WriteWithContext(ctx context.Context, path string, data map[string]interface{}) (*api.Secret, error) {
	args := m.Called(ctx, path, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*api.Secret), args.Error(1)
}

func createTestLogger() infrastructure.Logger {
	return infrastructure.New(config.LoggingConfig{
		Level:  "error", // Set to error to reduce test noise
		Format: "json",
	})
}

func TestPasetoKeyService_GetPublicKey_VaultDisabled(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   false,
		FallbackKeyHex: testPublicKeyHex,
	}

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	ctx := context.Background()
	key, err := service.GetPublicKey(ctx)

	require.NoError(t, err)
	assert.NotNil(t, key)
}

func TestPasetoKeyService_GetPublicKey_FromVault_Success(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   true,
		PasetoKeyPath:  "secret/data/paseto/public-key",
		KeyCacheTTL:    1 * time.Hour,
		FallbackKeyHex: testPublicKeyHex,
	}

	vaultSecret := &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				"public_key": testPublicKeyHex,
				"version":    "v1",
			},
		},
	}

	ctx := context.Background()
	mockSecretsRepo.On("GetSecrets", ctx, cfg.PasetoKeyPath).Return(vaultSecret, nil)

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	key, err := service.GetPublicKey(ctx)

	require.NoError(t, err)
	assert.NotNil(t, key)
	mockSecretsRepo.AssertExpectations(t)
}

func TestPasetoKeyService_GetPublicKey_FromCache(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   true,
		PasetoKeyPath:  "secret/data/paseto/public-key",
		KeyCacheTTL:    1 * time.Hour,
		FallbackKeyHex: testPublicKeyHex,
	}

	vaultSecret := &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				"public_key": testPublicKeyHex,
				"version":    "v1",
			},
		},
	}

	ctx := context.Background()
	mockSecretsRepo.On("GetSecrets", ctx, cfg.PasetoKeyPath).Return(vaultSecret, nil).Once()

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	// First call - should load from Vault
	key1, err := service.GetPublicKey(ctx)
	require.NoError(t, err)
	assert.NotNil(t, key1)

	// Second call - should use cache
	key2, err := service.GetPublicKey(ctx)
	require.NoError(t, err)
	assert.NotNil(t, key2)

	// Verify Vault was only called once
	mockSecretsRepo.AssertExpectations(t)
}

func TestPasetoKeyService_GetPublicKey_VaultError_FallbackToHardcoded(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   true,
		PasetoKeyPath:  "secret/data/paseto/public-key",
		KeyCacheTTL:    1 * time.Hour,
		FallbackKeyHex: testPublicKeyHex,
	}

	ctx := context.Background()
	mockSecretsRepo.On("GetSecrets", ctx, cfg.PasetoKeyPath).Return(nil, fmt.Errorf("vault connection error"))

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	key, err := service.GetPublicKey(ctx)

	require.NoError(t, err)
	assert.NotNil(t, key)
	mockSecretsRepo.AssertExpectations(t)
}

func TestPasetoKeyService_GetPublicKey_InvalidVaultResponse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		vaultSecret *api.Secret
		description string
	}{
		{
			name:        "nil secret",
			vaultSecret: nil,
			description: "Vault returns nil secret",
		},
		{
			name: "nil data field",
			vaultSecret: &api.Secret{
				Data: nil,
			},
			description: "Vault secret has nil data field",
		},
		{
			name: "missing data wrapper",
			vaultSecret: &api.Secret{
				Data: map[string]interface{}{
					"public_key": testPublicKeyHex,
				},
			},
			description: "Vault secret missing KV v2 data wrapper",
		},
		{
			name: "missing public_key field",
			vaultSecret: &api.Secret{
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"version": "v1",
					},
				},
			},
			description: "Vault secret missing public_key field",
		},
		{
			name: "empty public_key",
			vaultSecret: &api.Secret{
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"public_key": "",
						"version":    "v1",
					},
				},
			},
			description: "Vault secret has empty public_key",
		},
		{
			name: "invalid public_key format",
			vaultSecret: &api.Secret{
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"public_key": "invalid-hex",
						"version":    "v1",
					},
				},
			},
			description: "Vault secret has invalid public_key hex format",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockSecretsRepo := new(MockSecretsRepository)
			logger := createTestLogger()

			cfg := config.AuthConfig{
				UseVaultKeys:   true,
				PasetoKeyPath:  "secret/data/paseto/public-key",
				KeyCacheTTL:    1 * time.Hour,
				FallbackKeyHex: testPublicKeyHex,
			}

			ctx := context.Background()
			mockSecretsRepo.On("GetSecrets", ctx, cfg.PasetoKeyPath).Return(tc.vaultSecret, nil)

			service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

			key, err := service.GetPublicKey(ctx)

			require.NoError(t, err, "Should fall back to hardcoded key without error")
			assert.NotNil(t, key)
		})
	}
}

func TestPasetoKeyService_RefreshKey(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   true,
		PasetoKeyPath:  "secret/data/paseto/public-key",
		KeyCacheTTL:    1 * time.Hour,
		FallbackKeyHex: testPublicKeyHex,
	}

	vaultSecret := &api.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{
				"public_key": testPublicKeyHex,
				"version":    "v1",
			},
		},
	}

	ctx := context.Background()
	mockSecretsRepo.On("GetSecrets", ctx, cfg.PasetoKeyPath).Return(vaultSecret, nil)

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	err := service.RefreshKey(ctx)

	require.NoError(t, err)
	mockSecretsRepo.AssertExpectations(t)
}

func TestPasetoKeyService_LoadFallbackKey_InvalidHex(t *testing.T) {
	t.Parallel()

	mockSecretsRepo := new(MockSecretsRepository)
	logger := createTestLogger()

	cfg := config.AuthConfig{
		UseVaultKeys:   false,
		FallbackKeyHex: "invalid-hex-key",
	}

	service := infrastructure.NewPasetoKeyService(cfg, mockSecretsRepo, logger)

	ctx := context.Background()
	key, err := service.GetPublicKey(ctx)

	assert.Error(t, err)
	assert.Equal(t, paseto.V4AsymmetricPublicKey{}, key)
}
