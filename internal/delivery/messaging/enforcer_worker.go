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

type EnforcerPayload struct {
	BotID          uuid.UUID `json:"bot_id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	TelegramChatID int64     `json:"telegram_chat_id"`
}

type EnforcerWorker struct {
	db         *gorm.DB
	subRepo    repository.ISubscriptionRepository
	groupRepo  repository.ITelegramGroupRepository
	outboxRepo repository.IOutboxRepository
	log        *zap.Logger
}

const expiredSubscriptionBatchSize = 200

func NewEnforcerWorker(
	db *gorm.DB,
	subRepo repository.ISubscriptionRepository,
	groupRepo repository.ITelegramGroupRepository,
	outboxRepo repository.IOutboxRepository,
	log *zap.Logger,
) *EnforcerWorker {
	return &EnforcerWorker{
		db:         db,
		subRepo:    subRepo,
		groupRepo:  groupRepo,
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
	// Resolve target groups BEFORE the transaction — groupRepo query runs outside tx
	// to keep the transaction short and avoid lock contention.
	if sub.Package.ID == uuid.Nil || sub.User.ID == uuid.Nil {
		w.log.Warn("enforcer worker: subscription missing package or user relation, skipping",
			zap.String("sub_id", sub.ID.String()),
		)
		return
	}

	var targetGroups []entity.Group
	if sub.Package.IsAllAccess {
		// IsAllAccess: kick from ALL active groups belonging to this client
		groups, err := w.groupRepo.FindByClientID(ctx, w.db, sub.Package.ClientID)
		if err != nil {
			w.log.Error("enforcer worker: failed to fetch groups for all-access package",
				zap.String("sub_id", sub.ID.String()),
				zap.String("client_id", sub.Package.ClientID.String()),
				zap.Error(err),
			)
			return
		}
		targetGroups = groups
		w.log.Info("enforcer worker: all-access package — resolved groups from client",
			zap.String("sub_id", sub.ID.String()),
			zap.Int("group_count", len(targetGroups)),
		)
	} else {
		// Regular package: use the groups directly associated via package_groups
		targetGroups = sub.Package.Groups
	}

	if len(targetGroups) == 0 {
		w.log.Warn("enforcer worker: no target groups found for subscription, marking expired anyway",
			zap.String("sub_id", sub.ID.String()),
		)
	}

	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Update status to expired (optimistic lock: only if still 'active')
		result := tx.Model(&entity.Subscription{}).
			Where("id = ? AND status = ?", sub.ID, "active").
			Updates(map[string]any{
				"status":     "expired",
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			})

		if result.Error != nil {
			return result.Error
		}

		// RowsAffected == 0: another worker already processed this (race condition)
		if result.RowsAffected == 0 {
			w.log.Debug("enforcer worker: subscription already processed by another worker, skipping",
				zap.String("sub_id", sub.ID.String()),
			)
			return nil
		}

		// Publish eviction task for each target group
		for _, group := range targetGroups {
			payload := EnforcerPayload{
				BotID:          group.BotID,
				TelegramUserID: sub.User.TelegramUserID,
				TelegramChatID: group.TelegramChatID,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("enforcer worker: failed to marshal payload for group %s: %w", group.ID, err)
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
				return fmt.Errorf("enforcer worker: failed to create outbox for group %s: %w", group.ID, err)
			}
		}

		return nil
	})

	if err != nil {
		w.log.Error("enforcer worker: failed to process expired subscription",
			zap.String("sub_id", sub.ID.String()),
			zap.Error(err),
		)
		return
	}
	w.log.Info("enforcer worker: successfully marked subscription as expired and queued evictions",
		zap.String("sub_id", sub.ID.String()),
		zap.Int("groups_evicted", len(targetGroups)),
	)
}
