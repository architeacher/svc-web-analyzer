package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"aidanwoods.dev/go-paseto/v2"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

const (
	vaultKeyField        = "public_key"
	vaultKeyVersionField = "version"
)

type (
	// PasetoKeyService manages PASETO public keys with Vault integration and caching.
	PasetoKeyService struct {
		config          config.AuthConfig
		secretsRepo     ports.SecretsRepository
		logger          Logger
		cachedKey       *paseto.V4AsymmetricPublicKey
		cachedKeyExpiry time.Time
		mu              sync.RWMutex
	}
)

// NewPasetoKeyService creates a new instance of PasetoKeyService.
func NewPasetoKeyService(
	cfg config.AuthConfig,
	secretsRepo ports.SecretsRepository,
	logger Logger,
) *PasetoKeyService {
	return &PasetoKeyService{
		config:      cfg,
		secretsRepo: secretsRepo,
		logger:      logger,
	}
}

// GetPublicKey retrieves the PASETO public key, using cache when valid or loading from Vault.
func (s *PasetoKeyService) GetPublicKey(ctx context.Context) (paseto.V4AsymmetricPublicKey, error) {
	// If Vault keys are disabled, use the fallback key
	if !s.config.UseVaultKeys {
		s.logger.Warn().Msg("Vault keys disabled, using fallback key")

		return s.loadFallbackKey()
	}

	// Check if the cached key is still valid
	s.mu.RLock()
	if s.cachedKey != nil && time.Now().Before(s.cachedKeyExpiry) {
		key := *s.cachedKey
		s.mu.RUnlock()
		s.logger.Debug().Msg("using cached PASETO public key")

		return key, nil
	}
	s.mu.RUnlock()

	// Load key from Vault
	return s.loadKeyFromVault(ctx)
}

// RefreshKey forces a refresh of the cached key from Vault.
func (s *PasetoKeyService) RefreshKey(ctx context.Context) error {
	s.logger.Info().Msg("forcing PASETO key refresh from Vault")

	_, err := s.loadKeyFromVault(ctx)

	return err
}

// loadKeyFromVault loads the PASETO public key from Vault and updates the cache.
func (s *PasetoKeyService) loadKeyFromVault(ctx context.Context) (paseto.V4AsymmetricPublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cachedKey != nil && time.Now().Before(s.cachedKeyExpiry) {
		key := *s.cachedKey
		s.logger.Debug().Msg("key was loaded by another goroutine, using cached version")

		return key, nil
	}

	s.logger.Info().
		Str("path", s.config.PasetoKeyPath).
		Msg("Loading PASETO public key from Vault")

	secret, err := s.secretsRepo.GetSecrets(ctx, s.config.PasetoKeyPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to load PASETO key from Vault, falling back to hardcoded key")

		return s.loadFallbackKey()
	}

	if secret == nil || secret.Data == nil {
		err := fmt.Errorf("vault secret data is nil")
		s.logger.Error().Err(err).Msg("invalid Vault response, falling back to hardcoded key")

		return s.loadFallbackKey()
	}

	// Vault KV v2 wraps data in a "data" field
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		err := fmt.Errorf("vault secret data field is not a map")
		s.logger.Error().Err(err).Msg("invalid Vault response format, falling back to hardcoded key")

		return s.loadFallbackKey()
	}

	publicKeyHex, ok := data[vaultKeyField].(string)
	if !ok || publicKeyHex == "" {
		err := fmt.Errorf("public_key field not found or empty in Vault")
		s.logger.Error().Err(err).Msg("Invalid key in Vault, falling back to hardcoded key")

		return s.loadFallbackKey()
	}

	keyVersion, _ := data[vaultKeyVersionField].(string)
	if keyVersion == "" {
		keyVersion = "unknown"
	}

	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromHex(publicKeyHex)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("key_version", keyVersion).
			Msg("failed to parse PASETO public key from Vault, falling back to hardcoded key")

		return s.loadFallbackKey()
	}

	// Update cache
	s.cachedKey = &publicKey
	s.cachedKeyExpiry = time.Now().Add(s.config.KeyCacheTTL)

	s.logger.Info().
		Str("key_version", keyVersion).
		Str("cache_ttl", s.config.KeyCacheTTL.String()).
		Time("expiry", s.cachedKeyExpiry).
		Msg("Successfully loaded and cached PASETO public key from Vault")

	return publicKey, nil
}

// loadFallbackKey loads the fallback PASETO public key from the configuration.
func (s *PasetoKeyService) loadFallbackKey() (paseto.V4AsymmetricPublicKey, error) {
	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromHex(s.config.FallbackKeyHex)
	if err != nil {
		s.logger.Fatal().
			Err(err).
			Msg("failed to create PASETO public key from fallback hex")

		return paseto.V4AsymmetricPublicKey{}, fmt.Errorf("failed to create fallback PASETO public key: %w", err)
	}

	s.logger.Warn().Msg("using fallback PASETO public key - this should not be used in production")

	return publicKey, nil
}
