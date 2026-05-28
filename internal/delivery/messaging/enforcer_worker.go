package messaging

import (
	"context"
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

type EnforcerPayload struct {
	BotID          uuid.UUID `json:"bot_id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	TelegramChatID int64     `json:"telegram_chat_id"`
}

type EnforcerWorker struct {
	db         *gorm.DB
	subRepo    repository.ISubscriptionRepository
	outboxRepo repository.IOutboxRepository
	log        *zap.Logger
}

const expiredSubscriptionBatchSize = 200

func NewEnforcerWorker(
	db *gorm.DB,
	subRepo repository.ISubscriptionRepository,
	outboxRepo repository.IOutboxRepository,
	log *zap.Logger,
) *EnforcerWorker {
	return &EnforcerWorker{
		db:         db,
		subRepo:    subRepo,
		outboxRepo: outboxRepo,
		log:        log,
	}
}

func (w *EnforcerWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("enforcer worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("enforcer worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *EnforcerWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("enforcer").Observe(time.Since(start).Seconds())

	subs, err := w.subRepo.FindExpiredSubscriptions(ctx, w.db, expiredSubscriptionBatchSize)
	if err != nil {
		w.log.Error("enforcer worker: failed to fetch expired subscriptions", zap.Error(err))
		return
	}

	if len(subs) == 0 {
		return
	}

	w.log.Info("enforcer worker: processing expired subscriptions", zap.Int("count", len(subs)))

	for _, sub := range subs {
		w.processExpiredSubscription(ctx, &sub)
	}
}

func (w *EnforcerWorker) processExpiredSubscription(ctx context.Context, sub *entity.Subscription) {
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		// Update status to expired
		result := tx.Model(&entity.Subscription{}).
			Where("id = ? AND status = ?", sub.ID, "active").
			Updates(map[string]any{
				"status":     "expired",
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			})

		if result.Error != nil {
			return result.Error
		}

		// Jika tidak ada rows affected, berarti mungkin sudah diupdate process lain (race condition)
		if result.RowsAffected == 0 {
			return nil
		}

		// Publish eviction tasks for each group in the package
		if sub.Package.ID != uuid.Nil && sub.User.ID != uuid.Nil {
			for _, group := range sub.Package.Groups {
				payload := EnforcerPayload{
					BotID:          group.BotID,
					TelegramUserID: sub.User.TelegramUserID,
					TelegramChatID: group.TelegramChatID,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					return err
				}

				outbox := &entity.Outbox{
					ID:            uuid.New(),
					AggregateType: "subscription",
					AggregateID:   sub.ID,
					EventType:     "enforcer.kick",
					Payload:       datatypes.JSON(payloadBytes),
					Status:        "pending",
					RetryCount:    0,
					MaxRetries:    3,
					ProcessAfter:  now,
					CreatedAt:     now,
					UpdatedAt:     now,
				}

				if err := w.outboxRepo.Create(ctx, tx, outbox); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		w.log.Error("enforcer worker: failed to process expired subscription", zap.String("sub_id", sub.ID.String()), zap.Error(err))
		return
	}
	w.log.Info("enforcer worker: successfully marked subscription as expired", zap.String("sub_id", sub.ID.String()))
}
