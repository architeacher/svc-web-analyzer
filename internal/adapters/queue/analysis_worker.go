package queue

import (
	"context"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/architeacher/svc-web-analyzer/internal/ports"
	"github.com/architeacher/svc-web-analyzer/internal/usecases"
	"github.com/architeacher/svc-web-analyzer/internal/usecases/commands"
	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

// Ensure AnalysisWorker implements the MessageHandler interface
var _ ports.MessageHandler = (*AnalysisWorker)(nil)

type AnalysisWorker struct {
	app    *usecases.SubscriberApplication
	logger infrastructure.Logger
}

func NewAnalysisWorker(
	app *usecases.SubscriberApplication,
	logger infrastructure.Logger,
) *AnalysisWorker {
	return &AnalysisWorker{
		app:    app,
		logger: logger,
	}
}

func (w *AnalysisWorker) ProcessMessage(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error {
	var payload domain.AnalysisRequestPayload
	if err := msg.Unmarshal(&payload); err != nil {
		w.logger.Error().Err(err).Msg("failed to unmarshal message payload")

		return ctrl.Reject(msg)
	}

	result, err := w.app.Commands.ProcessAnalysisMessageHandler.Handle(ctx, commands.ProcessAnalysisMessageCommand{
		Payload: payload,
	})

	if err != nil {
		w.logger.Error().Err(err).Str("analysis_id", payload.AnalysisID.String()).
			Msg("failed to process analysis message")

		return ctrl.Requeue(msg)
	}

	if !result.Success {
		w.logger.Warn().
			Str("analysis_id", payload.AnalysisID.String()).
			Str("error_code", result.ErrorCode).
			Str("error_message", result.ErrorMessage).
			Msg("analysis processing completed with error")

		return ctrl.Ack(msg)
	}

	return ctrl.Ack(msg)
}
