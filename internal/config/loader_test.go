package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "sandbox")
	t.Setenv("APP_SERVICE_VERSION", "1.0.0")
	t.Setenv("APP_COMMIT_SHA", "1234xwz")
	t.Setenv("LOGGING_LEVEL", "debug")
	t.Setenv("POSTGRES_PASSWORD", "test.Secret")
	t.Setenv("RABBITMQ_USERNAME", "john.doe")
	t.Setenv("KEYDB_PASSWORD", "insecure.password")

	cfg, err := Init()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "sandbox", cfg.AppConfig.Env)
	assert.Equal(t, "svc-web-analyzer", cfg.AppConfig.ServiceName)
	assert.Equal(t, "1.0.0", cfg.AppConfig.ServiceVersion)
	assert.Equal(t, "1234xwz", cfg.AppConfig.CommitSHA)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "test.Secret", cfg.Storage.Password)
	assert.Equal(t, "john.doe", cfg.Queue.Username)
	assert.Equal(t, "insecure.password", cfg.Cache.Password)
}
