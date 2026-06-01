package messaging

import (
	"context"
	"fmt"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ExpiryReminderPayload is the message body sent to the ExpiryReminder queue.
type ExpiryReminderPayload struct {
	SubscriptionID string `json:"subscription_id"`
	TelegramUserID int64  `json:"telegram_user_id"`
	PackageID      string `json:"package_id"`
	ClientID       string `json:"client_id"`
	// ReminderHours indicates which reminder window triggered this (e.g. 72 or 24).
	ReminderHours int `json:"reminder_hours"`
}

// reminderWindows defines how many hours before expiry each reminder is sent.
// Add or remove values here to change the schedule (e.g. []int{72, 24, 6}).
var reminderWindows = []int{72, 24}

const expiryReminderBatchSize = 200

// ExpiryReminderWorker polls for subscriptions expiring soon and publishes
// reminder events via the outbox so members receive a Telegram DM warning.
type ExpiryReminderWorker struct {
	db         *gorm.DB
	subRepo    repository.ISubscriptionRepository
	outboxRepo repository.IOutboxRepository
	log        *zap.Logger
}

func NewExpiryReminderWorker(
	db *gorm.DB,
	subRepo repository.ISubscriptionRepository,
	outboxRepo repository.IOutboxRepository,
	log *zap.Logger,
) *ExpiryReminderWorker {
	return &ExpiryReminderWorker{
		db:         db,
		subRepo:    subRepo,
		outboxRepo: outboxRepo,
		log:        log,
	}
}

func (w *ExpiryReminderWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("expiry reminder worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("expiry reminder worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *ExpiryReminderWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("expiry_reminder").Observe(time.Since(start).Seconds())

	for _, hours := range reminderWindows {
		w.processWindow(ctx, hours)
	}
}

func (w *ExpiryReminderWorker) processWindow(ctx context.Context, withinHours int) {
	subs, err := w.subRepo.FindExpiringSoon(ctx, w.db, withinHours, expiryReminderBatchSize)
	if err != nil {
		w.log.Error("expiry reminder worker: failed to fetch expiring subscriptions",
			zap.Int("window_hours", withinHours),
			zap.Error(err),
		)
		return
	}

	if len(subs) == 0 {
		return
	}

	w.log.Info("expiry reminder worker: found subscriptions to remind",
		zap.Int("window_hours", withinHours),
		zap.Int("count", len(subs)),
	)

	for i := range subs {
		w.processSubscription(ctx, &subs[i], withinHours)
	}
}

func (w *ExpiryReminderWorker) processSubscription(ctx context.Context, sub *entity.Subscription, withinHours int) {
	if sub.User.ID == uuid.Nil || sub.Package.ID == uuid.Nil {
		w.log.Warn("expiry reminder worker: subscription missing preloaded relations, skipping",
			zap.String("sub_id", sub.ID.String()),
		)
		return
	}

	eventType := fmt.Sprintf("expiry.reminder_%dh", withinHours)

	payload := ExpiryReminderPayload{
		SubscriptionID: sub.ID.String(),
		TelegramUserID: sub.User.TelegramUserID,
		PackageID:      sub.PackageID.String(),
		ClientID:       sub.ClientID.String(),
		ReminderHours:  withinHours,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		w.log.Error("expiry reminder worker: failed to marshal payload",
			zap.String("sub_id", sub.ID.String()),
			zap.Error(err),
		)
		return
	}

	now := time.Now()
	outbox := &entity.Outbox{
		ID:            uuid.New(),
		AggregateType: "subscription",
		AggregateID:   sub.ID,
		EventType:     eventType,
		Payload:       datatypes.JSON(payloadBytes),
		Status:        "pending",
		RetryCount:    0,
		MaxRetries:    3,
		ProcessAfter:  now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := w.outboxRepo.Create(ctx, w.db, outbox); err != nil {
		w.log.Error("expiry reminder worker: failed to write outbox event",
			zap.String("sub_id", sub.ID.String()),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return
	}

	w.log.Info("expiry reminder worker: queued reminder",
		zap.String("sub_id", sub.ID.String()),
		zap.Int("reminder_hours", withinHours),
		zap.Time("expires_at", sub.ExpiredAt),
	)
}
