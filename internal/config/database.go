package config

import (
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDatabase initializes both GORM and sqlx database connections.
func NewDatabase(cfg *DatabaseConfig, logger *zap.Logger) (*entity.Database, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	logger.Info("database: connecting via gorm",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.Name),
		zap.String("sslmode", cfg.SSLMode),
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("database: failed to connect with gorm: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("database: failed to get sql.DB from gorm: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	logger.Info("database: gorm connected, initializing sqlx")

	sqlxDB := sqlx.NewDb(sqlDB, "postgres")
	if err := sqlxDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: failed to ping with sqlx: %w", err)
	}

	logger.Info("database: all connections established successfully")

	return &entity.Database{
		Gorm: gormDB,
		Sqlx: sqlxDB,
	}, nil
}

// CloseDatabase gracefully closes database connections.
func CloseDatabase(db *entity.Database, logger *zap.Logger) {
	if db == nil {
		return
	}

	if db.Sqlx != nil {
		if err := db.Sqlx.Close(); err != nil {
			logger.Error("database: failed to close sqlx", zap.Error(err))
		}
	}

	if db.Gorm != nil {
		sqlDB, err := db.Gorm.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				logger.Error("database: failed to close gorm", zap.Error(err))
			}
		}
	}

	logger.Info("database: connections closed")
}
