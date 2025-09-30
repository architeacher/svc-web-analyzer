package infrastructure

import (
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	appLogger "github.com/architeacher/svc-web-analyzer/pkg/logger"
)

type Profiler struct {
	config config.ProfilerConfig
	logger appLogger.Logger

	cpuProfileFile *os.File
}

func NewProfiler(cfg config.ProfilerConfig, logger appLogger.Logger) *Profiler {
	return &Profiler{
		config: cfg,
		logger: logger,
	}
}

func (p Profiler) Start() error {
	if !p.config.Enabled {
		return nil
	}

	if p.config.Path != "" {
		file, err := os.Create(p.config.Path)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}

		p.cpuProfileFile = file

		if err := pprof.StartCPUProfile(file); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}

		p.logger.Info().Str("path", p.config.Path).Msg("cpu profiling started")
	}

	if p.config.Port != 0 {
		go func() {
			addr := fmt.Sprintf(":%d", p.config.Port)

			p.logger.Info().Str("addr", addr).Msg("pprof server listening")

			if err := http.ListenAndServe(addr, nil); err != nil {
				p.logger.Error().Err(err).Msg("pprof server error")
			}
		}()
	}

	return nil
}

func (p Profiler) Stop() error {
	if !p.config.Enabled || p.config.Path == "" {
		return nil
	}

	pprof.StopCPUProfile()

	if p.cpuProfileFile != nil {
		if err := p.cpuProfileFile.Close(); err != nil {
			return fmt.Errorf("error closing CPU profile: %w", err)
		}
	}

	p.logger.Info().Msg("CPU profiling stopped")

	return nil
}
