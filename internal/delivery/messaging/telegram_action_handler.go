package messaging

import (
	"context"
	"fmt"
	"strings"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TelegramActionHandler struct {
	db              *gorm.DB
	packageRepo     repository.IPackageRepository
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewTelegramActionHandler(
	db *gorm.DB,
	packageRepo repository.IPackageRepository,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *TelegramActionHandler {
	return &TelegramActionHandler{
		db:              db,
		packageRepo:     packageRepo,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
	}
}

type SubscriptionActivatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	TelegramUserID int64  `json:"telegram_user_id"`
	PackageID      string `json:"package_id"`
	ClientID       string `json:"client_id"`
}

func (h *TelegramActionHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	// 1. Parse Payload
	var payload SubscriptionActivatedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("telegram action handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		return nil
	}

	h.logger.Info("telegram action handler: processing subscription.activated", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)

	// 2. Fetch Package & associated Groups
	pkgID, err := uuid.Parse(payload.PackageID)
	if err != nil {
		h.logger.Error("telegram action handler: invalid package id", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	pkg, err := h.packageRepo.FindByID(ctx, h.db, pkgID)
	if err != nil {
		return fmt.Errorf("package lookup failed: %w", err)
	}
	if pkg == nil {
		h.logger.Error("telegram action handler: package not found", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	if len(pkg.Groups) == 0 {
		h.logger.Info("telegram action handler: no groups associated with package, skipping", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	// 3. Fetch Bot for this Client
	clientID, err := uuid.Parse(payload.ClientID)
	if err != nil {
		h.logger.Error("telegram action handler: invalid client id", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}

	bot, err := h.botRepo.FindFirstByClientID(ctx, h.db, clientID)
	if err != nil {
		return fmt.Errorf("bot lookup failed: %w", err)
	}
	if bot == nil {
		h.logger.Error("telegram action handler: no bot found for client", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt bot token: %w", err)
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		return fmt.Errorf("failed to init telegram client: %w", err)
	}

	// 4. Generate Invite Links for each group
	var inviteLinks []string
	var failedGroups []string
	for _, group := range pkg.Groups {
		// Limit to 1 use, so it can't be shared
		subIDPrefix := payload.SubscriptionID
		if len(subIDPrefix) > 8 {
			subIDPrefix = subIDPrefix[:8]
		}
		linkName := fmt.Sprintf("Sub-%s", subIDPrefix)
		inviteLink, err := botClient.CreateChatInviteLink(ctx, group.TelegramChatID, linkName, true)
		if err != nil {
			h.logger.Error("telegram action handler: failed to create invite link", append(logFields, zap.String("group", group.Name), zap.Error(err))...)
			failedGroups = append(failedGroups, group.Name)
			continue
		}
		inviteLinks = append(inviteLinks, fmt.Sprintf("• %s: %s", group.Name, inviteLink))
	}

	if len(inviteLinks) == 0 {
		h.logger.Error("telegram action handler: failed to generate any invite links, dropping message",
			append(logFields,
				zap.String("subscription_id", payload.SubscriptionID),
				zap.String("package_id", payload.PackageID),
			)...,
		)
		return nil
	}

	// 5. Send DM to User
	message := fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\n\nBerikut adalah link khusus untuk masuk ke grup:\n%s\n\n<i>Link ini hanya berlaku untuk 1 kali pakai.</i>", pkg.Name, strings.Join(inviteLinks, "\n"))
	if len(failedGroups) > 0 {
		message = fmt.Sprintf("%s\n\n⚠️ Gagal membuat link untuk: %s. Silakan hubungi admin.", message, strings.Join(failedGroups, ", "))
	}

	if err := botClient.SendMessage(ctx, payload.TelegramUserID, message); err != nil {
		h.logger.Error("telegram action handler: failed to send dm", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Error(err))...)
		return fmt.Errorf("failed to send dm: %w", err)
	}

	h.logger.Info("telegram action handler: successfully processed subscription.activated", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)
	return nil
}
