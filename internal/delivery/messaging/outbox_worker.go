package messaging

import (
	"context"
	"fmt"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	gatewayMsg "github.com/Fadlihardiyanto/telegram-management-app/internal/gateway/messaging"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OutboxWorker struct {
	db         *entity.Database
	outboxRepo repository.IOutboxRepository
	publisher  *gatewayMsg.RabbitMQPublisher
	log        *zap.Logger
}

func NewOutboxWorker(
	db *entity.Database,
	outboxRepo repository.IOutboxRepository,
	publisher *gatewayMsg.RabbitMQPublisher,
	log *zap.Logger,
) *OutboxWorker {
	return &OutboxWorker{
		db:         db,
		outboxRepo: outboxRepo,
		publisher:  publisher,
		log:        log,
	}
}

func (w *OutboxWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("outbox worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("outbox worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *OutboxWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("outbox").Observe(time.Since(start).Seconds())

	events, err := w.outboxRepo.FindPending(ctx, w.db.Gorm, 50)
	if err != nil {
		w.log.Error("outbox worker: failed to fetch pending events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	w.log.Info("outbox worker: processing pending events", zap.Int("count", len(events)))

	for _, event := range events {
		w.processEvent(ctx, &event)
	}
}

func (w *OutboxWorker) processEvent(ctx context.Context, event *entity.Outbox) {
	// 1. Unmarshal payload to map[string]interface{} to prevent double JSON encoding in publisher
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
		w.log.Error("outbox worker: failed to unmarshal payload", zap.String("event_id", event.ID.String()), zap.Error(err))
		w.markAsFailed(ctx, event, fmt.Sprintf("unmarshal payload failed: %s", err.Error()))
		return
	}

	var pubErr error

	// 2. Publish to RabbitMQ based on event type
	switch event.EventType {
	case "subscription.activated", "subscription.expired", "subscription.cancelled":
		if isHighPriority(payloadMap) {
			pubErr = w.publisher.PublishTelegramActionHigh(ctx, payloadMap)
		} else {
			pubErr = w.publisher.PublishTelegramAction(ctx, payloadMap)
		}
	case "notification.send":
		pubErr = w.publisher.PublishNotification(ctx, payloadMap)
	case "enforcer.kick":
		pubErr = w.publisher.PublishEnforcer(ctx, payloadMap)
	case "expiry.reminder_72h", "expiry.reminder_24h":
		pubErr = w.publisher.PublishExpiryReminder(ctx, payloadMap)
	case "broadcast.send":
		pubErr = w.publisher.PublishBroadcast(ctx, payloadMap)
	default:
		// Default to telegram action for compatibility
		if isHighPriority(payloadMap) {
			pubErr = w.publisher.PublishTelegramActionHigh(ctx, payloadMap)
		} else {
			pubErr = w.publisher.PublishTelegramAction(ctx, payloadMap)
		}
	}

	// 3. Update status in DB based on publish result
	if pubErr != nil {
		w.log.Error("outbox worker: failed to publish event", zap.String("event_id", event.ID.String()), zap.Error(pubErr))
		w.markAsFailed(ctx, event, pubErr.Error())
	} else {
		w.markAsProcessed(ctx, event)
	}
}

func (w *OutboxWorker) markAsProcessed(ctx context.Context, event *entity.Outbox) {
	result := w.db.Gorm.WithContext(ctx).
		Model(&entity.Outbox{}).
		Where("id = ? AND status = ?", event.ID, "pending").
		Updates(map[string]any{
			"status":       "processed",
			"processed_at": gorm.Expr("CURRENT_TIMESTAMP"),
			"updated_at":   gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		w.log.Error("outbox worker: failed to update event to processed status", zap.String("event_id", event.ID.String()), zap.Error(result.Error))
		return
	}
	if result.RowsAffected == 0 {
		w.log.Warn("outbox worker: event already handled by another worker", zap.String("event_id", event.ID.String()))
		return
	}

	w.log.Info("outbox worker: event processed successfully", zap.String("event_id", event.ID.String()))
	metrics.OutboxEventsProcessed.WithLabelValues("processed").Inc()
}

func (w *OutboxWorker) markAsFailed(ctx context.Context, event *entity.Outbox, errMsg string) {
	now := time.Now()
	retryCount := event.RetryCount + 1

	// Exponential backoff for retry (1m, 2m, 4m, 8m ...)
	backoffDuration := time.Duration(1<<retryCount) * time.Minute
	if backoffDuration > 30*time.Minute {
		backoffDuration = 30 * time.Minute
	}
	processAfter := now.Add(backoffDuration)

	status := "pending"
	if retryCount >= event.MaxRetries {
		status = "failed"
		w.log.Warn("outbox worker: event reached max retries, marked as failed", zap.String("event_id", event.ID.String()))
	}

	result := w.db.Gorm.WithContext(ctx).
		Model(&entity.Outbox{}).
		Where("id = ? AND status = ?", event.ID, "pending").
		Updates(map[string]any{
			"status":        status,
			"retry_count":   retryCount,
			"last_error":    errMsg,
			"process_after": processAfter,
			"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		w.log.Error("outbox worker: failed to update event to failed status", zap.String("event_id", event.ID.String()), zap.Error(result.Error))
		return
	}
	if result.RowsAffected == 0 {
		w.log.Warn("outbox worker: event already handled by another worker", zap.String("event_id", event.ID.String()))
		return
	}

	metrics.OutboxEventsProcessed.WithLabelValues(status).Inc()
}

func isHighPriority(payload map[string]interface{}) bool {
	if hp, ok := payload["high_priority"]; ok {
		if v, ok := hp.(bool); ok {
			return v
		}
	}
	return false
}
