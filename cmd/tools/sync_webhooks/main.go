package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type TelegramBot struct {
	ID       string `gorm:"type:uuid;primaryKey"`
	Token    string `gorm:"type:text;not null"`
	IsActive bool   `gorm:"default:true"`
}

func main() {
	// Load .env if available
	_ = godotenv.Load(".env")

	baseURL := os.Getenv("TELEGRAM_WEBHOOK_BASE_URL")
	if baseURL == "" {
		log.Fatal("TELEGRAM_WEBHOOK_BASE_URL is empty in .env")
	}
	webhookSecret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)
	if os.Getenv("DB_HOST") == "" {
		// Fallback for local dev if .env failed to load
		dsn = "host=localhost port=5433 user=tgmanager password=pass dbname=tgmanager_db sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database: ", err)
	}

	var bots []TelegramBot
	db.Where("is_active = ?", true).Find(&bots)

	if len(bots) == 0 {
		log.Println("No active bots found in database.")
		return
	}

	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		encKey = os.Getenv("APP_ENCRYPTION_KEY")
	}
	if encKey == "" {
		log.Fatal("ENCRYPTION_KEY tidak dikonfigurasi di environment")
	}

	fmt.Println("========================================")
	fmt.Printf("Syncing %d bot(s) to new URL: %s\n", len(bots), baseURL)
	fmt.Println("========================================")

	successCount := 0
	for _, bot := range bots {
		token, err := crypto.Decrypt(bot.Token, encKey)
		if err != nil {
			log.Printf("[❌] Bot %s: Failed to decrypt token (check your encryption key)\n", bot.ID)
			continue
		}

		botAPI, err := tgbotapi.NewBotAPI(token)
		if err != nil {
			log.Printf("[❌] Bot %s: Failed to initialize Telegram API\n", bot.ID)
			continue
		}

		webhookURL := fmt.Sprintf("%s/webhooks/telegram/%s", baseURL, bot.ID)
		params := make(tgbotapi.Params)
		params.AddNonEmpty("url", webhookURL)
		if webhookSecret != "" {
			params.AddNonEmpty("secret_token", webhookSecret)
		}

		resp, err := botAPI.MakeRequest("setWebhook", params)
		if err != nil || !resp.Ok {
			log.Printf("[❌] Bot %s: Telegram API Error: %v (Response: %s)\n", bot.ID, err, string(resp.Result))
			continue
		}

		fmt.Printf("[✅] Bot %s -> Webhook updated successfully!\n", bot.ID)
		successCount++
	}

	fmt.Println("========================================")
	fmt.Printf("Finished! %d/%d webhooks synced.\n", successCount, len(bots))
}
