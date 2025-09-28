package runtime

import (
	"context"
	"os"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

type Option func(client *ServiceCtx)

type TracerShutdownFunc func(ctx context.Context) error

func WithLogger(logger *infrastructure.Logger) Option {
	return func(sCtx *ServiceCtx) {
		sCtx.deps.logger = logger
	}
}

func WithConfig(cfg *config.ServiceConfig) Option {
	return func(sCtx *ServiceCtx) {
		sCtx.deps.cfg = cfg
	}
}

func WithServiceTermination(ch chan os.Signal) Option {
	return func(sCtx *ServiceCtx) {
		sCtx.shutdownChannel = ch
	}
}

func WithWaitingForServer() Option {
	return func(sCtx *ServiceCtx) {
		sCtx.serverReady = make(chan struct{})
	}
}
