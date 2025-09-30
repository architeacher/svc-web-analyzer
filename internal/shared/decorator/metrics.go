package decorator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/ports"
)

type (
	commandMetricsDecorator[C Command, R any] struct {
		base   CommandHandler[C, R]
		client ports.MetricsClient
	}

	queryMetricsDecorator[Q Query, R Result] struct {
		base   QueryHandler[Q, R]
		client ports.MetricsClient
	}
)

func (d commandMetricsDecorator[C, R]) Handle(ctx context.Context, cmd C) (result R, err error) {
	start := time.Now()

	actionName := strings.ToLower(generateActionName(cmd))

	defer func() {
		end := time.Since(start)

		d.client.Inc(ctx, fmt.Sprintf("commands.%s.duration", actionName), int64(end.Seconds()))

		if err == nil {
			d.client.Inc(ctx, fmt.Sprintf("commands.%s.success", actionName), 1)
		} else {
			d.client.Inc(ctx, fmt.Sprintf("commands.%s.failure", actionName), 1)
		}
	}()

	return d.base.Handle(ctx, cmd)
}

func (d queryMetricsDecorator[Q, R]) Execute(ctx context.Context, query Q) (result R, err error) {
	start := time.Now()

	actionName := strings.ToLower(generateActionName(query))

	defer func() {
		end := time.Since(start)

		d.client.Inc(ctx, fmt.Sprintf("querys.%s.duration", actionName), int(end.Seconds()))

		if err == nil {
			d.client.Inc(ctx, fmt.Sprintf("querys.%s.success", actionName), 1)
		} else {
			d.client.Inc(ctx, fmt.Sprintf("querys.%s.failure", actionName), 1)
		}
	}()

	return d.base.Execute(ctx, query)
}
