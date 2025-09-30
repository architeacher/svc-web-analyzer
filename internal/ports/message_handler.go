//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

//counterfeiter:generate -o ../mocks/message_handler.go . MessageHandler

// MessageHandler defines the interface for processing queue messages
type MessageHandler interface {
	ProcessMessage(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error
}
