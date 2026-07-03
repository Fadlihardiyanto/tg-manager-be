package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	Worker   WorkerConfig
	Telegram TelegramConfig
	JWT      JWTAppConfig
	SMTP     SMTPConfig
	Midtrans MidtransConfig
	S3       S3Config
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name          string
	Env           string
	Debug         bool
	Port          int
	Timezone      string
	EncryptionKey  string // 32-bytes key for AES-256
	BaseURL        string
	FrontendURL    string
	AllowedOrigin  string
	BcryptCost     int
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RabbitMQConfig holds RabbitMQ connection settings.
type RabbitMQConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	VHost          string
	ReconnectDelay time.Duration
	MaxRetries     int
}

// DSN returns the AMQP connection string.
func (c RabbitMQConfig) DSN() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		c.User, c.Password, c.Host, c.Port, c.VHost)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// WorkerConfig holds worker-specific settings.
type WorkerConfig struct {
	Concurrency   int
	PrefetchCount int
}

// TelegramConfig holds Telegram API settings.
type TelegramConfig struct {
	WebhookSecret  string
	WebhookBaseURL string
	APITimeout     int
	RateLimit      int
}

// JWTAppConfig holds JWT settings for both Admin and Tenant.
// Admin and Tenant use SEPARATE secrets for security isolation.
type JWTAppConfig struct {
	// Admin JWT (Superadmin panel)
	AdminSecretKey     string
	AdminAccessExpiry  time.Duration
	AdminRefreshExpiry time.Duration

	// Tenant JWT (Client dashboard)
	TenantSecretKey     string
	TenantAccessExpiry  time.Duration
	TenantRefreshExpiry time.Duration

	Issuer string
}

// SMTPConfig holds SMTP settings for sending emails (OTP, notifications).
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type MidtransConfig struct {
	ServerKey string `mapstructure:"server_key"`
	ClientKey string `mapstructure:"client_key"`
	BaseURL   string `mapstructure:"base_url"`
	SnapURL   string `mapstructure:"snap_url"`
}

// S3Config holds S3-compatible storage settings (Cloudflare R2, MinIO, AWS S3, etc.).
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Region          string
	UsePathStyle    bool   // true for MinIO, false for R2/AWS
	PublicURL       string // custom public CDN/domain for serving files (e.g. https://cdn.example.com)
}

// LoadConfig reads configuration from .env file and environment variables.
func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: failed to read .env file: %w", err)
	}

	connMaxLifetime := viper.GetInt("DB_CONN_MAX_LIFETIME")
	if connMaxLifetime == 0 {
		connMaxLifetime = 300
	}

	cfg := &Config{
		App: AppConfig{
			Name:          viper.GetString("APP_NAME"),
			Env:           viper.GetString("APP_ENV"),
			Debug:         viper.GetBool("APP_DEBUG"),
			Port:          viper.GetInt("APP_PORT"),
			Timezone:      viper.GetString("APP_TIMEZONE"),
			EncryptionKey: viper.GetString("APP_ENCRYPTION_KEY"),
			BaseURL:       viper.GetString("APP_BASE_URL"),
			FrontendURL:   viper.GetString("APP_FRONTEND_URL"),
			AllowedOrigin: viper.GetString("ALLOWED_ORIGIN"),
			BcryptCost:    viper.GetInt("BCRYPT_COST"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetInt("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			Name:            viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSLMODE"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: time.Duration(connMaxLifetime) * time.Second,
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetInt("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
			PoolSize: viper.GetInt("REDIS_POOL_SIZE"),
		},
		RabbitMQ: RabbitMQConfig{
			Host:           viper.GetString("RABBITMQ_HOST"),
			Port:           viper.GetInt("RABBITMQ_PORT"),
			User:           viper.GetString("RABBITMQ_USER"),
			Password:       viper.GetString("RABBITMQ_PASSWORD"),
			VHost:          viper.GetString("RABBITMQ_VHOST"),
			ReconnectDelay: 5 * time.Second,
			MaxRetries:     0, // unlimited
		},
		Worker: WorkerConfig{
			Concurrency:   viper.GetInt("WORKER_CONCURRENCY"),
			PrefetchCount: viper.GetInt("WORKER_PREFETCH_COUNT"),
		},
		Telegram: TelegramConfig{
			WebhookSecret:  viper.GetString("TELEGRAM_WEBHOOK_SECRET"),
			WebhookBaseURL: viper.GetString("TELEGRAM_WEBHOOK_BASE_URL"),
			APITimeout:     viper.GetInt("TELEGRAM_API_TIMEOUT"),
			RateLimit:      viper.GetInt("TELEGRAM_RATE_LIMIT_PER_SECOND"),
		},
		JWT: JWTAppConfig{
			AdminSecretKey:      viper.GetString("ADMIN_JWT_SECRET"),
			AdminAccessExpiry:   viper.GetDuration("ADMIN_JWT_ACCESS_EXPIRY"),
			AdminRefreshExpiry:  viper.GetDuration("ADMIN_JWT_REFRESH_EXPIRY"),
			TenantSecretKey:     viper.GetString("JWT_SECRET"),
			TenantAccessExpiry:  viper.GetDuration("JWT_EXPIRATION"),
			TenantRefreshExpiry: viper.GetDuration("JWT_REFRESH_EXPIRATION"),
			Issuer:              viper.GetString("APP_NAME"),
		},
		SMTP: SMTPConfig{
			Host:      viper.GetString("SMTP_HOST"),
			Port:      viper.GetInt("SMTP_PORT"),
			Username:  viper.GetString("SMTP_USERNAME"),
			Password:  viper.GetString("SMTP_PASSWORD"),
			FromEmail: viper.GetString("SMTP_FROM_EMAIL"),
			FromName:  viper.GetString("SMTP_FROM_NAME"),
		},
		Midtrans: MidtransConfig{
			ServerKey: viper.GetString("MIDTRANS_SERVER_KEY"),
			ClientKey: viper.GetString("MIDTRANS_CLIENT_KEY"),
			BaseURL:   viper.GetString("MIDTRANS_BASE_URL"),
			SnapURL:   viper.GetString("MIDTRANS_SNAP_URL"),
		},
		S3: S3Config{
			Endpoint:        viper.GetString("S3_ENDPOINT"),
			AccessKeyID:     viper.GetString("S3_ACCESS_KEY_ID"),
			SecretAccessKey: viper.GetString("S3_SECRET_ACCESS_KEY"),
			BucketName:      viper.GetString("S3_BUCKET_NAME"),
			Region:          viper.GetString("S3_REGION"),
			UsePathStyle:    viper.GetBool("S3_USE_PATH_STYLE"),
			PublicURL:       viper.GetString("S3_PUBLIC_URL"),
		},
	}

	// Set defaults
	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.App.BcryptCost == 0 {
		cfg.App.BcryptCost = 12
	}
	if cfg.App.EncryptionKey == "" {
		return nil, fmt.Errorf("APP_ENCRYPTION_KEY tidak dikonfigurasi — wajib di-set di environment")
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 10
	}
	if cfg.RabbitMQ.Port == 0 {
		cfg.RabbitMQ.Port = 5672
	}
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = 10
	}
	if cfg.Worker.PrefetchCount == 0 {
		cfg.Worker.PrefetchCount = 5
	}
	if cfg.Telegram.APITimeout == 0 {
		cfg.Telegram.APITimeout = 30
	}
	if cfg.Telegram.RateLimit == 0 {
		cfg.Telegram.RateLimit = 25
	}

	// JWT Defaults
	if cfg.JWT.AdminSecretKey == "" {
		return nil, fmt.Errorf("ADMIN_JWT_SECRET tidak dikonfigurasi — wajib di-set di environment")
	}
	if cfg.JWT.AdminAccessExpiry == 0 {
		cfg.JWT.AdminAccessExpiry = 15 * time.Minute
	}
	if cfg.JWT.AdminRefreshExpiry == 0 {
		cfg.JWT.AdminRefreshExpiry = 24 * time.Hour
	}
	if cfg.JWT.TenantSecretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET (tenant) tidak dikonfigurasi — wajib di-set di environment")
	}
	if cfg.JWT.TenantAccessExpiry == 0 {
		cfg.JWT.TenantAccessExpiry = 24 * time.Hour
	}
	if cfg.JWT.TenantRefreshExpiry == 0 {
		cfg.JWT.TenantRefreshExpiry = 7 * 24 * time.Hour
	}
	if cfg.JWT.Issuer == "" {
		cfg.JWT.Issuer = cfg.App.Name
	}

	// SMTP Defaults
	if cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
	}

	// Midtrans Defaults
	if cfg.Midtrans.BaseURL == "" {
		if viper.GetBool("MIDTRANS_IS_PRODUCTION") {
			cfg.Midtrans.BaseURL = "https://api.midtrans.com"
		} else {
			cfg.Midtrans.BaseURL = "https://api.sandbox.midtrans.com"
		}
	}
	if cfg.Midtrans.SnapURL == "" {
		if viper.GetBool("MIDTRANS_IS_PRODUCTION") {
			cfg.Midtrans.SnapURL = "https://app.midtrans.com/snap/v1"
		} else {
			cfg.Midtrans.SnapURL = "https://app.sandbox.midtrans.com/snap/v1"
		}
	}

	// S3 Defaults
	if cfg.S3.Region == "" {
		cfg.S3.Region = "auto" // Cloudflare R2 uses "auto"
	}
	if cfg.S3.BucketName == "" {
		cfg.S3.BucketName = "tg-manager-storage"
	}

	return cfg, nil
}
