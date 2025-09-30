package infrastructure

import (
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/config"
	"github.com/architeacher/svc-web-analyzer/pkg/queue"
)

type Queue = queue.Queue

func NewQueue(cfg config.QueueConfig, logger Logger) (Queue, error) {
	queueConfig := queue.Config{
		Scheme:   "amqp",
		Username: cfg.Username,
		Password: cfg.Password,
		Host:     cfg.Host,
		Port:     cfg.Port,
		Vhost:    cfg.VirtualHost,
	}

	return queue.NewRabbitMQQueue(
		queueConfig,
		queue.WithLogger(queue.NewLoggerAdapter(logger)),
		queue.WithReconnectDelay(5*time.Second),
	), nil
}
