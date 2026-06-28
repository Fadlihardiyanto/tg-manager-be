package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/config"
	"go.uber.org/zap"
)

func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	// 2. Initialize infrastructure (Worker needs Consumer + Publisher for Outbox Worker)
	bootstrapConfig, err := config.NewBootstrapConfig(cfg,
		config.WithConsumer(),
		config.WithPublisher(),
	)
	if err != nil {
		panic(err)
	}
	defer bootstrapConfig.Shutdown()

	// 3. Wire dependencies
	config.BootstrapWorker(bootstrapConfig)

	// 4. Start Outbox Worker
	outboxCtx, outboxCancel := context.WithCancel(context.Background())
	defer outboxCancel()
	if bootstrapConfig.OutboxWorker != nil {
		go bootstrapConfig.OutboxWorker.Start(outboxCtx, 5*time.Second)
	}

	// 5. Start Order Cleanup Worker
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	if bootstrapConfig.OrderCleanupWorker != nil {
		go bootstrapConfig.OrderCleanupWorker.Start(cleanupCtx, 10*time.Minute)
	}

	// 6. Start Enforcer Worker
	enforcerCtx, enforcerCancel := context.WithCancel(context.Background())
	defer enforcerCancel()
	if bootstrapConfig.EnforcerWorker != nil {
		go bootstrapConfig.EnforcerWorker.Start(enforcerCtx, 1*time.Hour)
	}

	// 7. Start Group Sync Worker
	groupSyncCtx, groupSyncCancel := context.WithCancel(context.Background())
	defer groupSyncCancel()
	if bootstrapConfig.GroupSyncWorker != nil {
		go bootstrapConfig.GroupSyncWorker.Start(groupSyncCtx, 24*time.Hour)
	}

	// 8. Start Expiry Reminder Worker (polls every hour)
	expiryReminderCtx, expiryReminderCancel := context.WithCancel(context.Background())
	defer expiryReminderCancel()
	if bootstrapConfig.ExpiryReminderWorker != nil {
		go bootstrapConfig.ExpiryReminderWorker.Start(expiryReminderCtx, 1*time.Hour)
	}

	// 9. Start Broadcast Scheduler Worker (polls every 30 seconds)
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()
	if bootstrapConfig.BroadcastSchedulerWorker != nil {
		go bootstrapConfig.BroadcastSchedulerWorker.Start(schedulerCtx, 30*time.Second)
	}

	bootstrapConfig.Log.Info("worker: starting consumer...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- bootstrapConfig.Consumer.Start(ctx)
	}()

	// 6. Graceful shutdown — wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	bootstrapConfig.Log.Info("worker: received shutdown signal")
	schedulerCancel()      // Stop broadcast scheduler worker
	expiryReminderCancel() // Stop expiry reminder worker
	enforcerCancel()       // Stop the enforcer worker loop
	cleanupCancel()        // Stop the cleanup worker loop first
	outboxCancel()         // Stop the outbox worker loop
	cancel()               // Stop consumer

	shutdownTimeout := 30 * time.Second
	select {
	case err := <-errCh:
		if err != nil {
			bootstrapConfig.Log.Error("worker: consumer error", zap.Error(err))
		}
	case <-time.After(shutdownTimeout):
		bootstrapConfig.Log.Error("worker: shutdown timeout exceeded, force exiting")
	}
}
