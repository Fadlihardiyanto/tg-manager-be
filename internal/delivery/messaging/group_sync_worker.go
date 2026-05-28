package messaging

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type GroupSyncWorker struct {
	db              *gorm.DB
	groupRepo       repository.ITelegramGroupRepository
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewGroupSyncWorker(
	db *gorm.DB,
	groupRepo repository.ITelegramGroupRepository,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	log *zap.Logger,
) *GroupSyncWorker {
	return &GroupSyncWorker{
		db:              db,
		groupRepo:       groupRepo,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		log:             log,
	}
}

func (w *GroupSyncWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("group sync worker: starting polling loop", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("group sync worker: stopping loop")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *GroupSyncWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("group_sync").Observe(time.Since(start).Seconds())

	// 1. Fetch all active groups
	groups, err := w.groupRepo.FindAllActive(ctx, w.db)
	if err != nil {
		w.log.Error("group sync worker: failed to fetch active groups", zap.Error(err))
		return
	}

	if len(groups) == 0 {
		return
	}

	w.log.Info("group sync worker: starting sync", zap.Int("total_groups", len(groups)))

	// 2. Group by BotID to avoid multiple bot client instantiations for the same bot
	groupsByBot := make(map[uuid.UUID][]int)
	for i, group := range groups {
		groupsByBot[group.BotID] = append(groupsByBot[group.BotID], i)
	}

	// 3. Process each bot
	for botID, groupIndices := range groupsByBot {
		bot, err := w.botRepo.FindByID(ctx, w.db, botID)
		if err != nil || bot == nil {
			w.log.Error("group sync worker: failed to fetch bot", zap.String("bot_id", botID.String()), zap.Error(err))
			continue
		}

		token, err := crypto.Decrypt(bot.Token, w.encryptionKey)
		if err != nil {
			w.log.Error("group sync worker: failed to decrypt token", zap.String("bot_id", botID.String()), zap.Error(err))
			continue
		}

		botClient, err := w.telegramFactory.NewClient(token)
		if err != nil {
			w.log.Error("group sync worker: failed to init bot client", zap.String("bot_id", botID.String()), zap.Error(err))
			continue
		}

		// 4. Process each group for this bot
		for _, idx := range groupIndices {
			group := groups[idx]
			
			count, err := botClient.GetChatMembersCount(ctx, group.TelegramChatID)
			if err != nil {
				w.log.Warn("group sync worker: failed to get member count, marking inactive",
					zap.String("group_id", group.ID.String()),
					zap.Error(err),
				)
				// If we can't get the count, maybe the bot was kicked silently. We deactivate the group.
				group.IsActive = false
				group.InactiveReason = "Failed to fetch member count: " + err.Error()
				w.groupRepo.Update(ctx, w.db, &group)
			} else {
				// Update the member count
				if group.MemberCount != count {
					group.MemberCount = count
					group.UpdatedAt = time.Now()
					if err := w.groupRepo.Update(ctx, w.db, &group); err != nil {
						w.log.Error("group sync worker: failed to update group", zap.String("group_id", group.ID.String()), zap.Error(err))
					} else {
						w.log.Debug("group sync worker: updated member count", zap.String("group_id", group.ID.String()), zap.Int("count", count))
					}
				}
			}

			// Rate limit protection: sleep briefly between calls
			time.Sleep(50 * time.Millisecond)
		}
	}

	w.log.Info("group sync worker: finished sync", zap.Duration("duration", time.Since(start)))
}
