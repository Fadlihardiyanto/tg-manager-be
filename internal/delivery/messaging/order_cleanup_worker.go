package messaging

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OrderCleanupWorker struct {
	db         *gorm.DB
	orderRepo  repository.IOrderRepository
	discountUC usecase.IMemberDiscountUseCase
	log        *zap.Logger
}

func NewOrderCleanupWorker(
	db *gorm.DB,
	orderRepo repository.IOrderRepository,
	discountUC usecase.IMemberDiscountUseCase,
	log *zap.Logger,
) *OrderCleanupWorker {
	return &OrderCleanupWorker{
		db:         db,
		orderRepo:  orderRepo,
		discountUC: discountUC,
		log:        log,
	}
}

func (w *OrderCleanupWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("order cleanup worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("order cleanup worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *OrderCleanupWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("order_cleanup").Observe(time.Since(start).Seconds())

	orders, err := w.orderRepo.FindExpiredPendingOrders(ctx, w.db, 50)
	if err != nil {
		w.log.Error("order cleanup worker: failed to fetch expired orders", zap.Error(err))
		return
	}

	if len(orders) == 0 {
		return
	}

	w.log.Info("order cleanup worker: processing expired orders", zap.Int("count", len(orders)))

	for _, order := range orders {
		w.processExpiredOrder(ctx, &order)
	}
}

func (w *OrderCleanupWorker) processExpiredOrder(ctx context.Context, order *entity.Order) {
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&entity.Order{}).
			Where("id = ? AND status = ?", order.ID, "pending").
			Updates(map[string]any{
				"status":     "expired",
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return nil
		}

		if order.DiscountID != nil {
			if err := w.discountUC.RollbackUsage(ctx, tx, order.ID, *order.DiscountID); err != nil {
				w.log.Error("order cleanup worker: failed to rollback discount usage", zap.Error(err))
				return err
			}
			w.log.Info("order cleanup worker: discount usage rolled back", zap.String("order_id", order.ID.String()))
		}
		return nil
	})

	if err != nil {
		w.log.Error("order cleanup worker: failed to process expired order", zap.String("order_id", order.ID.String()), zap.Error(err))
	} else {
		w.log.Info("order cleanup worker: successfully expired order", zap.String("order_id", order.ID.String()))
	}
}
