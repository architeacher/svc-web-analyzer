package ports

import (
	"context"

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
	}
)
