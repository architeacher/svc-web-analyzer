package queue

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// Config is used to establish a connection with a RabbitMQ server.
type Config struct {
	Scheme   string
	Username string
	Password string
	Host     string
	Port     int
	Vhost    string
}

func getURL(cfg Config) string {
	uri := amqp.URI{
		Scheme:   cfg.Scheme,
		Username: cfg.Username,
		Password: cfg.Password,
		Host:     cfg.Host,
		Port:     cfg.Port,
		Vhost:    cfg.Vhost,
	}

	return uri.String()
}