package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rabbitmq"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// RabbitMQPublisher implements message publishing to RabbitMQ.
// Acts as an outbound adapter in Clean Architecture.
type RabbitMQPublisher struct {
	conn    *rabbitmq.Connection
	channel *amqp.Channel
	mu      sync.Mutex
	logger  *zap.Logger
}

// NewRabbitMQPublisher creates a new publisher with a dedicated channel and publisher confirms enabled.
func NewRabbitMQPublisher(conn *rabbitmq.Connection, logger *zap.Logger) (*RabbitMQPublisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("publisher: failed to open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return nil, fmt.Errorf("publisher: failed to enable confirms: %w", err)
	}

	logger.Info("publisher: initialized with publisher confirms enabled")

	return &RabbitMQPublisher{
		conn:    conn,
		channel: ch,
		logger:  logger,
	}, nil
}

// publish is the internal method that handles the actual AMQP publishing.
func (p *RabbitMQPublisher) publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	p.mu.Lock()
	if p.channel == nil || p.channel.IsClosed() {
		ch, err := p.conn.Channel()
		if err != nil {
			p.mu.Unlock()
			return fmt.Errorf("publisher: failed to recreate channel: %w", err)
		}

		if err := ch.Confirm(false); err != nil {
			ch.Close()
			p.mu.Unlock()
			return fmt.Errorf("publisher: failed to enable confirms on new channel: %w", err)
		}

		p.channel = ch
	}

	messageID := trace.MessageIDFromContext(ctx)
	if messageID == "" {
		messageID = uuid.NewString()
	}
	correlationID := trace.CorrelationIDFromContext(ctx)
	if correlationID == "" {
		correlationID = messageID
	}

	confirm, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			Timestamp:     time.Now(),
			MessageId:     messageID,
			CorrelationId: correlationID,
			Body:          body,
		},
	)
	p.mu.Unlock()

	if err != nil {
		metrics.MessagesPublished.WithLabelValues(exchange, routingKey, "error").Inc()
		return fmt.Errorf("publisher: failed to publish to %s/%s: %w", exchange, routingKey, err)
	}

	// Wait for broker confirmation
	if !confirm.Wait() {
		metrics.MessagesPublished.WithLabelValues(exchange, routingKey, "nack").Inc()
		return fmt.Errorf("publisher: message to %s/%s was nacked by broker", exchange, routingKey)
	}
	metrics.MessagesPublished.WithLabelValues(exchange, routingKey, "success").Inc()

	p.logger.Debug("publisher: message published and confirmed",
		zap.String("exchange", exchange),
		zap.String("routing_key", routingKey),
	)

	return nil
}

// PublishTelegramAction publishes a telegram action task to the telegram exchange.
func (p *RabbitMQPublisher) PublishTelegramAction(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal telegram action payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangeTelegram, rabbitmq.RoutingKeyTelegramAction, body)
}

// PublishPaymentWebhook publishes a payment webhook task to the payment exchange.
func (p *RabbitMQPublisher) PublishPaymentWebhook(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal payment webhook payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangePayment, rabbitmq.RoutingKeyPaymentWebhook, body)
}

// PublishNotification publishes a notification task to the notification exchange.
func (p *RabbitMQPublisher) PublishNotification(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal notification payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangeNotification, rabbitmq.RoutingKeyNotification, body)
}

// PublishGatekeeping publishes a gatekeeping task (join request validation).
func (p *RabbitMQPublisher) PublishGatekeeping(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal gatekeeping payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangeTelegram, rabbitmq.RoutingKeyGatekeeping, body)
}

// PublishEnforcer publishes an enforcer task (kick expired member).
func (p *RabbitMQPublisher) PublishEnforcer(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal enforcer payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangeTelegram, rabbitmq.RoutingKeyEnforcer, body)
}

// PublishExpiryReminder publishes an expiry reminder task (DM member before subscription expires).
func (p *RabbitMQPublisher) PublishExpiryReminder(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal expiry reminder payload: %w", err)
	}
	return p.publish(ctx, rabbitmq.ExchangeTelegram, rabbitmq.RoutingKeyExpiryReminder, body)
}

// Close closes the publisher's dedicated channel.
func (p *RabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel != nil && !p.channel.IsClosed() {
		p.logger.Info("publisher: closing channel")
		return p.channel.Close()
	}
	return nil
}
