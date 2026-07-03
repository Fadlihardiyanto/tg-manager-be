package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/google/uuid"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrationMemberHandler struct {
	db                  *entity.Database
	migrationMemberRepo repository.IMigrationMemberRepository
	packageRepo         repository.IPackageRepository
	subscriptionRepo    repository.ISubscriptionRepository
	telegramUserRepo    repository.ITelegramUserRepository
	telegramGroupRepo   repository.ITelegramGroupRepository
	telegramFactory     telegram.BotFactory
	encryptionKey       string
	log                 *zap.Logger
}

func NewMigrationMemberHandler(
	db *entity.Database,
	migrationMemberRepo repository.IMigrationMemberRepository,
	packageRepo repository.IPackageRepository,
	subscriptionRepo repository.ISubscriptionRepository,
	telegramUserRepo repository.ITelegramUserRepository,
	telegramGroupRepo repository.ITelegramGroupRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	log *zap.Logger,
) *MigrationMemberHandler {
	return &MigrationMemberHandler{
		db:                  db,
		migrationMemberRepo: migrationMemberRepo,
		packageRepo:         packageRepo,
		subscriptionRepo:    subscriptionRepo,
		telegramUserRepo:    telegramUserRepo,
		telegramGroupRepo:   telegramGroupRepo,
		telegramFactory:     telegramFactory,
		encryptionKey:       encryptionKey,
		log:                 log,
	}
}

func (h *MigrationMemberHandler) Name() string {
	return "migrasi"
}

func (h *MigrationMemberHandler) AllowedRoles() []string {
	return []string{"all_in_one", "sales_only"}
}

type claimResult struct {
	success     bool
	packageName string
	expiredAt   time.Time
	groupName   string
	errMsg      string
}

func (h *MigrationMemberHandler) Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	h.log.Info("executing /migrasi command",
		zap.Int64("user_id", msg.From.ID),
		zap.String("bot_id", bot.ID.String()),
	)

	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.log.Error("failed to decrypt bot token", zap.Error(err))
		return err
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.log.Error("failed to init bot client", zap.Error(err))
		return err
	}

	username := strings.TrimSpace(msg.From.UserName)
	if username == "" {
		return replyHTML(ctx, botClient, msg.Chat.ID, "❌ Maaf, Anda harus mengatur <b>username Telegram</b> terlebih dahulu.\n\nSilakan buka <b>Settings → Username</b> di aplikasi Telegram Anda, lalu coba lagi dengan /migrasi.")
	}

	username = strings.ToLower(username)

	claims, err := h.migrationMemberRepo.FindPendingClaimsByUsername(ctx, h.db.Gorm, username)
	if err != nil {
		h.log.Error("failed to find pending claims", zap.Error(err))
		return replyHTML(ctx, botClient, msg.Chat.ID, "⚠️ Terjadi kesalahan. Silakan coba lagi nanti.")
	}

	if len(claims) == 0 {
		return replyHTML(ctx, botClient, msg.Chat.ID, "❌ Tidak ada data migrasi yang tertunda untuk username Anda.\n\nSilakan hubungi admin grup jika Anda merasa seharusnya mendapatkan akses.")
	}

	var allAccessGroups []entity.Group
	allAccessCache := make(map[uuid.UUID]bool)

	results := make([]claimResult, 0, len(claims))

	for _, claim := range claims {
		result := claimResult{}

		pkg, err := h.packageRepo.FindByID(ctx, h.db.Gorm, claim.PackageID)
		if err != nil {
			h.log.Error("failed to find package for claim", zap.String("package_id", claim.PackageID.String()), zap.Error(err))
			result.errMsg = "⚠️ Paket tidak ditemukan"
			results = append(results, result)
			continue
		}

		var groupsToCheck []entity.Group
		if pkg.IsAllAccess {
			if !allAccessCache[claim.ClientID] {
				groups, errFetch := h.telegramGroupRepo.FindByClientID(ctx, h.db.Gorm, claim.ClientID, 1, 1000)
				if errFetch != nil {
					h.log.Error("failed to fetch all access groups", zap.Error(errFetch))
					result.errMsg = "⚠️ Gagal memeriksa grup all-access"
					results = append(results, result)
					continue
				}
				allAccessGroups = groups
				allAccessCache[claim.ClientID] = true
			}
			groupsToCheck = allAccessGroups
		} else {
			groupsToCheck = pkg.Groups
		}

		if len(groupsToCheck) == 0 {
			result.errMsg = "❌ Paket tidak terhubung ke grup"
			results = append(results, result)
			continue
		}

		var foundGroup *entity.Group
		for i := range groupsToCheck {
			group := &groupsToCheck[i]
			if !group.IsActive {
				continue
			}
			member, err := botClient.GetChatMember(ctx, group.TelegramChatID, msg.From.ID)
			if err != nil {
				h.log.Debug("getChatMember failed, trying next group",
					zap.Int64("chat_id", group.TelegramChatID),
					zap.Error(err),
				)
				continue
			}
			switch member.Status {
			case "creator", "administrator", "member", "restricted":
				foundGroup = group
			case "left", "kicked":
				h.log.Debug("user not in group", zap.Int64("chat_id", group.TelegramChatID), zap.String("status", member.Status))
			}
			if foundGroup != nil {
				break
			}
		}

		if foundGroup == nil {
			result.errMsg = fmt.Sprintf("❌ Tidak ditemukan di grup untuk paket <b>%s</b>", pkg.Name)
			results = append(results, result)
			continue
		}

		now := time.Now()
		txErr := h.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			telegramUser, err := h.telegramUserRepo.FindByTelegramID(ctx, tx, msg.From.ID)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("gagal mencari data pengguna: %w", err)
				}
				telegramUser = &entity.TelegramUser{
					ID:             uuid.New(),
					TelegramUserID: msg.From.ID,
					Username:       username,
					FirstName:      msg.From.FirstName,
					LastName:       msg.From.LastName,
					CreatedAt:      now,
					UpdatedAt:      now,
				}
				if err := h.telegramUserRepo.Create(ctx, tx, telegramUser); err != nil {
					return fmt.Errorf("gagal membuat data pengguna: %w", err)
				}
			} else {
				telegramUser.Username = username
				telegramUser.FirstName = msg.From.FirstName
				telegramUser.LastName = msg.From.LastName
				telegramUser.UpdatedAt = now
				if err := h.telegramUserRepo.Update(ctx, tx, telegramUser); err != nil {
					return fmt.Errorf("gagal memperbarui data pengguna: %w", err)
				}
			}

			subscription := &entity.Subscription{
				ID:               uuid.New(),
				TelegramUserID:   telegramUser.ID,
				PackageID:        claim.PackageID,
				ClientID:         claim.ClientID,
				Status:           "active",
				ActivatedAt:      now,
				ExpiredAt:        claim.ExpiredAt,
				GracePeriodHours: 0,
				LastCheckedAt:    now,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := h.subscriptionRepo.Create(ctx, tx, subscription); err != nil {
				return fmt.Errorf("gagal membuat langganan: %w", err)
			}

			if err := h.migrationMemberRepo.UpdateStatus(ctx, tx, claim.ID, "claimed"); err != nil {
				return fmt.Errorf("gagal memperbarui status klaim: %w", err)
			}

			return nil
		})

		if txErr != nil {
			h.log.Error("migration claim transaction failed",
				zap.String("claim_id", claim.ID.String()),
				zap.Error(txErr),
			)
			result.errMsg = fmt.Sprintf("⚠️ Gagal memproses paket <b>%s</b>", pkg.Name)
			results = append(results, result)
			continue
		}

		result.success = true
		result.packageName = pkg.Name
		result.expiredAt = claim.ExpiredAt
		result.groupName = foundGroup.Name
		results = append(results, result)
	}

	return h.sendClaimResults(ctx, botClient, msg.Chat.ID, results)
}

func (h *MigrationMemberHandler) sendClaimResults(ctx context.Context, client telegram.BotClient, chatID int64, results []claimResult) error {
	allFailed := true
	var sb strings.Builder

	for _, r := range results {
		if r.success {
			allFailed = false
		}
	}

	if allFailed {
		sb.WriteString("❌ <b>Tidak ada klaim yang berhasil.</b>\n\n")
		for _, r := range results {
			if r.errMsg != "" {
				sb.WriteString(r.errMsg)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\nPastikan Anda masih menjadi anggota grup, atau hubungi admin.")
		return replyHTML(ctx, client, chatID, sb.String())
	}

	succeeded := 0
	for _, r := range results {
		if r.success {
			succeeded++
		}
	}

	if succeeded == 1 && len(results) == 1 {
		r := results[0]
		msg := fmt.Sprintf(
			"✅ <b>Berhasil!</b> Langganan Anda telah diaktifkan.\n\n"+
				"📦 Paket: <b>%s</b>\n"+
				"📅 Berlaku hingga: <b>%s</b>\n"+
				"👥 Grup: <b>%s</b>\n\n"+
				"Gunakan /mysub untuk melihat status langganan Anda.",
			r.packageName,
			r.expiredAt.Format("02 January 2006"),
			r.groupName,
		)
		return replyHTML(ctx, client, chatID, msg)
	}

	sb.WriteString(fmt.Sprintf("✅ <b>Berhasil mengaktifkan %d Paket:</b>\n\n", succeeded))
	for _, r := range results {
		if r.success {
			sb.WriteString(fmt.Sprintf("📦 <b>%s</b> — berlaku hingga <b>%s</b>\n",
				r.packageName, r.expiredAt.Format("02 January 2006")))
		}
	}
	sb.WriteString("\nGunakan /mysub untuk melihat status langganan Anda.")

	failedCount := len(results) - succeeded
	if failedCount > 0 {
		sb.WriteString(fmt.Sprintf("\n\n⚠️ <b>%d paket gagal diklaim:</b>\n", failedCount))
		for _, r := range results {
			if !r.success && r.errMsg != "" {
				sb.WriteString(fmt.Sprintf("• %s\n", r.errMsg))
			}
		}
	}

	return replyHTML(ctx, client, chatID, sb.String())
}

func replyHTML(ctx context.Context, client telegram.BotClient, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := client.Send(ctx, msg)
	return err
}
