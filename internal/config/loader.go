package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
)

func Init() (*ServiceConfig, error) {
	cfg := &ServiceConfig{}

	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service configuration: %w", err)
	}

	if len(ServiceVersion) != 0 {
		cfg.AppConfig.ServiceVersion = ServiceVersion
	}

	if len(CommitSHA) != 0 {
		cfg.AppConfig.CommitSHA = CommitSHA
	}

	return cfg, nil
}

func Load(ctx context.Context, secretsRepo ports.SecretsRepository, cfg *ServiceConfig) error {
	if !cfg.SecretStorage.Enabled {
		return fmt.Errorf("secret storage is not enabled")
	}

	if err := loadVaultSecrets(ctx, secretsRepo, cfg); err != nil {
		return fmt.Errorf("failed to load secrets from Vault: %w", err)
	}

	return nil
}

func loadVaultSecrets(ctx context.Context, client ports.SecretsRepository, cfg *ServiceConfig) error {
	if err := authenticateVault(ctx, client, cfg.SecretStorage); err != nil {
		return fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	// Load secrets from the specific Vault path
	secretPath := fmt.Sprintf("apps/data/%s", cfg.SecretStorage.MountPath)
	if err := loadSecretsFromPath(ctx, client, cfg, secretPath); err != nil {
		return fmt.Errorf("failed to load secrets from Vault: %w", err)
	}

	return nil
}

func authenticateVault(ctx context.Context, client ports.SecretsRepository, config SecretStorageConfig) error {
	switch strings.ToLower(config.AuthMethod) {
	case "token":
		if config.Token == "" {
			return fmt.Errorf("token is required for token auth method")
		}
		client.SetToken(config.Token)
		return nil

	case "approle":
		if config.RoleID == "" || config.SecretID == "" {
			return fmt.Errorf("role_id and secret_id are required for approle auth method")
		}

		data := map[string]interface{}{
			"role_id":   config.RoleID,
			"secret_id": config.SecretID,
		}

		resp, err := client.WriteWithContext(ctx, "auth/approle/login", data)
		if err != nil {
			return fmt.Errorf("failed to authenticate via approle: %w", err)
		}

		if resp.Auth == nil {
			return fmt.Errorf("no auth info returned from Vault")
		}

		client.SetToken(resp.Auth.ClientToken)
		return nil

	default:
		return fmt.Errorf("unsupported auth method: %s", config.AuthMethod)
	}
}

func loadSecretsFromPath(ctx context.Context, client ports.SecretsRepository, cfg *ServiceConfig, secretPath string) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.SecretStorage.Timeout)
	defer cancel()

	// Use the full path directly as provided (already includes apps/data/svc-web-analyzer)
	fullPath := secretPath

	var secret *api.Secret
	var err error

	for attempt := 0; attempt <= cfg.SecretStorage.MaxRetries; attempt++ {
		secret, err = client.GetSecrets(ctx, fullPath)
		if err == nil {
			break
		}

		if attempt < cfg.SecretStorage.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to read secret from path %s after %d retries: %w", fullPath, cfg.SecretStorage.MaxRetries, err)
	}

	if secret == nil || secret.Data == nil {
		return nil
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid secret format at path %s", fullPath)
	}

	return applySecretsToConfig(cfg, data)
}

func applySecretsToConfig(cfg *ServiceConfig, data map[string]interface{}) error {
	// Apply secrets directly from flat key-value pairs stored in Vault
	for key, value := range data {
		if strValue, ok := value.(string); ok && strValue != "" {
			if err := applySecretToConfig(cfg, key, strValue); err != nil {
				return fmt.Errorf("failed to apply secrets to config: %w", err)
			}
		}
	}

	return nil
}

func applySecretToConfig(cfg *ServiceConfig, key, value string) error {
	// Set environment variable and update config based on key
	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("failed to set environment variable %s: %w", key, err)
	}

	switch key {
	// Database secrets
	case "POSTGRES_USERNAME":
		cfg.Storage.Username = value
	case "POSTGRES_PASSWORD":
		cfg.Storage.Password = value
	case "POSTGRES_HOST":
		cfg.Storage.Host = value
	case "POSTGRES_DATABASE":
		cfg.Storage.Database = value

	// Cache secrets
	case "KEYDB_PASSWORD":
		cfg.Cache.Password = value
	case "KEYDB_ADDR":
		cfg.Cache.Addr = value

	// Queue secrets
	case "RABBITMQ_USERNAME":
		cfg.Queue.Username = value
	case "RABBITMQ_PASSWORD":
		cfg.Queue.Password = value
	case "RABBITMQ_HOST":
		cfg.Queue.Host = value

	// Auth secrets
	case "AUTH_SECRET_KEY":
		cfg.Auth.SecretKey = value
	}

	return nil
}
