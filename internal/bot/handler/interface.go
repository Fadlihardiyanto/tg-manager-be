package handler

import (
	"context"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CommandHandler menangani text commands (e.g., /start, /bsh)
type CommandHandler interface {
	Name() string
	AllowedRoles() []string // Peran bot yang diizinkan (e.g., []string{"all_in_one", "sales_only"})
	Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error
}

// requirePrivateChat memblokir command yang hanya boleh dijalankan di chat
// pribadi dengan bot. Alasannya produk: Telegram hanya mengizinkan bot
// mengirim DM ke user yang PERNAH chat bot tersebut — command jualan
// (/packages, /mysubs) harus memaksa user berinteraksi privat dengan bot,
// supaya DM aktivasi/invite link bisa terkirim setelah pembelian.
// Mengembalikan (false, nil) setelah membalas instruksi di grup.
func requirePrivateChat(ctx context.Context, factory telegram.BotFactory, bot *entity.TelegramBot, msg *tgbotapi.Message, encKey string) (bool, error) {
	if msg.Chat.Type == "private" {
		return true, nil
	}

	token, err := crypto.Decrypt(bot.Token, encKey)
	if err != nil {
		return false, err
	}
	client, err := factory.NewClient(token)
	if err != nil {
		return false, err
	}

	reply := fmt.Sprintf("⚠️ Perintah /%s hanya dapat digunakan di DM (chat pribadi) dengan bot.\n\n"+
		"Silakan buka chat bot ini di Telegram dan kirim perintah tersebut di sana.", msg.Command())
	_ = client.SendMessage(ctx, msg.Chat.ID, reply)
	return false, nil
}
