package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

// MessageHandler defines the interface for processing queue messages
type MessageHandler interface {
	ProcessMessage(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error
}
