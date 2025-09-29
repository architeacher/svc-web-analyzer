package ports

import (
	"context"
	"database/sql"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
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
		SaveInTx(tx *sql.Tx, url string, options domain.AnalysisOptions) (*domain.Analysis, error)
	}

	// Updater updates an entry or entries in the database.
	Updater interface {
		Update(ctx context.Context, url string, options domain.AnalysisOptions) error
	}

	// Deleter deletes an entry or entries from the database.
	Deleter interface {
		Delete(ctx context.Context, analysisID string) error
	}

	AnalysisRepository interface {
		Finder
		Saver
		TransactionalSaver
	}

	// OutboxRepository handles outbox events for reliable message delivery.
	OutboxRepository interface {
		// SaveInTx saves an outbox event within a transaction
		SaveInTx(tx *sql.Tx, event *domain.OutboxEvent) error

		// FindPending finds pending outbox events ordered by priority and creation time.
		FindPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)

		// FindRetryable finds failed events that are ready for retry.
		FindRetryable(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)

		// ClaimForProcessing atomically claims an event for processing.
		ClaimForProcessing(ctx context.Context, eventID string) (*domain.OutboxEvent, error)

		// MarkPublished marks an event as successfully published.
		MarkPublished(ctx context.Context, eventID string) error

		// MarkFailed marks an event as failed with error details and retry timing.
		MarkFailed(ctx context.Context, eventID string, errorDetails string, nextRetryAt *time.Time) error

		// MarkPermanentlyFailed marks an event as permanently failed after max retries.
		MarkPermanentlyFailed(ctx context.Context, eventID string, errorDetails string) error
	}
)
