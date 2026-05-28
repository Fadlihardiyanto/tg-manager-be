package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rabbitmq"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Handler is a function that processes a delivered message.
// Return nil to Ack, return error to Nack (message goes to DLQ).
type Handler func(ctx context.Context, body []byte) error

// MessageConsumer dispatches RabbitMQ messages to registered handlers.
// Acts as an inbound adapter in Clean Architecture (like HTTP controllers).
type MessageConsumer struct {
	conn          *rabbitmq.Connection
	logger        *zap.Logger
	prefetchCount int
	handlers      map[string]Handler
	wg            sync.WaitGroup
	cancelFunc    context.CancelFunc
}

const maxDeliveryRetries = 3

// NewMessageConsumer creates a new consumer dispatcher.
func NewMessageConsumer(conn *rabbitmq.Connection, prefetchCount int, logger *zap.Logger) *MessageConsumer {
	return &MessageConsumer{
		conn:          conn,
		logger:        logger,
		prefetchCount: prefetchCount,
		handlers:      make(map[string]Handler),
	}
}

// RegisterHandler registers a handler for a specific queue.
// Call this before Start().
func (mc *MessageConsumer) RegisterHandler(queue string, handler Handler) {
	mc.handlers[queue] = handler
	mc.logger.Info("consumer: handler registered", zap.String("queue", queue))
}

// Start begins consuming from all registered queues.
// Blocks until the context is cancelled.
func (mc *MessageConsumer) Start(ctx context.Context) error {
	if len(mc.handlers) == 0 {
		return fmt.Errorf("consumer: no handlers registered")
	}

	ctx, cancel := context.WithCancel(ctx)
	mc.cancelFunc = cancel

	for queue, handler := range mc.handlers {
		mc.wg.Add(1)
		go mc.consume(ctx, queue, handler)
	}

	mc.logger.Info("consumer: started consuming", zap.Int("queues", len(mc.handlers)))

	// Wait for context cancellation
	<-ctx.Done()

	mc.logger.Info("consumer: shutting down, waiting for handlers to finish...")
	mc.wg.Wait()
	mc.logger.Info("consumer: shutdown complete")

	return nil
}

// consume listens to a single queue and dispatches messages to the handler.
func (mc *MessageConsumer) consume(ctx context.Context, queue string, handler Handler) {
	defer mc.wg.Done()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ch, err := mc.conn.Channel()
		if err != nil {
			mc.logger.Error("consumer: failed to open channel",
				zap.String("queue", queue),
				zap.Duration("retry_in", backoff),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff = nextBackoff(backoff, maxBackoff)
				continue
			}
		}

		if err := ch.Qos(mc.prefetchCount, 0, false); err != nil {
			mc.logger.Error("consumer: failed to set QoS",
				zap.String("queue", queue),
				zap.Error(err),
			)
			ch.Close()
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff = nextBackoff(backoff, maxBackoff)
				continue
			}
		}

		deliveries, err := ch.Consume(
			queue,
			"",    // consumer tag (auto-generated)
			false, // auto-ack disabled (manual ack)
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,
		)
		if err != nil {
			mc.logger.Error("consumer: failed to start consuming",
				zap.String("queue", queue),
				zap.Error(err),
			)
			ch.Close()
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff = nextBackoff(backoff, maxBackoff)
				continue
			}
		}

		backoff = time.Second

		mc.logger.Info("consumer: listening for messages", zap.String("queue", queue))

		// Process deliveries until channel closes or context is cancelled
		mc.processDeliveries(ctx, queue, deliveries, handler)

		ch.Close()

		if ctx.Err() != nil {
			return
		}

		mc.logger.Warn("consumer: channel closed, reconnecting...", zap.String("queue", queue))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff = nextBackoff(backoff, maxBackoff)
		}
	}
}

// processDeliveries handles individual message processing with Ack/Nack.
func (mc *MessageConsumer) processDeliveries(ctx context.Context, queue string, deliveries <-chan amqp.Delivery, handler Handler) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return // channel closed
			}

			if ctx.Err() != nil {
				d.Nack(false, true)
				return
			}

			retryCount := getRetryCount(d.Headers)
			if retryCount >= maxDeliveryRetries {
				mc.logger.Error("consumer: max retries exceeded, dropping message",
					zap.String("queue", queue),
					zap.Int("retries", retryCount),
					zap.String("routing_key", d.RoutingKey),
					zap.String("message_id", d.MessageId),
					zap.String("correlation_id", d.CorrelationId),
				)
				metrics.MessagesConsumed.WithLabelValues(queue, "dropped").Inc()
				d.Ack(false)
				continue
			}

			msgCtx := trace.WithMessageID(ctx, d.MessageId)
			msgCtx = trace.WithCorrelationID(msgCtx, d.CorrelationId)
			start := time.Now()
			err := handler(msgCtx, d.Body)
			metrics.MessageProcessingDuration.WithLabelValues(queue).Observe(time.Since(start).Seconds())
			if err != nil {
				mc.logger.Warn("consumer: handler failed, sending to DLQ",
					zap.String("queue", queue),
					zap.String("routing_key", d.RoutingKey),
					zap.Bool("redelivered", d.Redelivered),
					zap.String("message_id", d.MessageId),
					zap.String("correlation_id", d.CorrelationId),
					zap.Error(err),
				)

				// Nack without requeue → message goes to DLQ via DLX
				metrics.MessagesConsumed.WithLabelValues(queue, "error").Inc()
				d.Nack(false, false)
			} else {
				metrics.MessagesConsumed.WithLabelValues(queue, "success").Inc()
				d.Ack(false)
			}
		}
	}
}

// Stop gracefully stops all consumers.
func (mc *MessageConsumer) Stop() {
	if mc.cancelFunc != nil {
		mc.cancelFunc()
	}
}

func getRetryCount(headers amqp.Table) int {
	xDeath, ok := headers["x-death"]
	if !ok {
		return 0
	}

	deaths, ok := xDeath.([]interface{})
	if !ok || len(deaths) == 0 {
		return 0
	}

	first, ok := deaths[0].(amqp.Table)
	if !ok {
		return 0
	}

	count, ok := first["count"]
	if !ok {
		return 0
	}

	switch v := count.(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
