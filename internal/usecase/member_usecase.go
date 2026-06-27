package usecase

import (
	"context"
	"time"

	"github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IMemberUseCase defines the contract for member (telegram_users + subscriptions) operations.
type IMemberUseCase interface {
	// FindAll returns a paginated list of members with their primary subscription.
	FindAll(ctx context.Context, clientID uuid.UUID, filter model.MemberFilterRequest) ([]model.MemberResponse, int64, error)

	// FindByID returns the full detail of a single member including all subscription history.
	FindByID(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) (*model.MemberDetailResponse, error)

	// KickMember cancels active subscription and sends an outbox event to kick the user from Telegram groups.
	KickMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error

	// ExtendMember updates the active subscription expiry time.
	ExtendMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, newExpiryAt time.Time) error

	// SyncMember creates a sync_request outbox event.
	SyncMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error

	// ResendLink creates a resend_link outbox event for the user's active subscriptions.
	ResendLink(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error
}

type memberUseCase struct {
	db               *entity.Database
	tgUserRepo       repository.ITelegramUserRepository
	subscriptionRepo repository.ISubscriptionRepository
	outboxRepo       repository.IOutboxRepository
	auditLogRepo     repository.IAuditLogRepository
	log              *zap.Logger
}

func NewMemberUseCase(
	db *entity.Database,
	tgUserRepo repository.ITelegramUserRepository,
	subscriptionRepo repository.ISubscriptionRepository,
	outboxRepo repository.IOutboxRepository,
	auditLogRepo repository.IAuditLogRepository,
	log *zap.Logger,
) IMemberUseCase {
	return &memberUseCase{
		db:               db,
		tgUserRepo:       tgUserRepo,
		subscriptionRepo: subscriptionRepo,
		outboxRepo:       outboxRepo,
		auditLogRepo:     auditLogRepo,
		log:              log,
	}
}

// FindAll fetches paginated members with their primary subscription and order counts.
func (uc *memberUseCase) FindAll(ctx context.Context, clientID uuid.UUID, filter model.MemberFilterRequest) ([]model.MemberResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase FindAll start", zap.String("client_id", clientID.String()))

	// 1. Fetch paginated telegram users with preloaded subscriptions
	users, err := uc.tgUserRepo.FindMembersByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("member usecase FindAll fetch failed", zap.Error(err))
		return nil, 0, err
	}

	// 2. Count total matching members
	total, err := uc.tgUserRepo.CountMembersByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("member usecase FindAll count failed", zap.Error(err))
		return nil, 0, err
	}

	// 3. Batch fetch order counts for the returned users
	userIDs := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}

	orderCounts, err := uc.tgUserRepo.CountOrdersByUserIDs(ctx, uc.db.Gorm, userIDs, clientID)
	if err != nil {
		log.Error("member usecase FindAll order counts failed", zap.Error(err))
		return nil, 0, err
	}

	// 4. Convert uuid-keyed map to string-keyed map for the converter
	stringCounts := make(map[string]int64, len(orderCounts))
	for id, count := range orderCounts {
		stringCounts[id.String()] = count
	}

	return converter.MembersToResponse(users, stringCounts), total, nil
}

// FindByID fetches a single member's full detail including all subscription history.
func (uc *memberUseCase) FindByID(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) (*model.MemberDetailResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase FindByID start", zap.String("user_id", userID.String()))

	user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, uc.db.Gorm, userID, clientID)
	if err != nil {
		log.Error("member usecase FindByID fetch failed", zap.Error(err))
		return nil, err
	}
	if user == nil {
		return nil, helper.NewNotFound("Member")
	}

	// Fetch order count for this single user
	orderCounts, err := uc.tgUserRepo.CountOrdersByUserIDs(ctx, uc.db.Gorm, []uuid.UUID{userID}, clientID)
	if err != nil {
		log.Error("member usecase FindByID order count failed", zap.Error(err))
		return nil, err
	}

	totalOrders := orderCounts[userID]

	return converter.MemberDetailToResponse(user, totalOrders), nil
}

// KickMember cancels the active subscription and queues an outbox event.
func (uc *memberUseCase) KickMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase KickMember start", zap.String("user_id", userID.String()))

	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Find user to get telegram_user_id
		user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, tx, userID, clientID)
		if err != nil {
			return err
		}
		if user == nil {
			return helper.NewNotFound("Member")
		}

		// 2. Find active subscriptions for this user & client
		subs, err := uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, tx, user.TelegramUserID, clientID)
		if err != nil {
			return err
		}
		if len(subs) == 0 {
			return helper.NewBadRequest("Member tidak memiliki langganan aktif")
		}

		now := time.Now()
		for i := range subs {
			sub := &subs[i]
			// Update subscription status
			sub.Status = "cancelled"
			sub.KickedAt = &now
			if err := uc.subscriptionRepo.Update(ctx, tx, sub); err != nil {
				return err
			}

			// For each group in the package, create an outbox event
			if sub.Package.ID != uuid.Nil {
				for _, group := range sub.Package.Groups {
					payload := map[string]interface{}{
						"telegram_user_id": user.TelegramUserID,
						"telegram_chat_id": group.TelegramChatID,
						"reason":           "manual_kick",
					}
					payloadBytes, _ := sonic.Marshal(payload)

					outbox := entity.Outbox{
						AggregateType: "subscription",
						AggregateID:   sub.ID,
						EventType:     "member.kick",
						Payload:       datatypes.JSON(payloadBytes),
						Status:        "pending",
						ProcessAfter:  now,
						RetryCount:    0,
					}
					if err := uc.outboxRepo.Create(ctx, tx, &outbox); err != nil {
						return err
					}
				}
			}
		}

		auditLogMeta, _ := sonic.Marshal(map[string]interface{}{"reason": "manual_kick"})
		auditLog := entity.AuditLog{
			ClientID:   &clientID,
			EntityType: "member",
			EntityID:   userID,
			Action:     "kick_member",
			ActorType:  "system", // Ideally user ID from ctx
			ActorID:    "system",
			Metadata:   datatypes.JSON(auditLogMeta),
		}
		if err := uc.auditLogRepo.Create(ctx, tx, &auditLog); err != nil {
			log.Warn("failed to create audit log for kick", zap.Error(err))
		}

		return nil
	})
}

// ExtendMember updates the expiry date of an active subscription.
func (uc *memberUseCase) ExtendMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, newExpiryAt time.Time) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase ExtendMember start", zap.String("user_id", userID.String()))

	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, tx, userID, clientID)
		if err != nil {
			return err
		}
		if user == nil {
			return helper.NewNotFound("Member")
		}

		subs, err := uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, tx, user.TelegramUserID, clientID)
		if err != nil {
			return err
		}
		if len(subs) == 0 {
			return helper.NewBadRequest("Member tidak memiliki langganan aktif")
		}

		// Update expiry on all active subscriptions for this client
		for i := range subs {
			sub := &subs[i]
			oldExpiry := sub.ExpiredAt

			if newExpiryAt.Before(oldExpiry) {
				return helper.NewBadRequest("Tanggal kadaluarsa baru tidak boleh kurang dari tanggal kadaluarsa saat ini")
			}

			sub.ExpiredAt = newExpiryAt
			if err := uc.subscriptionRepo.Update(ctx, tx, sub); err != nil {
				return err
			}

			auditLogMeta, _ := sonic.Marshal(map[string]interface{}{
				"old_expiry": oldExpiry.Format(time.RFC3339),
				"new_expiry": newExpiryAt.Format(time.RFC3339),
			})
			auditLog := entity.AuditLog{
				ClientID:   &clientID,
				EntityType: "subscription",
				EntityID:   sub.ID,
				Action:     "extend_expiry",
				ActorType:  "system",
				ActorID:    "system",
				Metadata:   datatypes.JSON(auditLogMeta),
			}
			if err := uc.auditLogRepo.Create(ctx, tx, &auditLog); err != nil {
				log.Warn("failed to create audit log for extend", zap.Error(err))
			}
		}
		return nil
	})
}

// SyncMember creates a sync_request outbox event.
func (uc *memberUseCase) SyncMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase SyncMember start", zap.String("user_id", userID.String()))

	user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, uc.db.Gorm, userID, clientID)
	if err != nil {
		return err
	}
	if user == nil {
		return helper.NewNotFound("Member")
	}

	subs, err := uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, uc.db.Gorm, user.TelegramUserID, clientID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return helper.NewBadRequest("Member tidak memiliki langganan aktif")
	}

	now := time.Now()
	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range subs {
			sub := &subs[i]
			if sub.Package.ID != uuid.Nil {
				for _, group := range sub.Package.Groups {
					payload := map[string]interface{}{
						"telegram_user_id": user.TelegramUserID,
						"telegram_chat_id": group.TelegramChatID,
					}
					payloadBytes, _ := sonic.Marshal(payload)

					outbox := entity.Outbox{
						AggregateType: "member",
						AggregateID:   userID,
						EventType:     "member.sync_request",
						Payload:       datatypes.JSON(payloadBytes),
						Status:        "pending",
						ProcessAfter:  now,
						RetryCount:    0,
					}
					if err := uc.outboxRepo.Create(ctx, tx, &outbox); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

// ResendLink creates a resend_link outbox event for the user's active subscriptions.
func (uc *memberUseCase) ResendLink(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase ResendLink start", zap.String("user_id", userID.String()))

	user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, uc.db.Gorm, userID, clientID)
	if err != nil {
		return err
	}
	if user == nil {
		return helper.NewNotFound("Member")
	}

	subs, err := uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, uc.db.Gorm, user.TelegramUserID, clientID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return helper.NewBadRequest("Member tidak memiliki langganan aktif")
	}

	now := time.Now()
	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range subs {
			sub := &subs[i]
			payload := map[string]interface{}{
				"telegram_user_id": user.TelegramUserID,
				"subscription_id":  sub.ID,
				"package_id":       sub.PackageID,
				"client_id":        clientID,
			}
			payloadBytes, _ := sonic.Marshal(payload)

			outbox := entity.Outbox{
				AggregateType: "member",
				AggregateID:   userID,
				EventType:     "member.resend_link",
				Payload:       datatypes.JSON(payloadBytes),
				Status:        "pending",
				ProcessAfter:  now,
				RetryCount:    0,
			}
			if err := uc.outboxRepo.Create(ctx, tx, &outbox); err != nil {
				return err
			}
		}
		return nil
	})
}
