package repos

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

const (
	keyPrefix         = "svc-web-analyzer:"
	analysisKeyPrefix = keyPrefix + "analysis:"
	resultKeyPrefix   = keyPrefix + "result:"
)

type CacheRepository struct {
	client *infrastructure.KeydbClient
	config config.CacheConfig
	logger infrastructure.Logger
}

func NewCacheRepository(client *infrastructure.KeydbClient, cfg config.CacheConfig, logger infrastructure.Logger) *CacheRepository {
	return &CacheRepository{
		client: client,
		config: cfg,
		logger: logger,
	}
}

func (r *CacheRepository) Find(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	key := analysisKeyPrefix + analysisID

	data, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis from cache: %w", err)
	}

	var analysis domain.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		r.logger.Error().
			Err(err).
			Str("analysis_id", analysisID).
			Msg("failed to unmarshal cached analysis result")

		return nil, fmt.Errorf("failed to unmarshal cached analysis result: %w", err)
	}

	r.logger.Info().Str("url", analysis.URL).Msg("analysis result retrieved from cache")

	return &analysis, nil
}

func (r *CacheRepository) Set(ctx context.Context, analysis *domain.Analysis) error {
	key := analysisKeyPrefix + analysis.ID.String()

	data, err := json.Marshal(analysis)
	if err != nil {
		r.logger.Error().
			Err(err).
			Str("analysis_id", analysis.ID.String()).
			Msg("failed to marshal analysis for caching")

		return err
	}

	if err := r.client.Set(ctx, key, data, r.config.DefaultExpiry); err != nil {
		r.logger.Error().
			Err(err).
			Str("analysis_id", analysis.ID.String()).
			Str("url", analysis.URL).
			Msg("failed to save analysis to cache")

		return fmt.Errorf("failed to save analysis to cache: %w", err)
	}

	r.logger.Debug().Str("analysis_id", analysis.ID.String()).Str("url", analysis.URL).Msg("analysis saved to cache")

	return nil
}

func (r *CacheRepository) Delete(ctx context.Context, analysisID string) error {
	key := analysisKeyPrefix + analysisID

	return r.client.Delete(ctx, key)
}

// generateAnalysisKey creates a unique cache key based on URL and analysis options
func (r *CacheRepository) generateAnalysisKey(url string, options domain.AnalysisOptions) string {
	data := fmt.Sprintf("%s:%t:%t:%t:%s",
		url,
		options.IncludeHeadings,
		options.CheckLinks,
		options.DetectForms,
		options.Timeout.String(),
	)

	hash := sha1.Sum([]byte(data))

	return resultKeyPrefix + fmt.Sprintf("%x", hash)
}
