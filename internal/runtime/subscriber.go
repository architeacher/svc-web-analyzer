package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	cacheRepo     ports.CacheRepository
	webFetcher    ports.WebPageFetcher
	htmlAnalyzer  domain.HTMLAnalyzer
	linkChecker   ports.LinkChecker
	logger        *infrastructure.Logger
	storageClient *infrastructure.Storage
}

func NewAnalysisWorker(
	analysisRepo ports.AnalysisRepository,
	cacheRepo ports.CacheRepository,
	webFetcher ports.WebPageFetcher,
	htmlAnalyzer domain.HTMLAnalyzer,
	linkChecker ports.LinkChecker,
	storageClient *infrastructure.Storage,
	logger *infrastructure.Logger,
) *AnalysisWorker {
	return &AnalysisWorker{
		analysisRepo:  analysisRepo,
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

	if err := w.updateAnalysisStatus(ctx, payload.AnalysisID, domain.StatusInProgress); err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to update analysis status to in_progress")
		return ctrl.Requeue(msg)
	}

	content, err := w.webFetcher.Fetch(ctx, payload.URL, payload.Options.Timeout)
	if err != nil {
		if updateErr := w.markAnalysisFailed(ctx, payload.AnalysisID, "FETCH_ERROR", err.Error(), 0); updateErr != nil {
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
	existingAnalysis, err := w.findAnalysisByContentHash(ctx, contentHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check for existing content hash: %w", err)
	}

	if existingAnalysis != nil {
		return w.copyResultsFromExisting(ctx, analysisID, existingAnalysis, contentHash)
	}

	return w.performFullAnalysis(ctx, analysisID, contentHash, content, options)
}

func (w *AnalysisWorker) findAnalysisByContentHash(ctx context.Context, contentHash string) (*domain.Analysis, error) {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		SELECT id, url, status, results, created_at, completed_at, duration
		FROM analysis
		WHERE content_hash = $1 AND status = 'completed' AND results IS NOT NULL
		LIMIT 1`

	row := db.QueryRowContext(ctx, query, contentHash)

	var analysis domain.Analysis
	var resultsJSON []byte
	var duration *int64

	err = row.Scan(
		&analysis.ID,
		&analysis.URL,
		&analysis.Status,
		&resultsJSON,
		&analysis.CreatedAt,
		&analysis.CompletedAt,
		&duration,
	)

	if err != nil {
		return nil, err
	}

	if duration != nil {
		d := time.Duration(*duration) * time.Millisecond
		analysis.Duration = &d
	}

	if resultsJSON != nil {
		var results domain.AnalysisData
		if err := json.Unmarshal(resultsJSON, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
		analysis.Results = &results
	}

	return &analysis, nil
}

func (w *AnalysisWorker) copyResultsFromExisting(
	ctx context.Context,
	analysisID uuid.UUID,
	existingAnalysis *domain.Analysis,
	contentHash string,
) error {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	resultsJSON, err := json.Marshal(existingAnalysis.Results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	var durationMs *int64
	if existingAnalysis.Duration != nil {
		ms := existingAnalysis.Duration.Milliseconds()
		durationMs = &ms
	}

	query := `
		UPDATE analysis
		SET content_hash = $2,
			status = 'completed',
			results = $3,
			duration = $4,
			completed_at = NOW()
		WHERE id = $1`

	_, err = db.ExecContext(ctx, query, analysisID, contentHash, resultsJSON, durationMs)
	if err != nil {
		return fmt.Errorf("failed to copy results from existing analysis: %w", err)
	}

	// Update cache with copied analysis
	if w.cacheRepo != nil {
		analysis := &domain.Analysis{
			ID:      analysisID,
			Status:  domain.StatusCompleted,
			Results: existingAnalysis.Results,
		}
		if err := w.cacheRepo.Set(ctx, analysis); err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
				Msg("failed to update cache after copying analysis results")
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
	startTime := time.Now()

	if err := w.updateContentHash(ctx, analysisID, contentHash, int64(len(content.HTML))); err != nil {
		return fmt.Errorf("failed to update content hash: %w", err)
	}

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

	duration := time.Since(startTime)

	if err := w.saveAnalysisResults(ctx, analysisID, results, duration); err != nil {
		return fmt.Errorf("failed to save analysis results: %w", err)
	}

	// Update cache with completed analysis
	if w.cacheRepo != nil {
		analysis := &domain.Analysis{
			ID:      analysisID,
			Status:  domain.StatusCompleted,
			Results: results,
		}
		if err := w.cacheRepo.Set(ctx, analysis); err != nil {
			w.logger.Warn().Err(err).Str("analysis_id", analysisID.String()).
				Msg("failed to update cache after saving analysis results")
		}
	}

	w.logger.Info().
		Str("analysis_id", analysisID.String()).
		Str("content_hash", contentHash).
		Dur("duration", duration).
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

func (w *AnalysisWorker) updateAnalysisStatus(ctx context.Context, analysisID uuid.UUID, status domain.AnalysisStatus) error {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	var query string
	var args []interface{}

	if status == domain.StatusInProgress {
		query = "UPDATE analysis SET status = $1, started_at = NOW() WHERE id = $2"
		args = []interface{}{status, analysisID}
	} else {
		query = "UPDATE analysis SET status = $1 WHERE id = $2"
		args = []interface{}{status, analysisID}
	}

	_, err = db.ExecContext(ctx, query, args...)
	return err
}

func (w *AnalysisWorker) updateContentHash(ctx context.Context, analysisID uuid.UUID, contentHash string, contentSize int64) error {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := "UPDATE analysis SET content_hash = $1, content_size = $2 WHERE id = $3"
	_, err = db.ExecContext(ctx, query, contentHash, contentSize, analysisID)
	return err
}

func (w *AnalysisWorker) saveAnalysisResults(
	ctx context.Context,
	analysisID uuid.UUID,
	results *domain.AnalysisData,
	duration time.Duration,
) error {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	query := `
		UPDATE analysis
		SET status = 'completed',
			results = $2,
			duration = $3,
			completed_at = NOW()
		WHERE id = $1`

	_, err = db.ExecContext(ctx, query, analysisID, resultsJSON, duration.Milliseconds())
	return err
}

func (w *AnalysisWorker) markAnalysisFailed(
	ctx context.Context,
	analysisID uuid.UUID,
	errorCode string,
	errorMessage string,
	statusCode int,
) error {
	db, err := w.storageClient.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		UPDATE analysis
		SET status = 'failed',
			error_code = $2,
			error_message = $3,
			error_status_code = $4,
			completed_at = NOW()
		WHERE id = $1`

	_, err = db.ExecContext(ctx, query, analysisID, errorCode, errorMessage, statusCode)
	return err
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

	analysisRepo := adapters.NewPostgresRepository(c.storage)
	var cacheRepo ports.CacheRepository
	if c.cacheClient != nil {
		cacheRepo = adapters.NewCacheRepository(c.cacheClient, cfg.Cache, c.logger)
	}
	webFetcher := adapters.NewWebPageFetcher(cfg.WebFetcher, c.logger)
	htmlAnalyzer := adapters.NewHTMLAnalyzer(c.logger)
	linkChecker := adapters.NewLinkChecker(cfg.LinkChecker, c.logger)

	c.worker = NewAnalysisWorker(analysisRepo, cacheRepo, webFetcher, htmlAnalyzer, linkChecker, c.storage, c.logger)

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
		c.logger.Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
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
		c.queue.Close()
	}

	if c.cacheClient != nil {
		c.cacheClient.Close()
	}

	if c.storage != nil {
		c.storage.Close()
	}

	c.logger.Info().Msg("Cleanup completed")
}
