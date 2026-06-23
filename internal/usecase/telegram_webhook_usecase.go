package usecase

import (
	"context"
	"errors"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/bot/handler"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/gateway/messaging"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
)

type ITelegramWebhookUseCase interface {
	ProcessUpdate(ctx context.Context, botID uuid.UUID, update *tgbotapi.Update) error
}

type TelegramWebhookUseCase struct {
	db        *entity.Database
	publisher *messaging.RabbitMQPublisher
	botRepo     repository.ITelegramBotRepository
	groupRepo   repository.ITelegramGroupRepository
	commandRepo repository.ICustomCommandRepository
	router        *handler.Registry
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log           *zap.Logger
}

func NewTelegramWebhookUseCase(
	db *entity.Database,
	publisher *messaging.RabbitMQPublisher,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	commandRepo repository.ICustomCommandRepository,
	router *handler.Registry,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	log *zap.Logger,
) ITelegramWebhookUseCase {
	return &TelegramWebhookUseCase{
		db:        db,
		publisher: publisher,
		botRepo:     botRepo,
		groupRepo:   groupRepo,
		commandRepo:   commandRepo,
		router:        router,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		log:           log,
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
			// Buat grup baru
			newGroup := &entity.Group{
				ClientID:       bot.ClientID,
				BotID:          bot.ID,
				TelegramChatID: chatID,
				Name:           chatTitle,
				IsActive:       true,
			}
			if err := uc.groupRepo.Create(ctx, uc.db.Gorm, newGroup); err != nil {
				log.Error("failed to auto-create group", zap.Error(err))
				return err
			}
			log.Info("auto-created new group", zap.Int64("chat_id", chatID))
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
			return nil // Abaikan jika tidak ada custom command
		}
		log.Error("failed to find custom command", zap.Error(err))
		return err
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
		_, err = botClient.Send(ctx, reply)
		if err != nil {
			log.Error("failed to send custom text command", zap.Error(err))
			return err
		}
	} else if cmd.ResponseType == "photo" {
		// Jika tipe photo
		if cmd.TelegramFileID != nil && *cmd.TelegramFileID != "" {
			// Reuse existing file_id
			reply := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileID(*cmd.TelegramFileID))
			reply.Caption = cmd.ResponseText
			_, err = botClient.Send(ctx, reply)
			if err != nil {
				log.Error("failed to send custom photo command with file_id", zap.Error(err))
				return err
			}
		} else if cmd.FileUrl != nil && *cmd.FileUrl != "" {
			// Upload dari URL untuk pertama kalinya
			reply := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileURL(*cmd.FileUrl))
			reply.Caption = cmd.ResponseText
			sentMsg, err := botClient.Send(ctx, reply)
			if err != nil {
				log.Error("failed to send custom photo command with url", zap.Error(err))
				return err
			}

			// Save file_id for future use
			if sentMsg.Photo != nil && len(sentMsg.Photo) > 0 {
				largestPhoto := sentMsg.Photo[len(sentMsg.Photo)-1]
				fileID := largestPhoto.FileID
				cmd.TelegramFileID = &fileID
				if err := uc.commandRepo.Update(ctx, uc.db.Gorm, cmd); err != nil {
					log.Error("failed to save telegram_file_id", zap.Error(err))
					// Tidak me-return error karena pesan sudah terkirim
				}
			}
		} else {
			// Fallback ke teks jika tidak ada gambar (meskipun response_type = photo)
			reply := tgbotapi.NewMessage(msg.Chat.ID, cmd.ResponseText)
			_, err = botClient.Send(ctx, reply)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
