package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

type (
	SubscriberService interface {
		ProcessAnalysisRequest(ctx context.Context, payload domain.AnalysisRequestPayload) (*domain.ProcessAnalysisMessageResult, error)
	}

	subscriberService struct {
		analysisRepo ports.AnalysisRepository
		outboxRepo   ports.OutboxRepository
		cacheRepo    ports.CacheRepository
		webFetcher   ports.WebFetcher
		htmlAnalyzer domain.HTMLAnalyzer
		linkChecker  ports.LinkChecker
		logger       infrastructure.Logger
		metrics      infrastructure.Metrics
	}
)

func NewSubscriberService(
	analysisRepo ports.AnalysisRepository,
	outboxRepo ports.OutboxRepository,
	cacheRepo ports.CacheRepository,
	webFetcher ports.WebFetcher,
	htmlAnalyzer domain.HTMLAnalyzer,
	linkChecker ports.LinkChecker,
	logger infrastructure.Logger,
	metrics infrastructure.Metrics,
) SubscriberService {
	return &subscriberService{
		analysisRepo: analysisRepo,
		outboxRepo:   outboxRepo,
		cacheRepo:    cacheRepo,
		webFetcher:   webFetcher,
		htmlAnalyzer: htmlAnalyzer,
		linkChecker:  linkChecker,
		logger:       logger,
		metrics:      metrics,
	}
}

func (s *subscriberService) ProcessAnalysisRequest(
	ctx context.Context,
	payload domain.AnalysisRequestPayload,
) (*domain.ProcessAnalysisMessageResult, error) {
	s.logger.Info().
		Str("analysis_id", payload.AnalysisID.String()).
		Str("url", payload.URL).
		Msg("processing analysis request")

	outboxEvent, err := s.outboxRepo.GetByAggregateID(ctx, payload.AnalysisID.String())
	if err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "OUTBOX_ERROR",
			ErrorMessage: fmt.Sprintf("failed to get outbox event: %v", err),
		}, nil
	}

	if err := s.outboxRepo.MarkProcessed(ctx, outboxEvent.ID.String()); err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "OUTBOX_ERROR",
			ErrorMessage: fmt.Sprintf("failed to mark outbox event as processed: %v", err),
		}, nil
	}

	if err := s.analysisRepo.UpdateStatus(ctx, payload.AnalysisID.String(), domain.StatusInProgress); err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "STATUS_UPDATE_ERROR",
			ErrorMessage: fmt.Sprintf("failed to update analysis status: %v", err),
		}, nil
	}

	// Invalidate cache after status update
	if s.cacheRepo != nil {
		if err := s.cacheRepo.Delete(ctx, payload.AnalysisID.String()); err != nil {
			s.logger.Warn().Err(err).Str("analysis_id", payload.AnalysisID.String()).
				Msg("failed to invalidate cache after status update")
		}
	}

	content, err := s.webFetcher.Fetch(ctx, payload.URL, payload.Options.Timeout)
	if err != nil {
		if updateErr := s.analysisRepo.MarkFailed(ctx, payload.AnalysisID.String(), "FETCH_ERROR", err.Error(), 0); updateErr != nil {
			s.logger.Error().Err(updateErr).Str("analysis_id", payload.AnalysisID.String()).
				Msg("failed to mark analysis as failed")
		} else {
			// Invalidate cache after marking as failed
			if s.cacheRepo != nil {
				if cacheErr := s.cacheRepo.Delete(ctx, payload.AnalysisID.String()); cacheErr != nil {
					s.logger.Warn().Err(cacheErr).Str("analysis_id", payload.AnalysisID.String()).
						Msg("failed to invalidate cache after marking as failed")
				}
			}
		}

		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "FETCH_ERROR",
			ErrorMessage: fmt.Sprintf("failed to fetch web page: %v", err),
		}, nil
	}

	contentHashObj := domain.NewContentHash(content.HTML)
	contentHash := contentHashObj.String()

	existingAnalysis, err := s.checkDuplicateContent(ctx, contentHash)
	if err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "DUPLICATE_CHECK_ERROR",
			ErrorMessage: fmt.Sprintf("failed to check duplicate content: %v", err),
		}, nil
	}

	if existingAnalysis != nil {
		if err := s.copyAnalysisResults(ctx, payload.AnalysisID, contentHash, existingAnalysis); err != nil {
			return &domain.ProcessAnalysisMessageResult{
				Success:      false,
				ErrorCode:    "COPY_RESULTS_ERROR",
				ErrorMessage: fmt.Sprintf("failed to copy analysis results: %v", err),
			}, nil
		}

		s.logger.Info().
			Str("analysis_id", payload.AnalysisID.String()).
			Str("source_analysis_id", existingAnalysis.ID.String()).
			Str("content_hash", contentHash).
			Msg("copied results from existing analysis (duplicate content)")
	} else {
		if err := s.performFullAnalysis(ctx, payload.AnalysisID, contentHash, content, payload.Options); err != nil {
			return &domain.ProcessAnalysisMessageResult{
				Success:      false,
				ErrorCode:    "ANALYSIS_ERROR",
				ErrorMessage: fmt.Sprintf("failed to perform full analysis: %v", err),
			}, nil
		}

		s.logger.Info().
			Str("analysis_id", payload.AnalysisID.String()).
			Str("content_hash", contentHash).
			Msg("completed full analysis")
	}

	durationMs := time.Since(outboxEvent.CreatedAt).Milliseconds()

	if err := s.analysisRepo.UpdateCompletionDuration(ctx, payload.AnalysisID.String(), durationMs); err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "DURATION_UPDATE_ERROR",
			ErrorMessage: fmt.Sprintf("failed to update analysis duration: %v", err),
		}, nil
	}

	if err := s.outboxRepo.MarkCompleted(ctx, outboxEvent.ID.String()); err != nil {
		return &domain.ProcessAnalysisMessageResult{
			Success:      false,
			ErrorCode:    "OUTBOX_ERROR",
			ErrorMessage: fmt.Sprintf("failed to mark outbox event as completed: %v", err),
		}, nil
	}

	if s.cacheRepo != nil {
		if err := s.cacheRepo.Delete(ctx, payload.AnalysisID.String()); err != nil {
			s.logger.Warn().Err(err).Str("analysis_id", payload.AnalysisID.String()).
				Msg("failed to invalidate cache after completion")
		}
	}

	s.logger.Info().
		Str("analysis_id", payload.AnalysisID.String()).
		Str("content_hash", contentHash).
		Msg("successfully processed analysis request")

	return &domain.ProcessAnalysisMessageResult{
		Success:     true,
		ContentHash: contentHash,
	}, nil
}

func (s *subscriberService) checkDuplicateContent(ctx context.Context, contentHash string) (*domain.Analysis, error) {
	analysis, err := s.analysisRepo.FindByContentHash(ctx, contentHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check for existing content hash: %w", err)
	}

	return analysis, nil
}

func (s *subscriberService) copyAnalysisResults(ctx context.Context, analysisID uuid.UUID, contentHash string, sourceAnalysis *domain.Analysis) error {
	if err := s.analysisRepo.Update(
		ctx, analysisID.String(), contentHash, sourceAnalysis.ContentSize, sourceAnalysis.Results,
	); err != nil {
		return fmt.Errorf("failed to copy results from existing analysis: %w", err)
	}

	if s.cacheRepo == nil {
		return nil
	}

	analysis, err := s.analysisRepo.Find(ctx, analysisID.String())
	if err != nil {
		s.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
			Msg("failed to fetch updated analysis for cache")

		return nil
	}

	if err := s.cacheRepo.Set(ctx, analysis); err != nil {
		s.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
			Msg("failed to update cache after copying analysis results")
	}

	return nil
}

func (s *subscriberService) performFullAnalysis(ctx context.Context, analysisID uuid.UUID, contentHash string, content *domain.WebPageContent, options domain.AnalysisOptions) error {
	processingStart := time.Now()

	results, err := s.htmlAnalyzer.Analyze(ctx, content.URL, content.HTML, options)
	if err != nil {
		return fmt.Errorf("failed to analyze HTML: %w", err)
	}

	if options.CheckLinks && s.linkChecker != nil && len(results.Links.ExternalLinks) > 0 {
		results.Links.InaccessibleLinks = s.linkChecker.CheckAccessibility(ctx, results.Links.ExternalLinks)
	}

	processingDuration := time.Since(processingStart)

	results.FetchTime = uint64(content.FetchDuration.Milliseconds())
	results.ProcessingTime = uint64(processingDuration.Milliseconds())

	s.logger.Info().
		Str("analysis_id", analysisID.String()).
		Uint64("fetch_time_ms", results.FetchTime).
		Uint64("processing_time_ms", results.ProcessingTime).
		Msg("analysis timing captured")

	s.metrics.RecordFetchTime(ctx, content.FetchDuration)
	s.metrics.RecordProcessingTime(ctx, processingDuration)

	if err := s.analysisRepo.Update(ctx, analysisID.String(), contentHash, int64(len(content.HTML)), results); err != nil {
		return fmt.Errorf("failed to save analysis results: %w", err)
	}

	if s.cacheRepo == nil {
		return nil
	}

	analysis, err := s.analysisRepo.Find(ctx, analysisID.String())
	if err != nil {
		s.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
			Msg("failed to fetch updated analysis for cache")

		return nil
	}

	if err := s.cacheRepo.Set(ctx, analysis); err != nil {
		s.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
			Msg("failed to update cache after saving analysis results")
	}

	return nil
}
