package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/architeacher/svc-web-analyzer/internal/adapters"
	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

type AnalysisWorker struct {
	analysisRepo  ports.AnalysisRepository
	outboxRepo    ports.OutboxRepository
	cacheRepo     ports.CacheRepository
	webFetcher    ports.WebPageFetcher
	htmlAnalyzer  domain.HTMLAnalyzer
	linkChecker   ports.LinkChecker
	logger        *infrastructure.Logger
	storageClient *infrastructure.Storage
}

func NewAnalysisWorker(
	analysisRepo ports.AnalysisRepository,
	outboxRepo ports.OutboxRepository,
	cacheRepo ports.CacheRepository,
	webFetcher ports.WebPageFetcher,
	htmlAnalyzer domain.HTMLAnalyzer,
	linkChecker ports.LinkChecker,
	storageClient *infrastructure.Storage,
	logger *infrastructure.Logger,
) *AnalysisWorker {
	return &AnalysisWorker{
		analysisRepo:  analysisRepo,
		outboxRepo:    outboxRepo,
		cacheRepo:     cacheRepo,
		webFetcher:    webFetcher,
		htmlAnalyzer:  htmlAnalyzer,
		linkChecker:   linkChecker,
		storageClient: storageClient,
		logger:        logger,
	}
}

func (w *AnalysisWorker) ProcessMessage(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error {
	var payload domain.AnalysisRequestPayload
	if err := msg.Unmarshal(&payload); err != nil {
		w.logger.Error().Err(err).Msg("failed to unmarshal message payload")

		return ctrl.Reject(msg)
	}

	w.logger.Info().
		Str("analysis_id", payload.AnalysisID.String()).
		Str("url", payload.URL).
		Msg("Processing analysis request")

	outboxEventID, err := w.getOutboxEventID(ctx, payload.AnalysisID)
	if err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to get outbox event ID")

		return ctrl.Requeue(msg)
	}

	if err := w.outboxRepo.MarkProcessed(ctx, outboxEventID.String()); err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to mark outbox event as processed")

		return ctrl.Requeue(msg)
	}

	if err := w.analysisRepo.UpdateStatus(ctx, payload.AnalysisID.String(), domain.StatusInProgress); err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to update analysis status to in_progress")

		return ctrl.Requeue(msg)
	}

	content, err := w.webFetcher.Fetch(ctx, payload.URL, payload.Options.Timeout)
	if err != nil {
		if updateErr := w.analysisRepo.MarkFailed(ctx, payload.AnalysisID.String(), "FETCH_ERROR", err.Error(), 0); updateErr != nil {
			w.logger.Error().Err(updateErr).Str("analysis_id", payload.AnalysisID.String()).
				Msg("failed to mark analysis as failed")
		}

		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Str("url", payload.URL).Msg("failed to fetch web page")

		return ctrl.Ack(msg)
	}

	contentHash := w.CalculateContentHash(content.HTML)

	if err := w.handleContentDeduplication(ctx, payload.AnalysisID, contentHash, content, payload.Options); err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to handle content deduplication")

		return ctrl.Requeue(msg)
	}

	if err := w.outboxRepo.MarkCompleted(ctx, outboxEventID.String()); err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to mark outbox event as completed")

		return ctrl.Requeue(msg)
	}

	// Invalidate cache so the next fetch gets updated data with duration and completed_at.
	if w.cacheRepo != nil {
		if err := w.cacheRepo.Delete(ctx, payload.AnalysisID.String()); err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", payload.AnalysisID.String()).
				Msg("failed to invalidate cache after completion")
		}
	}

	w.logger.Info().
		Str("analysis_id", payload.AnalysisID.String()).
		Str("content_hash", contentHash).
		Msg("Successfully processed analysis request")

	return ctrl.Ack(msg)
}

func (w *AnalysisWorker) CalculateContentHash(html string) string {
	hash := sha256.Sum256([]byte(html))

	return hex.EncodeToString(hash[:])
}

func (w *AnalysisWorker) handleContentDeduplication(
	ctx context.Context,
	analysisID uuid.UUID,
	contentHash string,
	content *domain.WebPageContent,
	options domain.AnalysisOptions,
) error {
	existingAnalysis, err := w.analysisRepo.FindByContentHash(ctx, contentHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check for existing content hash: %w", err)
	}

	if existingAnalysis != nil {
		return w.copyResultsFromExisting(ctx, analysisID, existingAnalysis, contentHash)
	}

	return w.performFullAnalysis(ctx, analysisID, contentHash, content, options)
}

func (w *AnalysisWorker) copyResultsFromExisting(
	ctx context.Context,
	analysisID uuid.UUID,
	existingAnalysis *domain.Analysis,
	contentHash string,
) error {
	if err := w.analysisRepo.Update(ctx, analysisID.String(), contentHash, existingAnalysis.ContentSize, existingAnalysis.Results); err != nil {
		return fmt.Errorf("failed to copy results from existing analysis: %w", err)
	}

	if w.cacheRepo != nil {
		analysis, err := w.analysisRepo.Find(ctx, analysisID.String())
		if err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
				Msg("failed to fetch updated analysis for cache")
		} else {
			if err := w.cacheRepo.Set(ctx, analysis); err != nil {
				w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
					Msg("failed to update cache after copying analysis results")
			}
		}
	}

	w.logger.Info().
		Str("analysis_id", analysisID.String()).
		Str("source_analysis_id", existingAnalysis.ID.String()).
		Str("content_hash", contentHash).
		Msg("Copied results from existing analysis (duplicate content)")

	return nil
}

func (w *AnalysisWorker) performFullAnalysis(
	ctx context.Context,
	analysisID uuid.UUID,
	contentHash string,
	content *domain.WebPageContent,
	options domain.AnalysisOptions,
) error {
	results := &domain.AnalysisData{}

	// Perform synchronous basic analysis first
	results.HTMLVersion = w.htmlAnalyzer.ExtractHTMLVersion(content.HTML)
	results.Title = w.htmlAnalyzer.ExtractTitle(content.HTML)

	// Use goroutines for concurrent analysis of different components
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Channel for collecting errors from concurrent operations
	errCh := make(chan error, 3)

	// Concurrent heading extraction
	if options.IncludeHeadings {
		wg.Go(func() {
			headingCounts := w.htmlAnalyzer.ExtractHeadingCounts(content.HTML)
			mu.Lock()
			results.HeadingCounts = headingCounts
			mu.Unlock()
		})
	}

	// Concurrent link extraction and analysis
	wg.Go(func() {
		links, err := w.htmlAnalyzer.ExtractLinks(content.HTML, content.URL)
		if err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
				Msg("failed to extract links")
			errCh <- nil // Don't fail the entire analysis for link extraction errors
			return
		}

		linkAnalysis := w.AnalyzeLinks(ctx, links, options.CheckLinks)
		mu.Lock()
		results.Links = linkAnalysis
		mu.Unlock()
		errCh <- nil
	})

	// Concurrent form detection
	if options.DetectForms {
		wg.Go(func() {
			forms := w.htmlAnalyzer.ExtractForms(content.HTML, content.URL)
			mu.Lock()
			results.Forms = forms
			mu.Unlock()
			errCh <- nil
		})
	} else {
		errCh <- nil
	}

	// Wait for all concurrent operations to complete
	wg.Wait()
	close(errCh)

	// Check for any critical errors from concurrent operations
	for err := range errCh {
		if err != nil {
			return fmt.Errorf("concurrent analysis operation failed: %w", err)
		}
	}

	if err := w.analysisRepo.Update(ctx, analysisID.String(), contentHash, int64(len(content.HTML)), results); err != nil {
		return fmt.Errorf("failed to save analysis results: %w", err)
	}

	// Update cache with completed analysis
	if w.cacheRepo != nil {
		analysis, err := w.analysisRepo.Find(ctx, analysisID.String())
		if err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
				Msg("failed to fetch updated analysis for cache")
		} else {
			if err := w.cacheRepo.Set(ctx, analysis); err != nil {
				w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
					Msg("failed to update cache after saving analysis results")
			}
		}
	}

	w.logger.Info().
		Str("analysis_id", analysisID.String()).
		Str("content_hash", contentHash).
		Msg("Completed full analysis")

	return nil
}

func (w *AnalysisWorker) AnalyzeLinks(ctx context.Context, links []domain.Link, checkLinks bool) domain.LinkAnalysis {
	analysis := domain.LinkAnalysis{
		TotalCount:        len(links),
		InaccessibleLinks: []domain.InaccessibleLink{},
	}

	// Count link types concurrently if we have many links
	if len(links) > 100 {
		w.analyzeLinksConcurrently(links, &analysis)
	} else {
		// For smaller sets, sequential processing is fine
		for _, link := range links {
			switch link.Type {
			case domain.LinkTypeInternal:
				analysis.InternalCount++
			case domain.LinkTypeExternal:
				analysis.ExternalCount++
			}
		}
	}

	// Perform external link checking concurrently (already implemented in LinkChecker)
	if checkLinks && w.linkChecker != nil {
		var externalLinks []domain.Link
		for _, link := range links {
			if link.Type == domain.LinkTypeExternal {
				externalLinks = append(externalLinks, link)
			}
		}
		if len(externalLinks) > 0 {
			analysis.InaccessibleLinks = w.linkChecker.CheckAccessibility(ctx, externalLinks)
		}
	}

	return analysis
}

func (w *AnalysisWorker) analyzeLinksConcurrently(links []domain.Link, analysis *domain.LinkAnalysis) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	chunkSize := 50
	for i := 0; i < len(links); i += chunkSize {
		end := i + chunkSize
		if end > len(links) {
			end = len(links)
		}

		wg.Go(func() {
			chunk := links[i:end]
			var internalCount, externalCount int
			for _, link := range chunk {
				switch link.Type {
				case domain.LinkTypeInternal:
					internalCount++
				case domain.LinkTypeExternal:
					externalCount++
				}
			}

			mu.Lock()
			analysis.InternalCount += internalCount
			analysis.ExternalCount += externalCount
			mu.Unlock()
		})
	}

	wg.Wait()
}

func (w *AnalysisWorker) getOutboxEventID(ctx context.Context, analysisID uuid.UUID) (uuid.UUID, error) {
	event, err := w.outboxRepo.GetByAggregateID(ctx, analysisID.String())
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get outbox event: %w", err)
	}

	return event.ID, nil
}

type SubscriberCtx struct {
	worker      *AnalysisWorker
	logger      *infrastructure.Logger
	queue       infrastructure.Queue
	storage     *infrastructure.Storage
	cacheClient *infrastructure.KeydbClient
	cfg         *config.ServiceConfig

	shutdownChannel chan os.Signal
	ctx             context.Context
	cancelFunc      context.CancelFunc
}

func NewSubscriber() *SubscriberCtx {
	return &SubscriberCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}
}

func (c *SubscriberCtx) Run() {
	c.build()
	c.start()
	c.wait()
	c.shutdown()
}

func (c *SubscriberCtx) build() {
	c.ctx, c.cancelFunc = context.WithCancel(context.Background())

	cfg, err := config.Init()
	if err != nil {
		panic(fmt.Errorf("failed to load configuration: %w", err))
	}
	c.cfg = cfg

	c.logger = infrastructure.New(cfg.Logging)

	c.storage, err = infrastructure.NewStorage(cfg.Storage)
	if err != nil {
		c.logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	db, err := c.storage.GetDB()
	if err != nil {
		c.logger.Fatal().Err(err).Msg("failed to get database connection:")
	}

	c.cacheClient = infrastructure.NewKeyDBClient(cfg.Cache, c.logger)

	// Test cache connection
	cacheCtx, cacheCancel := context.WithTimeout(c.ctx, cfg.Cache.DialTimeout)
	defer cacheCancel()

	if err := c.cacheClient.Ping(cacheCtx); err != nil {
		c.logger.Warn().Err(err).Msg("failed to connect to cache, continuing without cache")
		c.cacheClient = nil
	} else {
		c.logger.Info().Msg("cache connection established")
	}

	analysisRepo := adapters.NewAnalysisRepository(db)
	outboxRepo := adapters.NewOutboxRepository(db)
	var cacheRepo ports.CacheRepository
	if c.cacheClient != nil {
		cacheRepo = adapters.NewCacheRepository(c.cacheClient, cfg.Cache, c.logger)
	}
	webFetcher := adapters.NewWebPageFetcher(cfg.WebFetcher, c.logger)
	htmlAnalyzer := adapters.NewHTMLAnalyzer(c.logger)
	linkChecker := adapters.NewLinkChecker(cfg.LinkChecker, c.logger)

	c.worker = NewAnalysisWorker(analysisRepo, outboxRepo, cacheRepo, webFetcher, htmlAnalyzer, linkChecker, c.storage, c.logger)

	queueConfig := queue.Config{
		Scheme:   "amqp",
		Username: cfg.Queue.Username,
		Password: cfg.Queue.Password,
		Host:     cfg.Queue.Host,
		Port:     cfg.Queue.Port,
		Vhost:    cfg.Queue.VirtualHost,
	}

	c.queue = queue.NewRabbitMQQueue(queueConfig,
		queue.WithLogger(queue.NewLoggerAdapter(c.logger)),
		queue.WithReconnectDelay(5*time.Second),
	)

	if err := c.queue.Connect(); err != nil {
		c.logger.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
}

func (c *SubscriberCtx) start() {
	c.logger.Info().
		Str("queue", c.cfg.Queue.QueueName).
		Msg("Starting analysis worker service")

	go func() {
		err := c.queue.Consume(c.ctx, c.cfg.Queue.QueueName, "analysis-worker", c.worker.ProcessMessage,
			queue.WithConsumingLogger(queue.NewLoggerAdapter(c.logger)),
			queue.WithErrorHandler(func(err error) {
				c.logger.Error().Err(err).Msg("Consumer error")
			}),
		)

		if err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Fatal().Err(err).Msg("Analysis worker failed")
		}
	}()
}

func (c *SubscriberCtx) wait() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
	<-c.shutdownChannel
}

func (c *SubscriberCtx) shutdown() {
	c.logger.Info().Msg("Received shutdown signal")
	defer c.cleanup()

	c.cancelFunc()
	c.logger.Info().Msg("Analysis worker service stopped")
}

func (c *SubscriberCtx) cleanup() {
	c.logger.Info().Msg("cleaning up resources...")

	if c.queue != nil {
		_ = c.queue.Close()
	}

	if c.cacheClient != nil {
		_ = c.cacheClient.Close()
	}

	if c.storage != nil {
		_ = c.storage.Close()
	}

	c.logger.Info().Msg("Cleanup completed")
}
