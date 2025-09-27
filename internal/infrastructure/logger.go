package infrastructure

import (
	"os"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/rs/zerolog"
)

type Logger struct {
	*zerolog.Logger
}

func New(cfg config.LoggingConfig) *Logger {
	var level zerolog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn", "warning":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	case "panic":
		level = zerolog.PanicLevel
	default:
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	if cfg.Format == "json" {
		logger = zerolog.New(os.Stdout)
	}

	logger = logger.With().Timestamp().Logger()

	return &Logger{
		Logger: &logger,
	}
}
