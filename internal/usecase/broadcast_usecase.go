package usecase

import (
	"context"
	json "github.com/bytedance/sonic"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IBroadcastUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.CreateBroadcastRequest) (*model.BroadcastResponse, error)
	List(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, filter *model.BroadcastFilterRequest) ([]model.BroadcastResponse, int64, error)
	DistributeScheduled(ctx context.Context, broadcastID uuid.UUID) error
	GetReach(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (*model.BroadcastReachResponse, error)
}

type BroadcastUseCase struct {
	db            *entity.Database
	broadcastRepo repository.IBroadcastRepository
	botRepo       repository.ITelegramBotRepository
	groupRepo     repository.ITelegramGroupRepository
	subRepo       repository.ISubscriptionRepository
	outboxRepo    repository.IOutboxRepository
	billingRepo   repository.IClientBillingRepository
	log           *zap.Logger
}

func NewBroadcastUseCase(
	db *entity.Database,
	broadcastRepo repository.IBroadcastRepository,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	subRepo repository.ISubscriptionRepository,
	outboxRepo repository.IOutboxRepository,
	billingRepo repository.IClientBillingRepository,
	log *zap.Logger,
) IBroadcastUseCase {
	return &BroadcastUseCase{
		db:            db,
		broadcastRepo: broadcastRepo,
		botRepo:       botRepo,
		groupRepo:     groupRepo,
		subRepo:       subRepo,
		outboxRepo:    outboxRepo,
		billingRepo:   billingRepo,
		log:           log,
	}
}

func (uc *BroadcastUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.CreateBroadcastRequest) (*model.BroadcastResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("broadcast usecase: starting create", zap.String("client_id", clientID.String()), zap.String("bot_id", req.BotID.String()))

	// 1. Validasi Bot milik Tenant
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, req.BotID)
	if err != nil || bot == nil {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}
	if bot.ClientID != clientID {
		return nil, helper.NewBadRequest("Akses ke bot ditolak")
	}

	// Validasi Kuota Broadcast Bulanan Tenant
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("broadcast usecase: failed to fetch billing", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxBroadcasts != -1 {
		// Hitung jumlah broadcast bulan ini
		var count int64
		now := time.Now().UTC()
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

		err := uc.db.Gorm.WithContext(ctx).Model(&entity.Broadcast{}).
			Where("client_id = ? AND created_at >= ? AND deleted_at IS NULL", clientID, startOfMonth).
			Count(&count).Error
		if err != nil {
			log.Error("broadcast usecase: failed to count broadcasts", zap.Error(err))
			return nil, fmt.Errorf("Gagal menghitung kuota broadcast")
		}

		if count >= int64(billing.Plan.MaxBroadcasts) {
			return nil, helper.NewBadRequest(fmt.Sprintf(
				"Kuota broadcast bulanan Anda sudah penuh (%d/%d). Silakan upgrade paket platform.",
				count, billing.Plan.MaxBroadcasts,
			))
		}
	}

	// Validasi IsImmediate vs Scheduled
	if req.IsImmediate {
		if req.ScheduledAt != nil {
			return nil, helper.NewBadRequest("scheduled_at tidak boleh diset untuk broadcast langsung")
		}
	} else {
		if req.ScheduledAt == nil {
			return nil, helper.NewBadRequest("scheduled_at harus diisi untuk broadcast terjadwal")
		}
		if req.ScheduledAt.Before(time.Now().Add(1 * time.Minute)) {
			return nil, helper.NewBadRequest("Waktu penjadwalan harus minimal 1 menit di masa depan")
		}
	}

	// 2. Simpan entitas Broadcast
	broadcastID := uuid.New()
	broadcast := &entity.Broadcast{
		ID:           broadcastID,
		ClientID:     clientID,
		BotUUID:      req.BotID,
		TargetType:   req.TargetType,
		MessageType:  req.MessageType,
		MessageText:  req.MessageText,
		FileUrl:      req.FileUrl,
		ScheduledAt:  req.ScheduledAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if len(req.GroupIDs) > 0 {
		b, _ := json.Marshal(req.GroupIDs)
		s := string(b)
		broadcast.GroupFilter = &s
	}

	if !req.IsImmediate {
		broadcast.Status = "scheduled"
		broadcast.TotalTargets = 0
		if err := uc.broadcastRepo.Create(ctx, uc.db.Gorm, broadcast); err != nil {
			log.Error("broadcast usecase: failed to create scheduled broadcast", zap.Error(err))
			return nil, fmt.Errorf("Gagal membuat jadwal broadcast")
		}
	} else {
		broadcast.Status = "pending"
		err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
			if err := uc.broadcastRepo.Create(ctx, tx, broadcast); err != nil {
				return err
			}
			return uc.distribute(ctx, tx, broadcast)
		})
		if err != nil {
			log.Error("broadcast usecase: immediate distribution failed", zap.Error(err))
			return nil, helper.NewBadRequest(err.Error())
		}
	}

	return converter.BroadcastToResponse(broadcast), nil
}

func (uc *BroadcastUseCase) distribute(ctx context.Context, tx *gorm.DB, broadcast *entity.Broadcast) error {
	log := logger.FromContext(ctx, uc.log)

	gidSet := make(map[uuid.UUID]bool)
	hasFilter := false
	if broadcast.GroupFilter != nil && *broadcast.GroupFilter != "" {
		var ids []uuid.UUID
		if err := json.Unmarshal([]byte(*broadcast.GroupFilter), &ids); err == nil && len(ids) > 0 {
			hasFilter = true
			for _, id := range ids {
				gidSet[id] = true
			}
		}
	}

	// 1. Kueri target penerima
	var targetChatIDs []int64

	if broadcast.TargetType == "group" {
		groups, err := uc.groupRepo.FindByBotID(ctx, tx, broadcast.BotUUID)
		if err != nil {
			log.Error("broadcast usecase: failed to fetch groups", zap.Error(err))
			return fmt.Errorf("Gagal mencari grup penerima")
		}
		for _, g := range groups {
			if !g.IsActive {
				continue
			}
			if hasFilter && !gidSet[g.ID] {
				continue
			}
			targetChatIDs = append(targetChatIDs, g.TelegramChatID)
		}
	} else if broadcast.TargetType == "member" {
		gids := make([]uuid.UUID, 0, len(gidSet))
		for id := range gidSet {
			gids = append(gids, id)
		}
		var err error
		targetChatIDs, err = uc.subRepo.FindActiveMemberIDsByBotID(ctx, tx, broadcast.ClientID, broadcast.BotUUID, gids)
		if err != nil {
			log.Error("broadcast usecase: failed to fetch active member subscriptions", zap.Error(err))
			return fmt.Errorf("Gagal mencari member aktif penerima")
		}
	}

	if len(targetChatIDs) == 0 {
		return fmt.Errorf("Tidak ditemukan target penerima aktif untuk broadcast ini")
	}

	// 2. Update status and total targets
	broadcast.TotalTargets = len(targetChatIDs)
	broadcast.Status = "pending"
	broadcast.UpdatedAt = time.Now()
	if err := tx.Save(broadcast).Error; err != nil {
		return fmt.Errorf("failed to update broadcast stats: %w", err)
	}

	// 3. Buat Outbox record untuk masing-masing target
	for _, chatID := range targetChatIDs {
		payloadMap := map[string]interface{}{
			"broadcast_id": broadcast.ID.String(),
			"bot_id":       broadcast.BotUUID.String(),
			"chat_id":      chatID,
			"message_type": broadcast.MessageType,
			"message_text": broadcast.MessageText,
		}
		if broadcast.FileUrl != nil {
			payloadMap["file_url"] = *broadcast.FileUrl
		}

		payloadBytes, err := json.Marshal(payloadMap)
		if err != nil {
			return fmt.Errorf("failed to marshal outbox payload: %w", err)
		}

		outbox := &entity.Outbox{
			ID:            uuid.New(),
			AggregateType: "broadcast",
			AggregateID:   broadcast.ID,
			EventType:     "broadcast.send",
			Payload:       payloadBytes,
			Status:        "pending",
			RetryCount:    0,
			MaxRetries:    3,
			ProcessAfter:  time.Now(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := uc.outboxRepo.Create(ctx, tx, outbox); err != nil {
			return fmt.Errorf("failed to create outbox record: %w", err)
		}
	}

	return nil
}

func (uc *BroadcastUseCase) DistributeScheduled(ctx context.Context, broadcastID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("broadcast usecase: distributing scheduled broadcast", zap.String("broadcast_id", broadcastID.String()))

	err := uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		var broadcast entity.Broadcast
		// Lock the row
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", broadcastID, "distributing").
			First(&broadcast).Error
		if err != nil {
			return err
		}

		// Run distribution
		return uc.distribute(ctx, tx, &broadcast)
	})

	if err != nil {
		log.Error("broadcast usecase: distribution failed, marking as failed in DB", zap.String("broadcast_id", broadcastID.String()), zap.Error(err))

		// Update status to failed using a clean DB connection
		updateErr := uc.db.Gorm.Model(&entity.Broadcast{}).
			Where("id = ? AND status = ?", broadcastID, "distributing").
			Update("status", "failed").Error
		if updateErr != nil {
			log.Error("broadcast usecase: failed to update status to failed", zap.String("broadcast_id", broadcastID.String()), zap.Error(updateErr))
		}
		return err
	}

	return nil
}

func (uc *BroadcastUseCase) List(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, filter *model.BroadcastFilterRequest) ([]model.BroadcastResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("broadcast usecase: listing history", zap.String("client_id", clientID.String()))

	// Validasi Bot milik Tenant
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil || bot == nil {
		return nil, 0, helper.NewBadRequest("Bot tidak valid")
	}
	if bot.ClientID != clientID {
		return nil, 0, helper.NewBadRequest("Akses ke bot ditolak")
	}

	list, err := uc.broadcastRepo.FindByClientID(ctx, uc.db.Gorm, clientID, &botID, filter.Page, filter.Limit)
	if err != nil {
		log.Error("broadcast usecase: list failed", zap.Error(err))
		return nil, 0, fmt.Errorf("Gagal mengambil riwayat broadcast")
	}

	total, err := uc.broadcastRepo.CountByClientID(ctx, uc.db.Gorm, clientID, &botID)
	if err != nil {
		log.Error("broadcast usecase: count failed", zap.Error(err))
		return nil, 0, fmt.Errorf("Gagal menghitung riwayat broadcast")
	}

	return converter.BroadcastListToResponse(list), total, nil
}

func (uc *BroadcastUseCase) GetReach(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (*model.BroadcastReachResponse, error) {
	log := logger.FromContext(ctx, uc.log)

	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil || bot == nil {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}
	if bot.ClientID != clientID {
		return nil, helper.NewBadRequest("Akses ke bot ditolak")
	}

	groups, err := uc.groupRepo.FindByBotID(ctx, uc.db.Gorm, botID)
	if err != nil {
		log.Error("broadcast reach: failed to fetch groups", zap.Error(err))
		return nil, fmt.Errorf("Gagal mengambil data grup")
	}

	activeCount := 0
	for _, g := range groups {
		if g.IsActive {
			activeCount++
		}
	}

	memberCount, err := uc.subRepo.CountActiveMembersByBotID(ctx, uc.db.Gorm, clientID, botID)
	if err != nil {
		log.Error("broadcast reach: failed to count members", zap.Error(err))
		return nil, fmt.Errorf("Gagal menghitung jumlah member")
	}

	return &model.BroadcastReachResponse{
		GroupCount:  activeCount,
		MemberCount: int(memberCount),
	}, nil
}
