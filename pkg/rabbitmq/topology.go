package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// DeclareTopology declares all exchanges, queues, and bindings idempotently.
func DeclareTopology(ch *amqp.Channel, topology TopologyConfig, logger *zap.Logger) error {
	// Declare exchanges
	for _, ex := range topology.Exchanges {
		if err := ch.ExchangeDeclare(
			ex.Name,
			ex.Kind,
			ex.Durable,
			ex.AutoDelete,
			ex.Internal,
			ex.NoWait,
			ex.Args,
		); err != nil {
			return fmt.Errorf("rabbitmq: failed to declare exchange %q: %w", ex.Name, err)
		}
		logger.Debug("rabbitmq: exchange declared", zap.String("exchange", ex.Name))
	}

	// Declare queues
	for _, q := range topology.Queues {
		if _, err := ch.QueueDeclare(
			q.Name,
			q.Durable,
			q.AutoDelete,
			q.Exclusive,
			q.NoWait,
			q.Args,
		); err != nil {
			return fmt.Errorf("rabbitmq: failed to declare queue %q: %w", q.Name, err)
		}
		logger.Debug("rabbitmq: queue declared", zap.String("queue", q.Name))
	}

	// Declare bindings
	for _, b := range topology.Bindings {
		if err := ch.QueueBind(
			b.QueueName,
			b.RoutingKey,
			b.ExchangeName,
			b.NoWait,
			b.Args,
		); err != nil {
			return fmt.Errorf("rabbitmq: failed to bind queue %q to exchange %q: %w",
				b.QueueName, b.ExchangeName, err)
		}
		logger.Debug("rabbitmq: queue bound to exchange",
			zap.String("queue", b.QueueName),
			zap.String("exchange", b.ExchangeName),
			zap.String("routing_key", b.RoutingKey),
		)
	}

	return nil
}

// DefaultTopology returns the default topology for TG-Manager.
// Includes DLQ with TTL-based auto-retry for telegram_action and payment_webhook.
func DefaultTopology() TopologyConfig {
	return TopologyConfig{
		Exchanges: []ExchangeConfig{
			{Name: ExchangeTelegram, Kind: "direct", Durable: true},
			{Name: ExchangePayment, Kind: "direct", Durable: true},
			{Name: ExchangeNotification, Kind: "direct", Durable: true},
			{Name: ExchangeDLX, Kind: "direct", Durable: true},
		},
		Queues: []QueueConfig{
			// Main queues with DLX binding
			{
				Name:    QueueTelegramAction,
				Durable: true,
				Args: amqp.Table{
					"x-dead-letter-exchange":    ExchangeDLX,
					"x-dead-letter-routing-key": RoutingKeyTelegramAction,
				},
			},
			{
				Name:    QueuePaymentWebhook,
				Durable: true,
				Args: amqp.Table{
					"x-dead-letter-exchange":    ExchangeDLX,
					"x-dead-letter-routing-key": RoutingKeyPaymentWebhook,
				},
			},
			{
				Name:    QueueNotification,
				Durable: true,
			},
			{
				Name:    QueueGatekeeping,
				Durable: true,
				Args: amqp.Table{
					"x-dead-letter-exchange":    ExchangeDLX,
					"x-dead-letter-routing-key": RoutingKeyGatekeeping,
				},
			},
			{
				Name:    QueueEnforcer,
				Durable: true,
				Args: amqp.Table{
					"x-dead-letter-exchange":    ExchangeDLX,
					"x-dead-letter-routing-key": RoutingKeyEnforcer,
				},
			},
			{
				Name:    QueueExpiryReminder,
				Durable: true,
				Args: amqp.Table{
					"x-dead-letter-exchange":    ExchangeDLX,
					"x-dead-letter-routing-key": RoutingKeyExpiryReminder,
				},
			},
			// DLQ queues with TTL → auto-retry back to original exchange
			{
				Name:    QueueTelegramActionDLQ,
				Durable: true,
				Args: amqp.Table{
					"x-message-ttl":             int32(60000), // 60 seconds
					"x-dead-letter-exchange":    ExchangeTelegram,
					"x-dead-letter-routing-key": RoutingKeyTelegramAction,
				},
			},
			{
				Name:    QueuePaymentWebhookDLQ,
				Durable: true,
				Args: amqp.Table{
					"x-message-ttl":             int32(60000),
					"x-dead-letter-exchange":    ExchangePayment,
					"x-dead-letter-routing-key": RoutingKeyPaymentWebhook,
				},
			},
			{
				Name:    QueueGatekeepingDLQ,
				Durable: true,
				Args: amqp.Table{
					"x-message-ttl":             int32(60000),
					"x-dead-letter-exchange":    ExchangeTelegram,
					"x-dead-letter-routing-key": RoutingKeyGatekeeping,
				},
			},
			{
				Name:    QueueEnforcerDLQ,
				Durable: true,
				Args: amqp.Table{
					"x-message-ttl":             int32(60000),
					"x-dead-letter-exchange":    ExchangeTelegram,
					"x-dead-letter-routing-key": RoutingKeyEnforcer,
				},
			},
			{
				Name:    QueueExpiryReminderDLQ,
				Durable: true,
				Args: amqp.Table{
					"x-message-ttl":             int32(60000),
					"x-dead-letter-exchange":    ExchangeTelegram,
					"x-dead-letter-routing-key": RoutingKeyExpiryReminder,
				},
			},
		},
		Bindings: []BindingConfig{
			// Main queue bindings
			{QueueName: QueueTelegramAction, RoutingKey: RoutingKeyTelegramAction, ExchangeName: ExchangeTelegram},
			{QueueName: QueuePaymentWebhook, RoutingKey: RoutingKeyPaymentWebhook, ExchangeName: ExchangePayment},
			{QueueName: QueueNotification, RoutingKey: RoutingKeyNotification, ExchangeName: ExchangeNotification},
			{QueueName: QueueGatekeeping, RoutingKey: RoutingKeyGatekeeping, ExchangeName: ExchangeTelegram},
			{QueueName: QueueEnforcer, RoutingKey: RoutingKeyEnforcer, ExchangeName: ExchangeTelegram},
			{QueueName: QueueExpiryReminder, RoutingKey: RoutingKeyExpiryReminder, ExchangeName: ExchangeTelegram},
			// DLQ bindings
			{QueueName: QueueTelegramActionDLQ, RoutingKey: RoutingKeyTelegramAction, ExchangeName: ExchangeDLX},
			{QueueName: QueuePaymentWebhookDLQ, RoutingKey: RoutingKeyPaymentWebhook, ExchangeName: ExchangeDLX},
			{QueueName: QueueGatekeepingDLQ, RoutingKey: RoutingKeyGatekeeping, ExchangeName: ExchangeDLX},
			{QueueName: QueueEnforcerDLQ, RoutingKey: RoutingKeyEnforcer, ExchangeName: ExchangeDLX},
			{QueueName: QueueExpiryReminderDLQ, RoutingKey: RoutingKeyExpiryReminder, ExchangeName: ExchangeDLX},
		},
	}
}
