package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/redis/go-redis/v9"
)

type KeydbClient struct {
	client *redis.Client
	logger *Logger
	config config.CacheConfig
}

func NewKeyDBClient(config config.CacheConfig, logger *Logger) *KeydbClient {
	opts := &redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
		MaxRetries:   config.MaxRetries,
	}

	client := redis.NewClient(opts)

	return &KeydbClient{
		client: client,
		logger: logger,
		config: config,
	}
}

func (c *KeydbClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *KeydbClient) Close() error {
	return c.client.Close()
}

func (c *KeydbClient) Get(ctx context.Context, key string) ([]byte, error) {
	startTime := time.Now()

	result, err := c.client.Get(ctx, key).Result()
	duration := time.Since(startTime)

	c.logger.Debug().
		Str("key", key).
		Int64("duration_ms", duration.Milliseconds()).
		Bool("hit", err == nil).
		Msg("keydb get operation")

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrCacheUnavailable
		}
		c.logger.Error().
			Str("key", key).
			Str("error", err.Error()).
			Msg("keydb get operation failed")

		return nil, err
	}

	return []byte(result), nil
}

func (c *KeydbClient) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	if expiry == 0 {
		expiry = c.config.DefaultExpiry
	}

	startTime := time.Now()

	err := c.client.Set(ctx, key, value, expiry).Err()
	duration := time.Since(startTime)

	c.logger.Debug().
		Str("key", key).
		Str("expiry", expiry.String()).
		Int64("duration_ms", duration.Milliseconds()).
		Bool("success", err == nil).
		Msg("keydb set operation")

	if err != nil {
		c.logger.Error().
			Str("key", key).
			Str("error", err.Error()).
			Msg("keydb set operation failed")
	}

	return err
}

func (c *KeydbClient) Delete(ctx context.Context, key string) error {
	startTime := time.Now()

	err := c.client.Del(ctx, key).Err()
	duration := time.Since(startTime)

	c.logger.Debug().
		Str("key", key).
		Int64("duration_ms", duration.Milliseconds()).
		Bool("success", err == nil).
		Msg("keydb delete operation")

	if err != nil {
		c.logger.Error().
			Str("key", key).
			Str("error", err.Error()).
			Msg("keydb delete operation failed")
	}

	return err
}

// keydb statistics and monitoring

func (c *KeydbClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get keydb info
	info, err := c.client.Info(ctx, "memory", "stats", "clients").Result()
	if err != nil {
		return nil, err
	}

	stats["redis_info"] = info

	// Get pool stats
	poolStats := c.client.PoolStats()
	stats["pool_stats"] = map[string]interface{}{
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
	}

	return stats, nil
}

// HealthCheck verifies that the cache service is responsive
func (c *KeydbClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := c.Ping(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}
