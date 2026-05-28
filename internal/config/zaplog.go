package config

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapLogger creates a new Zap logger configured based on the application environment.
func NewZapLogger(cfg *AppConfig) (*zap.Logger, error) {
	var zapCfg zap.Config

	if cfg.Debug {
		// Development: console-friendly, debug level, colored output
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		// Production: JSON format, info level, optimized for log aggregation
		zapCfg = zap.NewProductionConfig()
	}

	// Common settings
	zapCfg.EncoderConfig.TimeKey = "timestamp"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.EncoderConfig.CallerKey = "caller"
	zapCfg.EncoderConfig.MessageKey = "message"

	logger, err := zapCfg.Build(
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.FatalLevel),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}
