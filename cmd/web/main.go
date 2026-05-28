package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/config"
	"go.uber.org/zap"
)

func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	// 2. Initialize infrastructure (Web needs: Fiber + Publisher only, no Consumer)
	bootstrapConfig, err := config.NewBootstrapConfig(cfg,
		config.WithFiber(),
		config.WithPublisher(),
	)
	if err != nil {
		panic(err)
	}
	defer bootstrapConfig.Shutdown()

	// 3. Wire dependencies (Repositories → UseCases → Controllers → Routes)
	config.BootstrapWeb(bootstrapConfig)

	// 4. Start Fiber server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.App.Port)
		bootstrapConfig.Log.Info("web: starting server", zap.String("addr", addr))
		if err := bootstrapConfig.App.Listen(addr); err != nil {
			bootstrapConfig.Log.Error("web: server error", zap.Error(err))
		}
	}()

	// 5. Graceful shutdown — wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	bootstrapConfig.Log.Info("web: received shutdown signal")
}
