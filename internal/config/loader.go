package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
)

// Loader handles configuration loading and reloading.
type Loader struct {
	cfg              *ServiceConfig
	secretsRepo      ports.SecretsRepository
	configSignalChan chan os.Signal
	reloadErrors     chan error
	ticker           *time.Ticker
	lastVersion      int
}

// NewLoader creates a new config loader instance.
func NewLoader(cfg *ServiceConfig, secretsRepo ports.SecretsRepository, initialVersion int) *Loader {
	return &Loader{
		cfg:              cfg,
		secretsRepo:      secretsRepo,
		configSignalChan: make(chan os.Signal, 1),
		reloadErrors:     make(chan error, 1),
		lastVersion:      initialVersion,
	}
}

// WatchConfigSignals monitors for SIGHUP (reload) and SIGUSR1 (dump) signals.
// It also starts a background ticker for periodic config reloading if enabled.
// It returns a channel that will receive reload errors for logging by the caller.
func (l *Loader) WatchConfigSignals(ctx context.Context) <-chan error {
	signal.Notify(l.configSignalChan, syscall.SIGHUP, syscall.SIGUSR1)

	if l.cfg.SecretStorage.Enabled && l.cfg.SecretStorage.PollInterval > 0 {
		l.ticker = time.NewTicker(l.cfg.SecretStorage.PollInterval)
	}

	go func() {
		defer signal.Stop(l.configSignalChan)
		defer close(l.configSignalChan)
		defer close(l.reloadErrors)

		if l.ticker != nil {
			defer l.ticker.Stop()
		}

		var reloadTickerChan <-chan time.Time
		if l.ticker != nil {
			reloadTickerChan = l.ticker.C
		}

		for {
			select {
			case <-ctx.Done():
				return

			case <-reloadTickerChan:
				l.configSignalChan <- syscall.SIGHUP

			case sig := <-l.configSignalChan:
				switch sig {
				case syscall.SIGHUP:
					l.handleConfigReload(ctx)

				case syscall.SIGUSR1:
					l.DumpConfig()
				}
			}
		}
	}()

	return l.reloadErrors
}

// DumpConfig outputs the current configuration to stdout as JSON.
func (l *Loader) DumpConfig() {
	configJSON, err := json.MarshalIndent(l.cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error marshaling config: %v\n", err)

		return
	}

	fmt.Fprintf(os.Stdout, "\n=== Configuration Dump ===\n%s\n=== End Configuration ===\n\n", string(configJSON))
}

func (l *Loader) handleConfigReload(ctx context.Context) {
	metadata, err := loadSecretsFromPath(ctx, l.secretsRepo, l.cfg, "metadata")
	if err != nil {
		l.reportReloadStatus(fmt.Errorf("failed to load secret metadata: %w", err))

		return
	}

	currentVersion, err := getSecretVersion(metadata)
	if err != nil {
		l.reportReloadStatus(fmt.Errorf("failed to get secret version: %w", err))

		return
	}

	if currentVersion == l.lastVersion {
		return
	}

	version, err := Load(ctx, l.secretsRepo, l.cfg)
	if err != nil {
		l.reportReloadStatus(err)

		return
	}

	l.lastVersion = version
	l.reportReloadStatus(nil)
}

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

	if len(APIVersion) != 0 {
		cfg.AppConfig.APIVersion = APIVersion
	}

	return cfg, nil
}

func Load(ctx context.Context, secretsRepo ports.SecretsRepository, cfg *ServiceConfig) (int, error) {
	if !cfg.SecretStorage.Enabled {
		return 0, fmt.Errorf("secret storage is not enabled")
	}

	if err := authenticateVault(ctx, secretsRepo, cfg.SecretStorage); err != nil {
		return 0, fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	data, err := loadSecretsFromPath(ctx, secretsRepo, cfg, "data")
	if err != nil {
		return 0, fmt.Errorf("failed to load secrets from Vault: %w", err)
	}

	if err := applySecretsToConfig(cfg, data); err != nil {
		return 0, fmt.Errorf("failed to apply secrets to config: %w", err)
	}

	metadata, err := loadSecretsFromPath(ctx, secretsRepo, cfg, "metadata")
	if err != nil {
		return 0, fmt.Errorf("failed to load secret metadata: %w", err)
	}

	version, err := getSecretVersion(metadata)
	if err != nil {
		return 0, fmt.Errorf("failed to get secret version: %w", err)
	}

	return version, nil
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

func getSecretsWithRetry(ctx context.Context, client ports.SecretsRepository, cfg *ServiceConfig, pathType, mountPath string) (*api.Secret, error) {
	path := fmt.Sprintf("apps/%s/%s", pathType, mountPath)

	ctx, cancel := context.WithTimeout(ctx, cfg.SecretStorage.Timeout)
	defer cancel()

	var secret *api.Secret
	var err error

	for attempt := 0; attempt <= cfg.SecretStorage.MaxRetries; attempt++ {
		secret, err = client.GetSecrets(ctx, path)
		if err == nil {
			break
		}

		if attempt < cfg.SecretStorage.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read from path %s after %d retries: %w", path, cfg.SecretStorage.MaxRetries, err)
	}

	return secret, nil
}

func getSecretVersion(metadata map[string]interface{}) (int, error) {
	if metadata == nil {
		return 0, nil
	}

	currentVersion, ok := metadata["current_version"]
	if !ok {
		return 0, nil
	}

	switch v := currentVersion.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case json.Number:
		version, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("failed to parse version: %w", err)
		}

		return int(version), nil
	default:
		return 0, fmt.Errorf("unexpected version type: %T", currentVersion)
	}
}

func loadSecretsFromPath(ctx context.Context, client ports.SecretsRepository, cfg *ServiceConfig, pathType string) (map[string]interface{}, error) {
	secret, err := getSecretsWithRetry(ctx, client, cfg, "data", cfg.SecretStorage.MountPath)
	if err != nil {
		return nil, err
	}

	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	// For KV v2, the response structure is: secret.Data[pathType] where pathType is "data" or "metadata"
	result, ok := secret.Data[pathType].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format at path apps/data/%s, missing '%s' key", cfg.SecretStorage.MountPath, pathType)
	}

	return result, nil
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
	// Middleware environment variable and update config based on key
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

// reportReloadStatus sends reload status (error or nil for success) to reloadErrors channel.
// It uses non-blocking send to avoid blocking if no receiver is ready.
func (l *Loader) reportReloadStatus(err error) {
	select {
	case l.reloadErrors <- err:
	default:
	}
}
