package usecase

import (
	"context"
	"errors"
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

	// KickMember cancels active subscription(s) and sends outbox events to kick the user from Telegram groups.
	// If subscriptionID is non-nil, only that specific subscription is cancelled (selective kick).
	// If nil, all active subscriptions are cancelled (global kick).
	KickMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, subscriptionID *uuid.UUID) error

	// BulkKickMembers performs best-effort selective kicks for many (member, subscription)
	// pairs. Items whose subscription is missing, not owned, or already expired are
	// skipped silently (not counted, not failed).
	BulkKickMembers(ctx context.Context, clientID uuid.UUID, items []model.BulkMemberKickItem) model.BulkDeleteResult

	// ExtendMember adds N days to a specific subscription's expiry.
	ExtendMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, req *model.ExtendMemberRequest) error

	// BulkExtendMembers performs best-effort subscription extends for many
	// (member, subscription, additional_days) items. Items whose subscription is
	// missing, not owned, or not active are skipped silently (not counted, not failed).
	BulkExtendMembers(ctx context.Context, clientID uuid.UUID, items []model.BulkMemberExtendItem) model.BulkDeleteResult

	// SyncMember creates a sync_request outbox event.
	SyncMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID) error

	// ResendLink creates a resend_link outbox event for the user's active subscriptions.
	ResendLink(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, subscriptionID *uuid.UUID) error
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

// KickMember cancels active subscription(s) and queues outbox events.
// subscriptionID=nil → global kick (all active), non-nil → selective kick (single subscription).
func (uc *memberUseCase) KickMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, subscriptionID *uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase KickMember start", zap.String("user_id", userID.String()))

	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, tx, userID, clientID)
		if err != nil {
			return err
		}
		if user == nil {
			return helper.NewNotFound("Member")
		}

		now := time.Now()

		if subscriptionID != nil {
			// ── Selective kick: cancel only the specified subscription ──
			skipped, err := uc.kickSubscriptionSelective(ctx, tx, clientID, userID, *subscriptionID, user.TelegramUserID, now)
			if err != nil {
				return err
			}
			if skipped {
				return helper.NewBadRequest("Langganan tidak ditemukan atau sudah tidak aktif")
			}
		} else {
			// ── Global kick: cancel all active subscriptions (existing behavior) ──
			subs, err := uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, tx, user.TelegramUserID, clientID)
			if err != nil {
				return err
			}
			if len(subs) == 0 {
				return helper.NewBadRequest("Member tidak memiliki langganan aktif")
			}

			for i := range subs {
				sub := &subs[i]
				sub.Status = "cancelled"
				sub.KickedAt = &now
				if err := uc.subscriptionRepo.Update(ctx, tx, sub); err != nil {
					return err
				}

				if err := createKickOutboxEvents(ctx, tx, uc.outboxRepo, sub, user.TelegramUserID, now); err != nil {
					return err
				}
			}
		}

		return uc.writeKickAuditLog(ctx, tx, clientID, userID)
	})
}

// kickSubscriptionSelective cancels a single subscription if it is valid and active.
// Returns (skipped=true, nil) when the subscription is missing, not owned by the
// member/client, or already expired — callers decide whether to treat that as an
// error (single kick) or a silent skip (bulk kick).
func (uc *memberUseCase) kickSubscriptionSelective(ctx context.Context, tx *gorm.DB, clientID, userID, subscriptionID uuid.UUID, telegramUserID int64, now time.Time) (bool, error) {
	sub, err := uc.subscriptionRepo.FindActiveByIdWithPackage(ctx, tx, subscriptionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, err
	}
	if sub.TelegramUserID != userID || sub.ClientID != clientID {
		return true, nil
	}
	if now.After(sub.ExpiredAt) {
		return true, nil
	}

	sub.Status = "cancelled"
	sub.KickedAt = &now
	if err := uc.subscriptionRepo.Update(ctx, tx, sub); err != nil {
		return false, err
	}

	if err := createKickOutboxEvents(ctx, tx, uc.outboxRepo, sub, telegramUserID, now); err != nil {
		return false, err
	}

	return false, nil
}

func (uc *memberUseCase) writeKickAuditLog(ctx context.Context, tx *gorm.DB, clientID, userID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)

	auditLogMeta, marshalErr := sonic.Marshal(map[string]interface{}{"reason": "manual_kick"})
	if marshalErr != nil {
		return marshalErr
	}
	auditLog := entity.AuditLog{
		ClientID:   &clientID,
		EntityType: "member",
		EntityID:   userID,
		Action:     "kick_member",
		ActorType:  "system",
		ActorID:    "system",
		Metadata:   datatypes.JSON(auditLogMeta),
	}
	if err := uc.auditLogRepo.Create(ctx, tx, &auditLog); err != nil {
		log.Error("failed to create audit log for kick", zap.Error(err))
		return err
	}
	return nil
}

// BulkKickMembers performs best-effort selective kicks. Items whose subscription
// is missing, not owned, or already expired are skipped silently.
func (uc *memberUseCase) BulkKickMembers(ctx context.Context, clientID uuid.UUID, items []model.BulkMemberKickItem) model.BulkDeleteResult {
	log := logger.FromContext(ctx, uc.log)

	result := model.BulkDeleteResult{Deleted: 0, Failed: []model.BulkDeleteFailure{}}
	for _, item := range items {
		skipped := false
		err := uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, tx, item.MemberID, clientID)
			if err != nil {
				return err
			}
			if user == nil {
				skipped = true // member not in this tenant → skip
				return nil
			}

			now := time.Now()
			s, err := uc.kickSubscriptionSelective(ctx, tx, clientID, item.MemberID, item.SubscriptionID, user.TelegramUserID, now)
			if err != nil {
				return err
			}
			if s {
				skipped = true // skip silently (not counted, not failed)
				return nil
			}

			return uc.writeKickAuditLog(ctx, tx, clientID, item.MemberID)
		})
		if err != nil {
			log.Warn("member usecase BulkKickMembers item failed", zap.String("member_id", item.MemberID.String()), zap.String("subscription_id", item.SubscriptionID.String()), zap.Error(err))
			result.Failed = append(result.Failed, model.BulkDeleteFailure{ID: item.MemberID, Error: err.Error()})
			continue
		}
		if skipped {
			continue
		}
		result.Deleted++
	}

	if len(result.Failed) == 0 {
		result.Failed = nil
	}
	return result
}

// createKickOutboxEvents creates enforcer.kick outbox events for every group in the subscription's package.
// ponytail: extracted to deduplicate between global and selective kick paths.
func createKickOutboxEvents(ctx context.Context, tx *gorm.DB, outboxRepo repository.IOutboxRepository, sub *entity.Subscription, telegramUserID int64, now time.Time) error {
	if sub.Package.ID == uuid.Nil {
		return nil
	}
	for _, group := range sub.Package.Groups {
		payload := map[string]interface{}{
			"telegram_user_id": telegramUserID,
			"telegram_chat_id": group.TelegramChatID,
			"bot_id":           group.BotUUID.String(),
		}
		payloadBytes, err := sonic.Marshal(payload)
		if err != nil {
			return err
		}

		outbox := entity.Outbox{
			AggregateType: "subscription",
			AggregateID:   sub.ID,
			EventType:     "enforcer.kick",
			Payload:       datatypes.JSON(payloadBytes),
			Status:        "pending",
			ProcessAfter:  now,
			RetryCount:    0,
		}
		if err := outboxRepo.Create(ctx, tx, &outbox); err != nil {
			return err
		}
	}
	return nil
}

// ExtendMember adds N days to the expiry of a specific subscription.
func (uc *memberUseCase) ExtendMember(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, req *model.ExtendMemberRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase ExtendMember start", zap.String("user_id", userID.String()))

	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sub entity.Subscription
		if err := uc.subscriptionRepo.FindById(ctx, tx, &sub, req.SubscriptionID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return helper.NewNotFound("Langganan")
			}
			return err
		}

		if sub.TelegramUserID != userID {
			return helper.NewNotFound("Langganan")
		}
		if sub.ClientID != clientID {
			return helper.NewNotFound("Langganan")
		}
		if sub.Status != "active" {
			return helper.NewBadRequest("Hanya langganan aktif yang dapat diperpanjang")
		}

		oldExpiry := sub.ExpiredAt
		sub.ExpiredAt = sub.ExpiredAt.AddDate(0, 0, req.AdditionalDays)
		if err := uc.subscriptionRepo.Update(ctx, tx, &sub); err != nil {
			return err
		}

		return uc.writeExtendAuditLog(ctx, tx, clientID, sub, req.AdditionalDays, oldExpiry)
	})
}

// BulkExtendMembers performs best-effort subscription extends. Items whose
// subscription is missing, not owned, or not active are skipped silently.
func (uc *memberUseCase) BulkExtendMembers(ctx context.Context, clientID uuid.UUID, items []model.BulkMemberExtendItem) model.BulkDeleteResult {
	log := logger.FromContext(ctx, uc.log)

	result := model.BulkDeleteResult{Deleted: 0, Failed: []model.BulkDeleteFailure{}}
	for _, item := range items {
		skipped := false
		err := uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, tx, item.MemberID, clientID)
			if err != nil {
				return err
			}
			if user == nil {
				skipped = true // member not in this tenant → skip
				return nil
			}

			var sub entity.Subscription
			if err := uc.subscriptionRepo.FindById(ctx, tx, &sub, item.SubscriptionID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					skipped = true // subscription missing → skip
					return nil
				}
				return err
			}
			if sub.TelegramUserID != item.MemberID || sub.ClientID != clientID || sub.Status != "active" {
				skipped = true // not owned or not active → skip
				return nil
			}

			oldExpiry := sub.ExpiredAt
			sub.ExpiredAt = sub.ExpiredAt.AddDate(0, 0, item.AdditionalDays)
			if err := uc.subscriptionRepo.Update(ctx, tx, &sub); err != nil {
				return err
			}

			return uc.writeExtendAuditLog(ctx, tx, clientID, sub, item.AdditionalDays, oldExpiry)
		})
		if err != nil {
			log.Warn("member usecase BulkExtendMembers item failed", zap.String("member_id", item.MemberID.String()), zap.String("subscription_id", item.SubscriptionID.String()), zap.Error(err))
			result.Failed = append(result.Failed, model.BulkDeleteFailure{ID: item.MemberID, Error: err.Error()})
			continue
		}
		if skipped {
			continue
		}
		result.Deleted++
	}

	if len(result.Failed) == 0 {
		result.Failed = nil
	}
	return result
}

// writeExtendAuditLog writes an extend_expiry audit log entry.
func (uc *memberUseCase) writeExtendAuditLog(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, sub entity.Subscription, additionalDays int, oldExpiry time.Time) error {
	log := logger.FromContext(ctx, uc.log)

	auditLogMeta, marshalErr := sonic.Marshal(map[string]interface{}{
		"subscription_id": sub.ID.String(),
		"additional_days": additionalDays,
		"old_expiry":      oldExpiry.Format(time.RFC3339),
		"new_expiry":      sub.ExpiredAt.Format(time.RFC3339),
	})
	if marshalErr != nil {
		uc.log.Warn("member usecase: failed to marshal extend audit log meta", zap.Error(marshalErr))
	}
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
	return nil
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
					payloadBytes, marshalErr := sonic.Marshal(payload)
					if marshalErr != nil {
						uc.log.Warn("member usecase: failed to marshal sync_request outbox payload", zap.Error(marshalErr))
					}

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
func (uc *memberUseCase) ResendLink(ctx context.Context, clientID uuid.UUID, userID uuid.UUID, subscriptionID *uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member usecase ResendLink start", zap.String("user_id", userID.String()))

	user, err := uc.tgUserRepo.FindMemberDetailByID(ctx, uc.db.Gorm, userID, clientID)
	if err != nil {
		return err
	}
	if user == nil {
		return helper.NewNotFound("Member")
	}

	var subs []entity.Subscription
	if subscriptionID != nil {
		sub, err := uc.subscriptionRepo.FindActiveByIdWithPackage(ctx, uc.db.Gorm, *subscriptionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return helper.NewNotFound("Langganan aktif tidak ditemukan")
			}
			return err
		}
		if sub.ClientID != clientID || sub.TelegramUserID != user.ID {
			return helper.NewNotFound("Langganan aktif tidak ditemukan")
		}
		subs = append(subs, *sub)
	} else {
		var err error
		subs, err = uc.subscriptionRepo.FindActiveByTelegramUserID(ctx, uc.db.Gorm, user.TelegramUserID, clientID)
		if err != nil {
			return err
		}
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
				"is_resend":        true,
			}
			payloadBytes, marshalErr := sonic.Marshal(payload)
			if marshalErr != nil {
				uc.log.Warn("member usecase: failed to marshal resend_link outbox payload", zap.Error(marshalErr))
			}

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
