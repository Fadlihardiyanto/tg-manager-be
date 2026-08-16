package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/config"
	"go.uber.org/zap"
)

// runWorkerWithRestart runs a worker in a loop: a normal Start return (ctx
// cancelled) exits; a panic is recovered, logged, and the worker restarts
// after a backoff instead of dying permanently.
func runWorkerWithRestart(name string, log *zap.Logger, ctx context.Context, start func()) {
	const backoff = 5 * time.Second
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("worker: "+name+" panicked", zap.Any("panic", r))
				}
			}()
			start()
		}()
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

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
		go runWorkerWithRestart("outbox", bootstrapConfig.Log, outboxCtx, func() {
			bootstrapConfig.OutboxWorker.Start(outboxCtx, 5*time.Second)
		})
	}

	// 5. Start Order Cleanup Worker
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	if bootstrapConfig.OrderCleanupWorker != nil {
		go runWorkerWithRestart("order-cleanup", bootstrapConfig.Log, cleanupCtx, func() {
			bootstrapConfig.OrderCleanupWorker.Start(cleanupCtx, 10*time.Minute)
		})
	}

	// 6. Start Enforcer Worker
	enforcerCtx, enforcerCancel := context.WithCancel(context.Background())
	defer enforcerCancel()
	if bootstrapConfig.EnforcerWorker != nil {
		go runWorkerWithRestart("enforcer", bootstrapConfig.Log, enforcerCtx, func() {
			bootstrapConfig.EnforcerWorker.Start(enforcerCtx, 1*time.Hour)
		})
	}

	// 7. Start Group Sync Worker
	groupSyncCtx, groupSyncCancel := context.WithCancel(context.Background())
	defer groupSyncCancel()
	if bootstrapConfig.GroupSyncWorker != nil {
		go runWorkerWithRestart("group-sync", bootstrapConfig.Log, groupSyncCtx, func() {
			bootstrapConfig.GroupSyncWorker.Start(groupSyncCtx, 1*time.Hour)
		})
	}

	// 8. Start Expiry Reminder Worker (polls every hour)
	expiryReminderCtx, expiryReminderCancel := context.WithCancel(context.Background())
	defer expiryReminderCancel()
	if bootstrapConfig.ExpiryReminderWorker != nil {
		go runWorkerWithRestart("expiry-reminder", bootstrapConfig.Log, expiryReminderCtx, func() {
			bootstrapConfig.ExpiryReminderWorker.Start(expiryReminderCtx, 1*time.Minute)
		})
	}

	// 9. Start Broadcast Scheduler Worker (polls every 30 seconds)
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()
	if bootstrapConfig.BroadcastSchedulerWorker != nil {
		go runWorkerWithRestart("broadcast-scheduler", bootstrapConfig.Log, schedulerCtx, func() {
			bootstrapConfig.BroadcastSchedulerWorker.Start(schedulerCtx, 30*time.Second)
		})
	}

	bootstrapConfig.Log.Info("worker: starting consumer...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				bootstrapConfig.Log.Error("worker: consumer panicked", zap.Any("panic", r))
				errCh <- fmt.Errorf("consumer panicked: %v", r)
			}
		}()
		errCh <- bootstrapConfig.Consumer.Start(ctx)
	}()

	// 6. Graceful shutdown — wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	bootstrapConfig.Log.Info("worker: received shutdown signal")
	groupSyncCancel()      // Stop group sync worker
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
