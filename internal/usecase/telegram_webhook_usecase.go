package usecase

import (
	"context"
	"errors"
	"fmt"
	json "github.com/bytedance/sonic"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/bot/handler"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/gateway/messaging"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
)

type ITelegramWebhookUseCase interface {
	ProcessUpdate(ctx context.Context, botID uuid.UUID, update *tgbotapi.Update) error
}

type TelegramWebhookUseCase struct {
	db              *entity.Database
	publisher       *messaging.RabbitMQPublisher
	botRepo         repository.ITelegramBotRepository
	groupRepo       repository.ITelegramGroupRepository
	commandRepo     repository.ICustomCommandRepository
	subRepo         repository.ISubscriptionRepository
	router          *handler.Registry
	telegramFactory telegram.BotFactory
	redisClient     *redis.Client
	encryptionKey   string
	log             *zap.Logger
}

func NewTelegramWebhookUseCase(
	db *entity.Database,
	publisher *messaging.RabbitMQPublisher,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	commandRepo repository.ICustomCommandRepository,
	subRepo repository.ISubscriptionRepository,
	router *handler.Registry,
	telegramFactory telegram.BotFactory,
	redisClient *redis.Client,
	encryptionKey string,
	log *zap.Logger,
) ITelegramWebhookUseCase {
	return &TelegramWebhookUseCase{
		db:              db,
		publisher:       publisher,
		botRepo:         botRepo,
		groupRepo:       groupRepo,
		commandRepo:     commandRepo,
		subRepo:         subRepo,
		router:          router,
		telegramFactory: telegramFactory,
		redisClient:     redisClient,
		encryptionKey:   encryptionKey,
		log:             log,
	}
}

func (uc *TelegramWebhookUseCase) ProcessUpdate(ctx context.Context, botID uuid.UUID, update *tgbotapi.Update) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("telegram webhook process update start", zap.String("bot_id", botID.String()), zap.Int("update_id", update.UpdateID))

	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil {
		log.Error("failed to find bot", zap.Error(err))
		return err
	}
	if bot == nil {
		log.Warn("bot not found", zap.String("bot_id", botID.String()))
		return nil
	}

	// Switch bot context if the incoming update belongs to a group connected to a different Bot record
	var chatID int64
	if update.Message != nil {
		chatID = update.Message.Chat.ID
	} else if update.MyChatMember != nil {
		chatID = update.MyChatMember.Chat.ID
	} else if update.ChatJoinRequest != nil {
		chatID = update.ChatJoinRequest.Chat.ID
	} else if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		chatID = update.CallbackQuery.Message.Chat.ID
	}

	if chatID != 0 {
		isConnectOrTransfer := update.Message != nil && update.Message.IsCommand() && (update.Message.Command() == "connect" || update.Message.Command() == "transfer")
		isCallbackConnectOrTransfer := update.CallbackQuery != nil && (strings.HasPrefix(update.CallbackQuery.Data, "connect_ok:") ||
			strings.HasPrefix(update.CallbackQuery.Data, "transfer_ok:") ||
			update.CallbackQuery.Data == "connect_cancel" ||
			update.CallbackQuery.Data == "transfer_cancel")

		if !isConnectOrTransfer && !isCallbackConnectOrTransfer {
			group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, chatID)
			if err == nil && group != nil {
				if group.BotUUID != bot.ID {
					groupBot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, group.BotUUID)
					if err == nil && groupBot != nil {
						log.Info("switching bot context to group bot", zap.String("original_bot_id", bot.ID.String()), zap.String("group_bot_id", groupBot.ID.String()))
						bot = groupBot
					}
				}
			}
		}
	}

	botRole := bot.BotRole
	if botRole == "" {
		botRole = "all_in_one"
	}

	// 1. Handle Chat Join Request (Event)
	if update.ChatJoinRequest != nil {
		if botRole != "sales_only" {
			return uc.handleChatJoinRequest(ctx, bot, update.ChatJoinRequest)
		}
		log.Debug("sales_only bot ignoring chat join request")
	}

	// 2. Handle Bot Added/Removed from Group (Event)
	if update.MyChatMember != nil {
		if botRole != "sales_only" {
			return uc.handleMyChatMember(ctx, bot, update.MyChatMember)
		}
		log.Debug("sales_only bot ignoring my chat member")
	}

	// 3. Handle Messages via Central Routing Engine
	if update.Message != nil {
		if update.Message.From == nil {
			log.Debug("received message update without sender (likely channel post), skipping command routing",
				zap.Int64("chat_id", update.Message.Chat.ID),
			)
		} else {
			log.Debug("received message update, routing to handler", zap.Int64("chat_id", update.Message.Chat.ID), zap.Int64("user_id", update.Message.From.ID))

			// Intercept /connect and /transfer commands
			if update.Message.IsCommand() {
				cmd := update.Message.Command()
				if cmd == "connect" {
					return uc.handleConnectCommand(ctx, bot, update.Message)
				}
				if cmd == "transfer" {
					return uc.handleTransferCommand(ctx, bot, update.Message)
				}
			}

			handled, err := uc.router.HandleCommand(ctx, bot, update.Message)
			if err != nil {
				return err
			}
			if !handled && update.Message.IsCommand() {
				// Fallback to custom command
				return uc.handleCustomCommand(ctx, bot, update.Message)
			}
			return nil
		}
	}

	// 4. Handle Callback Queries via Central Routing Engine
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Message != nil {
			log.Debug("received callback query update, routing to handler", zap.Int64("chat_id", update.CallbackQuery.Message.Chat.ID), zap.Int64("user_id", update.CallbackQuery.From.ID))
		} else {
			log.Debug("received callback query without message context, routing to handler", zap.Int64("user_id", update.CallbackQuery.From.ID))
		}

		data := update.CallbackQuery.Data
		if strings.HasPrefix(data, "connect_ok:") || strings.HasPrefix(data, "transfer_ok:") || data == "connect_cancel" || data == "transfer_cancel" {
			return uc.handleConnectCallback(ctx, bot, update.CallbackQuery)
		}

		return uc.router.HandleCallback(ctx, bot, update.CallbackQuery)
	}

	log.Debug("telegram webhook ignored update type", zap.Int("update_id", update.UpdateID))
	return nil
}

func (uc *TelegramWebhookUseCase) handleChatJoinRequest(ctx context.Context, bot *entity.TelegramBot, req *tgbotapi.ChatJoinRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("received chat join request",
		zap.String("bot_id", bot.ID.String()),
		zap.Int64("chat_id", req.Chat.ID),
		zap.Int64("user_id", req.From.ID),
	)

	// Gatekeeping Logic (Publish to RabbitMQ)
	tgUserID := req.From.ID
	chatID := req.Chat.ID

	// Create payload
	payload := map[string]interface{}{
		"bot_id":           bot.ID,
		"telegram_user_id": tgUserID,
		"telegram_chat_id": chatID,
	}

	// Publish to RabbitMQ
	if err := uc.publisher.PublishGatekeeping(ctx, payload); err != nil {
		log.Error("failed to publish gatekeeping task", zap.Error(err))
		return err // Webhook will be retried by Telegram if 500
	}

	log.Info("gatekeeping task published successfully", zap.Int64("user_id", tgUserID))

	return nil
}

func (uc *TelegramWebhookUseCase) handleMyChatMember(ctx context.Context, bot *entity.TelegramBot, update *tgbotapi.ChatMemberUpdated) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("received my chat member update",
		zap.String("bot_id", bot.ID.String()),
		zap.Int64("chat_id", update.Chat.ID),
		zap.String("new_status", update.NewChatMember.Status),
	)

	status := update.NewChatMember.Status
	chatID := update.Chat.ID
	chatTitle := update.Chat.Title

	// Cek apakah grup sudah ada di database
	group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, chatID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("failed to fetch group by telegram id", zap.Error(err))
		return err
	}

	if status == "member" || status == "administrator" {
		if group == nil {
			log.Info("bot added to group but group is not registered yet, ignoring auto-creation", zap.Int64("chat_id", chatID))
		} else {
			// Update grup yang ada
			group.Name = chatTitle
			group.IsActive = true
			group.InactiveReason = ""
			if err := uc.groupRepo.Update(ctx, uc.db.Gorm, group); err != nil {
				log.Error("failed to update existing group on rejoin", zap.Error(err))
				return err
			}
			log.Info("activated existing group", zap.Int64("chat_id", chatID))
		}
	} else if status == "kicked" || status == "left" {
		if group != nil {
			group.IsActive = false
			group.InactiveReason = "Bot was kicked or left the group"
			if err := uc.groupRepo.Update(ctx, uc.db.Gorm, group); err != nil {
				log.Error("failed to mark group as inactive", zap.Error(err))
				return err
			}
			log.Info("marked group as inactive", zap.Int64("chat_id", chatID))
		}
	}

	return nil
}

func (uc *TelegramWebhookUseCase) handleCustomCommand(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	log := logger.FromContext(ctx, uc.log)

	trigger := "/" + msg.Command()
	log.Info("handling custom command", zap.String("trigger", trigger))

	cmd, err := uc.commandRepo.FindByBotIDAndTrigger(ctx, uc.db.Gorm, bot.ID, trigger)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Debug("custom command not found", zap.String("trigger", trigger))
			return nil
		}
		log.Error("failed to find custom command", zap.Error(err))
		return err
	}

	if ok, reason := uc.canExecuteCustomCommand(ctx, cmd, bot, msg); !ok {
		if reason != "" {
			_ = uc.sendReply(ctx, bot, msg.Chat.ID, reason)
		}
		return nil
	}

	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		log.Error("failed to decrypt bot token", zap.Error(err))
		return err
	}

	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		log.Error("failed to get telegram client", zap.Error(err))
		return err
	}

	if cmd.ResponseType == "text" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, cmd.ResponseText)
		reply.ParseMode = tgbotapi.ModeHTML
		_, err = botClient.Send(ctx, reply)
		if err != nil {
			log.Error("failed to send custom text command", zap.Error(err))
			return err
		}
	} else if cmd.ResponseType == "photo" {
		var err error
		var sentMsg tgbotapi.Message

		if cmd.TelegramFileID != nil && *cmd.TelegramFileID != "" {
			// Reuse existing file_id
			reply := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileID(*cmd.TelegramFileID))
			reply.Caption = cmd.ResponseText
			reply.ParseMode = tgbotapi.ModeHTML
			sentMsg, err = botClient.Send(ctx, reply)
			if err != nil {
				log.Warn("failed to send custom photo command with file_id, falling back to URL", zap.Error(err))
				err = nil
				cmd.TelegramFileID = nil
			}
		}

		// Fallback ke URL jika file_id tidak ada atau gagal
		if (cmd.TelegramFileID == nil || *cmd.TelegramFileID == "") && cmd.FileUrl != nil && *cmd.FileUrl != "" {
			reply := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileURL(*cmd.FileUrl))
			reply.Caption = cmd.ResponseText
			reply.ParseMode = tgbotapi.ModeHTML
			sentMsg, err = botClient.Send(ctx, reply)
			if err != nil {
				log.Error("failed to send custom photo command with url", zap.Error(err))
				return err
			}

			// Save file_id for future use
			if sentMsg.Photo != nil && len(sentMsg.Photo) > 0 {
				largestPhoto := sentMsg.Photo[len(sentMsg.Photo)-1]
				fileID := largestPhoto.FileID
				cmd.TelegramFileID = &fileID
				if errUpdate := uc.commandRepo.Update(ctx, uc.db.Gorm, cmd); errUpdate != nil {
					log.Error("failed to save telegram_file_id", zap.Error(errUpdate))
				}
			}
		}

		// Fallback ke teks jika tidak ada gambar sama sekali
		if (cmd.TelegramFileID == nil || *cmd.TelegramFileID == "") && (cmd.FileUrl == nil || *cmd.FileUrl == "") {
			reply := tgbotapi.NewMessage(msg.Chat.ID, cmd.ResponseText)
			reply.ParseMode = tgbotapi.ModeHTML
			_, err = botClient.Send(ctx, reply)
			if err != nil {
				return err
			}
		}
	} else if cmd.ResponseType == "document" {
		var err error
		var sentMsg tgbotapi.Message

		if cmd.TelegramFileID != nil && *cmd.TelegramFileID != "" {
			// Reuse existing file_id
			reply := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FileID(*cmd.TelegramFileID))
			reply.Caption = cmd.ResponseText
			reply.ParseMode = tgbotapi.ModeHTML
			sentMsg, err = botClient.Send(ctx, reply)
			if err != nil {
				log.Warn("failed to send custom document command with file_id, falling back to URL", zap.Error(err))
				err = nil
				cmd.TelegramFileID = nil
			}
		}

		// Fallback ke URL jika file_id tidak ada atau gagal
		if (cmd.TelegramFileID == nil || *cmd.TelegramFileID == "") && cmd.FileUrl != nil && *cmd.FileUrl != "" {
			reply := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FileURL(*cmd.FileUrl))
			reply.Caption = cmd.ResponseText
			reply.ParseMode = tgbotapi.ModeHTML
			sentMsg, err = botClient.Send(ctx, reply)
			if err != nil {
				log.Error("failed to send custom document command with url", zap.Error(err))
				return err
			}

			// Save file_id for future use
			if sentMsg.Document != nil {
				fileID := sentMsg.Document.FileID
				cmd.TelegramFileID = &fileID
				if errUpdate := uc.commandRepo.Update(ctx, uc.db.Gorm, cmd); errUpdate != nil {
					log.Error("failed to save telegram_file_id for document", zap.Error(errUpdate))
				}
			}
		}

		// Fallback ke teks jika tidak ada berkas sama sekali
		if (cmd.TelegramFileID == nil || *cmd.TelegramFileID == "") && (cmd.FileUrl == nil || *cmd.FileUrl == "") {
			reply := tgbotapi.NewMessage(msg.Chat.ID, cmd.ResponseText)
			_, err = botClient.Send(ctx, reply)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// canExecuteCustomCommand checks whether a user is allowed to execute a custom command.
func (uc *TelegramWebhookUseCase) canExecuteCustomCommand(ctx context.Context, cmd *entity.CustomCommand, bot *entity.TelegramBot, msg *tgbotapi.Message) (bool, string) {
	log := logger.FromContext(ctx, uc.log)

	switch cmd.ChatTypeScope {
	case "dm_only":
		if msg.Chat.Type != "private" {
			return false, "Perintah ini hanya bisa digunakan di chat pribadi (DM) dengan bot"
		}
	case "group_only":
		if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
			return false, "Perintah ini hanya bisa digunakan di dalam grup"
		}
	}

	if len(cmd.GroupIDs) > 0 {
		groups, err := uc.groupRepo.FindByIDs(ctx, uc.db.Gorm, parseUUIDs(cmd.GroupIDs))
		if err != nil {
			log.Error("failed to check group IDs for custom command", zap.Error(err))
			return false, ""
		}
		allowed := false
		for _, g := range groups {
			if g.TelegramChatID == msg.Chat.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "Perintah ini tidak tersedia di grup ini"
		}
	}

	switch cmd.AccessScope {
	case "admin":
		isAdmin, err := uc.isSenderAdmin(ctx, bot, msg.Chat.ID, msg.From.ID)
		if err != nil || !isAdmin {
			return false, "Perintah ini hanya untuk admin grup"
		}
	case "member":
		subs, err := uc.subRepo.FindActiveByTelegramUserID(ctx, uc.db.Gorm, msg.From.ID, bot.ClientID)
		if err != nil || len(subs) == 0 {
			return false, "Perintah ini hanya tersedia untuk member yang berlangganan"
		}
		if len(cmd.PackageIDs) > 0 {
			allowed := false
			for _, s := range subs {
				for _, pid := range parseUUIDs(cmd.PackageIDs) {
					if s.PackageID == pid {
						allowed = true
						break
					}
				}
				if allowed {
					break
				}
			}
			if !allowed {
				return false, "Anda belum membeli paket yang diperlukan untuk perintah ini"
			}
		}
	}

	return true, ""
}

func (uc *TelegramWebhookUseCase) handleConnectCommand(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	log := logger.FromContext(ctx, uc.log)

	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Perintah ini hanya dapat dijalankan di dalam grup Telegram yang ingin Anda hubungkan, silahkan masukan bot ke dalam group anda.")
	}

	token := msg.CommandArguments()
	if token == "" {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "Silakan sertakan kode koneksi. Contoh: <code>/connect KODE123</code>")
	}

	// Restrict to Admin
	isAdmin, err := uc.isSenderAdmin(ctx, bot, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.Error("failed to check if sender is admin", zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Gagal memvalidasi hak akses Anda di grup ini.")
	}
	if !isAdmin {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Hanya Administrator grup yang dapat menjalankan perintah ini.")
	}

	// Check if bot is admin in the group
	isBotAdmin, err := uc.isSenderAdmin(ctx, bot, msg.Chat.ID, bot.BotID)
	if err != nil || !isBotAdmin {
		log.Warn("failed to check if bot is admin", zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Bot ini belum menjadi administrator di grup. Silakan jadikan bot sebagai administrator grup terlebih dahulu, lalu coba lagi.")
	}

	// Verify token in Redis
	redisKey := fmt.Sprintf("connect_group:%s", token)
	val, err := uc.redisClient.Get(ctx, redisKey).Result()
	if err != nil {
		log.Warn("failed to get connect token from redis", zap.String("token", token), zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Kode koneksi tidak valid atau sudah kedaluwarsa.")
	}

	var payload struct {
		ClientID uuid.UUID `json:"client_id"`
		BotID    uuid.UUID `json:"bot_id"`
	}
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		log.Error("failed to unmarshal connect token payload", zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Terjadi kesalahan sistem.")
	}

	payloadBot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, payload.BotID)
	if err != nil || payloadBot == nil {
		log.Warn("bot not found for connect token payload", zap.String("bot_id", payload.BotID.String()))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Bot tidak ditemukan.")
	}

	if payloadBot.BotID != bot.BotID {
		log.Info("bot ID mismatch for connect token, ignoring silently", zap.Int64("expected", payloadBot.BotID), zap.Int64("actual", bot.BotID))
		return nil
	}

	// Switch bot context to the one that generated the connect token
	bot = payloadBot

	// Cek apakah grup sudah ada di database
	group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, msg.Chat.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("failed to fetch group by telegram id", zap.Error(err))
		return err
	}

	if group != nil {
		if group.ClientID != payload.ClientID {
			return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Grup ini sudah terdaftar oleh klien lain.")
		}

		if group.BotUUID != payload.BotID {
			return uc.sendReply(ctx, bot, msg.Chat.ID, fmt.Sprintf("❌ Grup ini sudah dikelola oleh bot lain. Untuk memindahkan pengelolaan ke bot ini (@%s), gunakan perintah: <code>/transfer %s</code>", bot.Username, token))
		}
	}

	// Send Confirmation Message with Inline Keyboard
	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		return err
	}
	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		return err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Hubungkan", "connect_ok:"+token),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "connect_cancel"),
		),
	)

	msgToSend := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("⚠️ <b>Konfirmasi Koneksi Grup</b>\n\nApakah Anda yakin ingin menghubungkan grup <b>%s</b> ke dashboard Bot <b>@%s</b>?", msg.Chat.Title, bot.Username))
	msgToSend.ParseMode = "HTML"
	msgToSend.ReplyMarkup = keyboard

	_, err = botClient.Send(ctx, msgToSend)
	return err
}

func (uc *TelegramWebhookUseCase) handleTransferCommand(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	log := logger.FromContext(ctx, uc.log)

	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Perintah ini hanya dapat dijalankan di dalam grup Telegram yang ingin Anda hubungkan, silahkan masukan bot ke dalam group anda.")
	}

	token := msg.CommandArguments()
	if token == "" {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "Silakan sertakan kode koneksi. Contoh: <code>/transfer KODE123</code>")
	}

	// Restrict to Admin
	isAdmin, err := uc.isSenderAdmin(ctx, bot, msg.Chat.ID, msg.From.ID)
	if err != nil {
		log.Error("failed to check if sender is admin", zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Gagal memvalidasi hak akses Anda di grup ini.")
	}
	if !isAdmin {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Hanya Administrator grup yang dapat menjalankan perintah ini.")
	}

	// Verify token in Redis
	redisKey := fmt.Sprintf("connect_group:%s", token)
	val, err := uc.redisClient.Get(ctx, redisKey).Result()
	if err != nil {
		log.Warn("failed to get connect token from redis for transfer", zap.String("token", token), zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Kode transfer tidak valid atau sudah kedaluwarsa.")
	}

	var payload struct {
		ClientID uuid.UUID `json:"client_id"`
		BotID    uuid.UUID `json:"bot_id"`
	}
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		log.Error("failed to unmarshal connect token payload for transfer", zap.Error(err))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Terjadi kesalahan sistem.")
	}

	payloadBot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, payload.BotID)
	if err != nil || payloadBot == nil {
		log.Warn("bot not found for connect token payload during transfer", zap.String("bot_id", payload.BotID.String()))
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Bot tidak ditemukan.")
	}

	if payloadBot.BotID != bot.BotID {
		log.Info("bot ID mismatch for transfer token, ignoring silently", zap.Int64("expected", payloadBot.BotID), zap.Int64("actual", bot.BotID))
		return nil
	}

	// Switch bot context to the one that generated the connect token
	bot = payloadBot

	// Cek apakah grup sudah ada di database
	group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, msg.Chat.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("failed to fetch group by telegram id for transfer", zap.Error(err))
		return err
	}

	if group == nil {
		return uc.sendReply(ctx, bot, msg.Chat.ID, fmt.Sprintf("❌ Grup ini belum pernah terdaftar di sistem. Silakan gunakan perintah <code>/connect %s</code> terlebih dahulu.", token))
	}

	if group.ClientID != payload.ClientID {
		return uc.sendReply(ctx, bot, msg.Chat.ID, "❌ Grup ini sudah terdaftar oleh klien lain.")
	}

	if group.BotUUID == payload.BotID {
		return uc.sendReply(ctx, bot, msg.Chat.ID, fmt.Sprintf("ℹ️ Grup ini sudah dikelola oleh bot ini (@%s).", bot.Username))
	}

	// Send Confirmation Message with Inline Keyboard
	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		return err
	}
	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		return err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Pindahkan", "transfer_ok:"+token),
			tgbotapi.NewInlineKeyboardButtonData("❌ Batalkan", "transfer_cancel"),
		),
	)

	msgToSend := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("⚠️ <b>Konfirmasi Pengalihan Grup</b>\n\nApakah Anda yakin ingin memindahkan pengelolaan grup <b>%s</b> ke Bot <b>@%s</b>?", msg.Chat.Title, bot.Username))
	msgToSend.ParseMode = "HTML"
	msgToSend.ReplyMarkup = keyboard

	_, err = botClient.Send(ctx, msgToSend)
	return err
}

func (uc *TelegramWebhookUseCase) handleConnectCallback(ctx context.Context, bot *entity.TelegramBot, cb *tgbotapi.CallbackQuery) error {
	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		return err
	}
	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		return err
	}

	// Restrict to Admin (check if the user clicking the button is admin)
	isAdmin, err := uc.isSenderAdmin(ctx, bot, cb.Message.Chat.ID, cb.From.ID)
	if err != nil || !isAdmin {
		callbackConfig := tgbotapi.NewCallback(cb.ID, "❌ Hanya Administrator yang dapat menekan tombol ini.")
		callbackConfig.ShowAlert = true
		_, _ = botClient.Request(ctx, callbackConfig)
		return nil
	}

	data := cb.Data

	// Handle Cancel
	if data == "connect_cancel" || data == "transfer_cancel" {
		callbackConfig := tgbotapi.NewCallback(cb.ID, "Koneksi dibatalkan.")
		_, _ = botClient.Request(ctx, callbackConfig)

		editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Koneksi dibatalkan.")
		_, err = botClient.Send(ctx, editMsg)
		return err
	}

	// Handle Confirm Connect
	if strings.HasPrefix(data, "connect_ok:") {
		token := strings.TrimPrefix(data, "connect_ok:")
		redisKey := fmt.Sprintf("connect_group:%s", token)
		val, err := uc.redisClient.Get(ctx, redisKey).Result()
		if err != nil {
			callbackConfig := tgbotapi.NewCallback(cb.ID, "❌ Kode koneksi kedaluwarsa.")
			callbackConfig.ShowAlert = true
			_, _ = botClient.Request(ctx, callbackConfig)

			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Koneksi gagal: Kode koneksi tidak valid atau sudah kedaluwarsa.")
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		var payload struct {
			ClientID uuid.UUID `json:"client_id"`
			BotID    uuid.UUID `json:"bot_id"`
		}
		if err := json.Unmarshal([]byte(val), &payload); err != nil {
			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Terjadi kesalahan sistem.")
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		// Cek apakah grup sudah ada di database
		group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, cb.Message.Chat.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if group != nil {
			if group.ClientID != payload.ClientID {
				editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Grup ini sudah terdaftar oleh klien lain.")
				_, err = botClient.Send(ctx, editMsg)
				return err
			}
			if group.BotUUID != payload.BotID {
				editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("❌ Grup ini sudah dikelola oleh bot lain. Gunakan perintah: /transfer %s", token))
				_, err = botClient.Send(ctx, editMsg)
				return err
			}

			group.IsActive = true
			group.InactiveReason = ""
			group.Name = cb.Message.Chat.Title
			if err := uc.groupRepo.Update(ctx, uc.db.Gorm, group); err != nil {
				return err
			}
		} else {
			newGroup := &entity.Group{
				ID:             uuid.New(),
				ClientID:       payload.ClientID,
				BotUUID:        payload.BotID,
				TelegramChatID: cb.Message.Chat.ID,
				Name:           cb.Message.Chat.Title,
				IsActive:       true,
			}
			if err := uc.groupRepo.Create(ctx, uc.db.Gorm, newGroup); err != nil {
				return err
			}
		}

		uc.redisClient.Set(ctx, fmt.Sprintf("connect_group_status:%s", token), "success", 5*time.Minute)
		uc.redisClient.Del(ctx, redisKey)

		// Answer callback and update UI
		callbackConfig := tgbotapi.NewCallback(cb.ID, "Grup berhasil terhubung!")
		_, _ = botClient.Request(ctx, callbackConfig)

		editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "✅ <b>Sukses!</b> Grup ini sekarang telah terhubung ke dashboard Anda.")
		editMsg.ParseMode = "HTML"
		_, err = botClient.Send(ctx, editMsg)
		return err
	}

	// Handle Confirm Transfer
	if strings.HasPrefix(data, "transfer_ok:") {
		token := strings.TrimPrefix(data, "transfer_ok:")
		redisKey := fmt.Sprintf("connect_group:%s", token)
		val, err := uc.redisClient.Get(ctx, redisKey).Result()
		if err != nil {
			callbackConfig := tgbotapi.NewCallback(cb.ID, "❌ Kode transfer kedaluwarsa.")
			callbackConfig.ShowAlert = true
			_, _ = botClient.Request(ctx, callbackConfig)

			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Pengalihan gagal: Kode transfer tidak valid atau sudah kedaluwarsa.")
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		var payload struct {
			ClientID uuid.UUID `json:"client_id"`
			BotID    uuid.UUID `json:"bot_id"`
		}
		if err := json.Unmarshal([]byte(val), &payload); err != nil {
			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Terjadi kesalahan sistem.")
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		// Cek apakah grup sudah ada di database
		group, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, cb.Message.Chat.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if group == nil {
			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("❌ Grup ini belum terdaftar. Silakan gunakan perintah /connect %s", token))
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		if group.ClientID != payload.ClientID {
			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, "❌ Grup ini sudah terdaftar oleh klien lain.")
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		if group.BotUUID == payload.BotID {
			editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, fmt.Sprintf("ℹ️ Grup ini sudah dikelola oleh bot ini (@%s).", bot.Username))
			_, err = botClient.Send(ctx, editMsg)
			return err
		}

		var oldBotUsername string
		oldBot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, group.BotUUID)
		if err == nil && oldBot != nil {
			oldBotUsername = oldBot.Username
		}

		group.IsActive = true
		group.InactiveReason = ""
		group.Name = cb.Message.Chat.Title
		group.BotUUID = payload.BotID
		if err := uc.groupRepo.Update(ctx, uc.db.Gorm, group); err != nil {
			return err
		}

		uc.redisClient.Set(ctx, fmt.Sprintf("connect_group_status:%s", token), "success", 5*time.Minute)
		uc.redisClient.Del(ctx, redisKey)

		// Answer callback and update UI
		callbackConfig := tgbotapi.NewCallback(cb.ID, "Pengalihan berhasil!")
		_, _ = botClient.Request(ctx, callbackConfig)

		replyText := fmt.Sprintf("✅ <b>Pengalihan Berhasil!</b> Pengelolaan grup ini telah dipindahkan dari @%s ke @%s.", oldBotUsername, bot.Username)
		if oldBotUsername == "" {
			replyText = fmt.Sprintf("✅ <b>Pengalihan Berhasil!</b> Pengelolaan grup ini telah dipindahkan ke @%s.", bot.Username)
		}

		editMsg := tgbotapi.NewEditMessageText(cb.Message.Chat.ID, cb.Message.MessageID, replyText)
		editMsg.ParseMode = "HTML"
		_, err = botClient.Send(ctx, editMsg)
		return err
	}

	return nil
}

func (uc *TelegramWebhookUseCase) isSenderAdmin(ctx context.Context, bot *entity.TelegramBot, chatID int64, userID int64) (bool, error) {
	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		return false, err
	}
	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		return false, err
	}
	member, err := botClient.GetChatMember(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	return member.Status == "creator" || member.Status == "administrator", nil
}

func (uc *TelegramWebhookUseCase) sendReply(ctx context.Context, bot *entity.TelegramBot, chatID int64, text string) error {
	decryptedToken, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		uc.log.Error("failed to decrypt bot token for reply", zap.Error(err))
		return err
	}
	botClient, err := uc.telegramFactory.NewClient(decryptedToken)
	if err != nil {
		return err
	}
	return botClient.SendMessage(ctx, chatID, text)
}

func parseUUIDs(ids []string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		result = append(result, parsed)
	}
	return result
}
