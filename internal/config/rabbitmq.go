package config

import (
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rabbitmq"
	"go.uber.org/zap"
)

// NewRabbitMQ initializes a RabbitMQ connection and declares the default topology.
func NewRabbitMQ(cfg *RabbitMQConfig, logger *zap.Logger) (*rabbitmq.Connection, error) {
	// Create connection with auto-reconnect
	conn, err := rabbitmq.NewConnection(
		cfg.DSN(),
		logger,
		rabbitmq.WithReconnectDelay(cfg.ReconnectDelay),
		rabbitmq.WithMaxRetries(cfg.MaxRetries),
	)
	if err != nil {
		return nil, fmt.Errorf("config: failed to connect to rabbitmq: %w", err)
	}

	// Declare topology (exchanges, queues, bindings)
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("config: failed to open channel for topology: %w", err)
	}
	defer ch.Close()

	topology := rabbitmq.DefaultTopology()
	if err := rabbitmq.DeclareTopology(ch, topology, logger); err != nil {
		conn.Close()
		return nil, fmt.Errorf("config: failed to declare topology: %w", err)
	}

	logger.Info("config: rabbitmq topology declared successfully")

	return conn, nil
}
