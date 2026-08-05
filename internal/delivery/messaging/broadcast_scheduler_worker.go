package messaging

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BroadcastSchedulerWorker struct {
	db          *gorm.DB
	broadcastUC usecase.IBroadcastUseCase
	log         *zap.Logger
}

func NewBroadcastSchedulerWorker(
	db *gorm.DB,
	broadcastUC usecase.IBroadcastUseCase,
	log *zap.Logger,
) *BroadcastSchedulerWorker {
	return &BroadcastSchedulerWorker{
		db:          db,
		broadcastUC: broadcastUC,
		log:         log,
	}
}

func (w *BroadcastSchedulerWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("broadcast scheduler worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("broadcast scheduler worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *BroadcastSchedulerWorker) Process(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("broadcast scheduler worker: panicked", zap.Any("panic", r))
		}
	}()
	start := time.Now()
	defer func() {
		metrics.WorkerCycleDuration.WithLabelValues("broadcast_scheduler").Observe(time.Since(start).Seconds())
	}()

	var scheduledIDs []uuid.UUID

	// 1. Lock scheduled broadcasts and mark them as "distributing" in a short transaction
	err := w.db.Transaction(func(tx *gorm.DB) error {
		var broadcasts []entity.Broadcast
		now := time.Now().UTC()

		err := tx.Session(&gorm.Session{PrepareStmt: false}).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND scheduled_at <= ? AND deleted_at IS NULL", "scheduled", now).
			Find(&broadcasts).Error
		if err != nil {
			return err
		}

		for _, b := range broadcasts {
			b.Status = "distributing"
			b.UpdatedAt = time.Now()
			if err := tx.Save(&b).Error; err != nil {
				return err
			}
			scheduledIDs = append(scheduledIDs, b.ID)
		}

		return nil
	})

	if err != nil {
		w.log.Error("broadcast scheduler worker: failed to fetch scheduled broadcasts", zap.Error(err))
		return
	}

	if len(scheduledIDs) == 0 {
		return
	}

	w.log.Info("broadcast scheduler worker: distributing scheduled broadcasts", zap.Int("count", len(scheduledIDs)))

	// 2. Process each broadcast distribution using the UseCase
	for _, id := range scheduledIDs {
		w.log.Info("broadcast scheduler worker: processing broadcast", zap.String("id", id.String()))
		if err := w.broadcastUC.DistributeScheduled(ctx, id); err != nil {
			w.log.Error("broadcast scheduler worker: failed to distribute broadcast", zap.String("id", id.String()), zap.Error(err))
		} else {
			w.log.Info("broadcast scheduler worker: successfully distributed broadcast", zap.String("id", id.String()))
		}
	}
}
