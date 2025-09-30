package infrastructure

import (
	"database/sql"
	"fmt"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Storage struct {
	config config.StorageConfig
	db     *sqlx.DB
}

func NewStorage(config config.StorageConfig) (*Storage, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.Username,
		config.Password,
		config.Database,
		config.SSLMode,
	)

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Storage{
		config: config,
		db:     db,
	}, nil
}

func (s *Storage) GetDB() (*sqlx.DB, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	return s.db, nil
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}

	return nil
}

func (s *Storage) Ping() error {
	if s.db == nil {
		return fmt.Errorf("database connection not initialized")
	}
	return s.db.Ping()
}

func (s *Storage) Stats() sql.DBStats {
	if s.db == nil {
		return sql.DBStats{}
	}

	return s.db.DB.Stats()
}
