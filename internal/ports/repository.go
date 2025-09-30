//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/jmoiron/sqlx"
)

type (
	// Finder reads data from the database.
	Finder interface {
		Find(ctx context.Context, analysisID string) (*domain.Analysis, error)
	}

	// Saver saves an entry in the database.
	Saver interface {
		Save(ctx context.Context, url string, options domain.AnalysisOptions) (*domain.Analysis, error)
	}

	// TransactionalSaver saves an entry in the database within a transaction.
	TransactionalSaver interface {
		SaveInTx(ctx context.Context, tx *sqlx.Tx, url string, options domain.AnalysisOptions) (*domain.Analysis, error)
	}

	Updater interface {
		Update(ctx context.Context, analysisID, contentHash string, contentSize int64, results *domain.AnalysisData) error
		UpdateStatus(ctx context.Context, analysisID string, status domain.AnalysisStatus) error
		UpdateCompletionDuration(ctx context.Context, analysisID string, durationMs int64) error
		MarkFailed(ctx context.Context, analysisID, errorCode, errorMessage string, statusCode int) error
	}

	// Deleter deletes an entry or entries from the database.
	Deleter interface {
		Delete(ctx context.Context, analysisID string) error
	}
)

//counterfeiter:generate -o ../mocks/analysis_repository.go . AnalysisRepository
//counterfeiter:generate -o ../mocks/outbox_repository.go . OutboxRepository
type (
	// AnalysisRepository provides methods for managing web page analysis data.
	AnalysisRepository interface {
		Finder
		FindByContentHash(ctx context.Context, contentHash string) (*domain.Analysis, error)
		Saver
		TransactionalSaver
		Updater
		Deleter
	}

	// OutboxRepository handles outbox events for reliable message delivery.
	OutboxRepository interface {
		// SaveInTx saves an outbox event within a transaction
		SaveInTx(ctx context.Context, tx *sqlx.Tx, event *domain.OutboxEvent) error

		// FindPending finds pending outbox events ordered by priority and creation time.
		FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)

		// FindRetryable finds failed events that are ready for retry.
		FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)

		// ClaimForProcessing atomically claims an event for processing.
		ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error)

		// MarkPublished marks an event as successfully published.
		MarkPublished(ctx context.Context, eventID string) error

		// MarkProcessed marks when a Subscriber starts processing the analysis.
		MarkProcessed(ctx context.Context, eventID string) error

		// MarkCompleted marks when a Subscriber completes the analysis.
		MarkCompleted(ctx context.Context, eventID string) error

		// MarkFailed marks an event as failed with error details and retry timing.
		MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error

		// MarkPermanentlyFailed marks an event as permanently failed after max retries.
		MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error

		// GetByAggregateID retrieves the most recent outbox event for an aggregate.
		GetByAggregateID(ctx context.Context, aggregateID string) (*domain.OutboxEvent, error)
	}
)
