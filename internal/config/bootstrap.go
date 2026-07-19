package config

import (
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/bot/handler"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/controller"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/route"
	deliveryMsg "github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/messaging"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	gatewayMsg "github.com/Fadlihardiyanto/telegram-management-app/internal/gateway/messaging"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/mailer"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/otp"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/pdf"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rabbitmq"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/ratelimit"
	pkg_s3 "github.com/Fadlihardiyanto/telegram-management-app/pkg/s3"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// BootstrapConfig holds all initialized infrastructure dependencies.
// This struct is passed to Bootstrap() for wiring repositories, usecases, and controllers.
type BootstrapConfig struct {
	Config                   *Config
	App                      *fiber.App
	Log                      *zap.Logger
	DB                       *entity.Database
	Jwt                      *pkg_jwt.JWTConfig
	Redis                    *redis.Client
	RabbitMQ                 *rabbitmq.Connection
	Validate                 *validator.Validate
	TelegramFactory          telegram.BotFactory
	Publisher                *gatewayMsg.RabbitMQPublisher
	Consumer                 *deliveryMsg.MessageConsumer
	OtpService               *otp.EmailOTPService
	Mailer                   mailer.Sender
	SMTPMailer               mailer.Sender
	Midtrans                 *midtrans.Client
	S3                       *pkg_s3.Client
	OutboxWorker             *deliveryMsg.OutboxWorker
	OrderCleanupWorker       *deliveryMsg.OrderCleanupWorker
	EnforcerWorker           *deliveryMsg.EnforcerWorker
	GroupSyncWorker          *deliveryMsg.GroupSyncWorker
	ExpiryReminderWorker     *deliveryMsg.ExpiryReminderWorker
	BroadcastSchedulerWorker *deliveryMsg.BroadcastSchedulerWorker
}

// BootstrapOption allows selective initialization of components.
type BootstrapOption func(*bootstrapOptions)

type bootstrapOptions struct {
	withPublisher bool
	withConsumer  bool
	withFiber     bool
}

// WithPublisher enables Publisher initialization (used by cmd/web).
func WithPublisher() BootstrapOption {
	return func(o *bootstrapOptions) {
		o.withPublisher = true
	}
}

// WithConsumer enables Consumer initialization (used by cmd/worker).
func WithConsumer() BootstrapOption {
	return func(o *bootstrapOptions) {
		o.withConsumer = true
	}
}

// WithFiber enables Fiber app initialization (used by cmd/web).
func WithFiber() BootstrapOption {
	return func(o *bootstrapOptions) {
		o.withFiber = true
	}
}

// NewBootstrapConfig initializes infrastructure based on the provided options.
//
// Usage:
//
//	Web:    NewBootstrapConfig(cfg, WithFiber(), WithPublisher())
//	Worker: NewBootstrapConfig(cfg, WithConsumer())
func NewBootstrapConfig(cfg *Config, opts ...BootstrapOption) (*BootstrapConfig, error) {
	// Parse options
	options := &bootstrapOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// 1. Setup Zap logger
	logger, err := NewZapLogger(&cfg.App)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: failed to create logger: %w", err)
	}

	logger.Info("bootstrap: starting application",
		zap.String("app", cfg.App.Name),
		zap.String("env", cfg.App.Env),
		zap.Int("port", cfg.App.Port),
	)

	// 2. Initialize Database
	db, err := NewDatabase(&cfg.Database, logger)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	// 3. Initialize Redis (needed by both web and worker for rate limiting, distributed lock, session)
	redisClient, err := NewRedisClient(&cfg.Redis, logger)
	if err != nil {
		CloseDatabase(db, logger)
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	// 4. Initialize RabbitMQ connection (shared by publisher and consumer)
	var rabbitConn *rabbitmq.Connection
	if options.withPublisher || options.withConsumer {
		rabbitConn, err = NewRabbitMQ(&cfg.RabbitMQ, logger)
		if err != nil {
			redisClient.Close()
			CloseDatabase(db, logger)
			return nil, fmt.Errorf("bootstrap: %w", err)
		}
	}

	// 5. Create Publisher (Gateway - Outbound Adapter) — only for web
	var publisher *gatewayMsg.RabbitMQPublisher
	if options.withPublisher && rabbitConn != nil {
		publisher, err = gatewayMsg.NewRabbitMQPublisher(rabbitConn, logger)
		if err != nil {
			rabbitConn.Close()
			redisClient.Close()
			CloseDatabase(db, logger)
			return nil, fmt.Errorf("bootstrap: %w", err)
		}
		logger.Info("bootstrap: publisher initialized")
	}

	// 6. Create Consumer (Delivery - Inbound Adapter) — only for worker
	var consumer *deliveryMsg.MessageConsumer
	if options.withConsumer && rabbitConn != nil {
		consumer = deliveryMsg.NewMessageConsumer(rabbitConn, cfg.Worker.PrefetchCount, logger)
		logger.Info("bootstrap: consumer initialized")
	}

	// 7. Initialize Fiber — only for web
	var fiberApp *fiber.App
	if options.withFiber {
		fiberApp = NewFiber(&cfg.App)
		logger.Info("bootstrap: fiber initialized")
	}

	// 8. Initialize Validator
	validate := NewValidator()

	// 9. Initialize Telegram Bot Factory
	telegramFactory := telegram.NewBotFactory(time.Duration(cfg.Telegram.APITimeout)*time.Second, logger)

	jwtConfig := NewJWTConfig(cfg)

	smtpMailer := mailer.New(mailer.Config{
		Host:      cfg.SMTP.Host,
		Port:      cfg.SMTP.Port,
		Username:  cfg.SMTP.Username,
		Password:  cfg.SMTP.Password,
		FromEmail: cfg.SMTP.FromEmail,
		FromName:  cfg.SMTP.FromName,
	})

	mailerSender := mailer.Sender(smtpMailer)
	if options.withPublisher && publisher != nil {
		mailerSender = gatewayMsg.NewQueueMailer(publisher)
	}

	otpService := otp.NewEmailOTPService(redisClient)

	// Midtrans client (used by both web and worker for billing-related operations)
	midtransClient := midtrans.NewClient(midtrans.Config{
		ServerKey: cfg.Midtrans.ServerKey,
		ClientKey: cfg.Midtrans.ClientKey,
		BaseURL:   cfg.Midtrans.BaseURL,
		SnapURL:   cfg.Midtrans.SnapURL,
	})

	// S3-compatible storage (Cloudflare R2, MinIO, AWS S3)
	var s3Client *pkg_s3.Client
	if cfg.S3.Endpoint != "" && cfg.S3.AccessKeyID != "" {
		s3Client, err = pkg_s3.NewClient(&pkg_s3.Config{
			Endpoint:        cfg.S3.Endpoint,
			AccessKeyID:     cfg.S3.AccessKeyID,
			SecretAccessKey: cfg.S3.SecretAccessKey,
			BucketName:      cfg.S3.BucketName,
			Region:          cfg.S3.Region,
			UsePathStyle:    cfg.S3.UsePathStyle,
			PublicURL:       cfg.S3.PublicURL,
		}, logger)
		if err != nil {
			logger.Warn("bootstrap: S3 client initialization skipped", zap.Error(err))
			s3Client = nil
		}
	} else {
		logger.Info("bootstrap: S3 storage not configured, skipping initialization")
	}

	logger.Info("bootstrap: all infrastructure initialized successfully")

	return &BootstrapConfig{
		Config:          cfg,
		App:             fiberApp,
		Log:             logger,
		DB:              db,
		Redis:           redisClient,
		RabbitMQ:        rabbitConn,
		Validate:        validate,
		TelegramFactory: telegramFactory,
		Publisher:       publisher,
		Consumer:        consumer,
		Jwt:             jwtConfig,
		Mailer:          mailerSender,
		SMTPMailer:      smtpMailer,
		OtpService:      otpService,
		Midtrans:        midtransClient,
		S3:              s3Client,
	}, nil
}

func BootstrapWeb(config *BootstrapConfig) {
	clientRepo := repository.NewClientRepository(config.Log)
	botRepo := repository.NewTelegramBotRepository()
	groupRepo := repository.NewTelegramGroupRepository()
	packageRepo := repository.NewPackageRepository()
	subscriptionRepo := repository.NewSubscriptionRepository()
	orderRepo := repository.NewOrderRepository()
	auditLogRepo := repository.NewAuditLogRepository()
	telegramUserRepo := repository.NewTelegramUserRepository()
	discountRepo := repository.NewMemberDiscountRepository()
	platformDiscountRepo := repository.NewPlatformDiscountRepository()
	outboxRepo := repository.NewOutboxRepository()
	customCommandRepo := repository.NewCustomCommandRepository()
	broadcastRepo := repository.NewBroadcastRepository()
	tenantAnalyticsRepo := repository.NewTenantAnalyticsRepository()
	migrationMemberRepo := repository.NewMigrationMemberRepository()

	adminUserRepo := repository.NewAdminUserRepository(config.Log)
	adminPermissionRepo := repository.NewAdminPermissionRepository(config.Log)
	adminRoleRepo := repository.NewAdminRoleRepository(config.Log)
	userRepo := repository.NewUserRepository(config.Log)
	clientUserRepo := repository.NewClientUserRepository(config.Log)
	planRepo := repository.NewPlatformPlanRepository()
	billingRepo := repository.NewClientBillingRepository()
	tenantPermissionRepo := repository.NewTenantPermissionRepository(config.Log)

	// Usecases
	adminAuthUC := usecase.NewAdminAuthUseCase(config.DB, adminUserRepo, adminPermissionRepo, config.Log, config.Redis, config.OtpService, config.Mailer, outboxRepo, config.Jwt, config.Config.App.FrontendURL, config.Config.App.BcryptCost)
	adminPermissionUC := usecase.NewAdminPermissionUseCase(config.DB, adminPermissionRepo, config.Log)
	adminRoleUC := usecase.NewAdminRoleUseCase(config.DB, adminRoleRepo, adminPermissionRepo, config.Log)
	adminUserMgmtUC := usecase.NewAdminUserManagementUseCase(config.DB, adminUserRepo, adminRoleRepo, config.Log, config.Config.App.BcryptCost)
	adminTenantUC := usecase.NewAdminTenantUseCase(config.DB, clientRepo, userRepo, clientUserRepo, config.Log, config.Config.App.BcryptCost)
	adminTenantUserUC := usecase.NewAdminTenantUserUseCase(config.DB, clientRepo, userRepo, clientUserRepo, config.Log, config.Config.App.BcryptCost)
	planUC := usecase.NewPlatformPlanUseCase(config.DB, planRepo, billingRepo, config.Log)
	platformDiscountUC := usecase.NewPlatformDiscountUseCase(config.DB, platformDiscountRepo, config.Log)
	memberDiscountUC := usecase.NewMemberDiscountUseCase(config.DB, discountRepo, config.Log)
	featureGateUC := usecase.NewFeatureGateUseCase(config.DB, billingRepo, botRepo, groupRepo, packageRepo, customCommandRepo, broadcastRepo, tenantAnalyticsRepo, config.Log)
	pdfClient := pdf.NewClient(&pdf.Config{}, config.Log)
	billingUC := usecase.NewClientBillingUseCase(config.DB, billingRepo, planRepo, clientRepo, platformDiscountRepo, platformDiscountUC, featureGateUC, config.Midtrans, config.S3, pdfClient, config.Redis, config.Log, config.Config.App.BaseURL, config.Config.App.FrontendURL)
	memberOrderUC := usecase.NewMemberOrderUseCase(config.DB, orderRepo, subscriptionRepo, packageRepo, telegramUserRepo, clientRepo, billingRepo, discountRepo, memberDiscountUC, outboxRepo, botRepo, config.Redis, config.Log, config.Config.App.EncryptionKey, config.Config.Midtrans.BaseURL, config.Config.Midtrans.SnapURL, config.Config.App.BaseURL, config.Config.App.FrontendURL, config.Config.App.MidtransPaymentLinkMode)
	tenantAuthUC := usecase.NewTenantAuthUseCase(config.DB, userRepo, clientRepo, clientUserRepo, tenantPermissionRepo, outboxRepo, config.Log, config.Redis, config.Jwt, config.Config.App.FrontendURL, config.Config.App.BcryptCost)
	tenantAnalyticsUC := usecase.NewTenantAnalyticsUseCase(config.DB.Gorm, tenantAnalyticsRepo, config.Log)
	auditLogUC := usecase.NewAuditLogUseCase(config.DB.Gorm, auditLogRepo, config.Log)
	botUC := usecase.NewTelegramBotUseCase(config.DB, botRepo, billingRepo, config.TelegramFactory, config.Log, config.Config.App.EncryptionKey, config.Config.Telegram.WebhookBaseURL, config.Config.Telegram.WebhookSecret)
	groupSyncWorker := deliveryMsg.NewGroupSyncWorker(config.DB.Gorm, groupRepo, botRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	groupUC := usecase.NewTelegramGroupUseCase(config.DB, groupRepo, botRepo, billingRepo, config.TelegramFactory, config.Redis, config.Log, config.Config.App.EncryptionKey, groupSyncWorker.Process)
	packageUC := usecase.NewPackageUseCase(config.DB, packageRepo, groupRepo, billingRepo, config.Log)
	tenantProfileUC := usecase.NewTenantProfileUseCase(config.DB, clientRepo, config.Redis, config.Config.App.EncryptionKey, config.Log)
	customCommandUC := usecase.NewCustomCommandUseCase(config.DB, customCommandRepo, botRepo, billingRepo, config.S3, config.Log)
	memberUC := usecase.NewMemberUseCase(config.DB, telegramUserRepo, subscriptionRepo, outboxRepo, auditLogRepo, config.Log)
	tenantTransactionUC := usecase.NewTenantTransactionUseCase(config.DB, orderRepo, config.S3, config.Log)
	uploadUC := usecase.NewUploadUseCase(config.S3, config.Log)
	broadcastUC := usecase.NewBroadcastUseCase(config.DB, broadcastRepo, botRepo, groupRepo, outboxRepo, billingRepo, config.Log)
	migrationMemberUC := usecase.NewMigrationMemberUseCase(config.DB, migrationMemberRepo, packageRepo, featureGateUC, config.Log)

	// Bot Handlers & Registry
	startHandler := handler.NewStartHandler(config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	packagesHandler := handler.NewPackagesHandler(config.DB, packageRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	packageSelectHandler := handler.NewPackageSelectHandler(memberOrderUC, config.TelegramFactory, config.Config.App.EncryptionKey, config.Redis, config.Log)
	mySubHandler := handler.NewMySubHandler(config.DB, subscriptionRepo, groupRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	statusHandler := handler.NewStatusAliasHandler(mySubHandler) // /status → same logic as /mysub
	myOrdersHandler := handler.NewMyOrdersHandler(config.DB, orderRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	migrationMemberCmdHandler := handler.NewMigrationMemberHandler(config.DB, migrationMemberRepo, packageRepo, subscriptionRepo, telegramUserRepo, groupRepo, config.TelegramFactory, config.Config.App.EncryptionKey, featureGateUC.CheckQuota, config.Log)

	cmdRegistry := handler.NewRegistry()
	cmdRegistry.Register(startHandler)
	cmdRegistry.Register(packagesHandler)
	cmdRegistry.Register(mySubHandler)
	cmdRegistry.Register(statusHandler)
	cmdRegistry.Register(myOrdersHandler)
	cmdRegistry.Register(migrationMemberCmdHandler)
	cmdRegistry.RegisterCallback(packageSelectHandler)

	webhookUC := usecase.NewTelegramWebhookUseCase(config.DB, config.Publisher, botRepo, groupRepo, customCommandRepo, subscriptionRepo, cmdRegistry, config.TelegramFactory, config.Redis, config.Config.App.EncryptionKey, config.Log)

	// Controllers
	adminAuthCtrl := controller.NewAdminAuthController(adminAuthUC, config.Log, config.Validate)
	adminRoleCtrl := controller.NewAdminRoleController(adminRoleUC, config.Log, config.Validate)
	adminPermissionCtrl := controller.NewAdminPermissionController(adminPermissionUC, config.Log)
	adminUserCtrl := controller.NewAdminUserController(adminRoleUC, adminUserMgmtUC, config.Log, config.Validate)
	adminClientCtrl := controller.NewAdminClientController(adminTenantUC, config.Log, config.Validate)
	adminClientUserCtrl := controller.NewAdminClientUserController(adminTenantUserUC, config.Log, config.Validate)
	planCtrl := controller.NewPlatformPlanController(planUC, config.Log, config.Validate)
	billingCtrl := controller.NewClientBillingController(billingUC, config.Log, config.Validate)
	memberOrderCtrl := controller.NewMemberOrderController(memberOrderUC, config.Log, config.Validate)
	tenantAuthCtrl := controller.NewTenantAuthController(tenantAuthUC, config.Log, config.Validate)
	botCtrl := controller.NewTelegramBotController(botUC, config.Log, config.Validate)
	groupCtrl := controller.NewTelegramGroupController(groupUC, config.Log, config.Validate)
	packageCtrl := controller.NewPackageController(packageUC, config.Log, config.Validate)
	webhookCtrl := controller.NewTelegramWebhookController(webhookUC, config.Log, config.Config.Telegram.WebhookSecret)
	memberDiscountCtrl := controller.NewMemberDiscountController(memberDiscountUC, config.Log, config.Validate)
	tenantAnalyticsCtrl := controller.NewTenantAnalyticsController(tenantAnalyticsUC, config.Log)
	auditLogCtrl := controller.NewAuditLogController(auditLogUC, config.Log)
	tenantProfileCtrl := controller.NewTenantProfileController(tenantProfileUC, config.Log, config.Validate)
	customCommandCtrl := controller.NewCustomCommandController(customCommandUC, config.Log, config.Validate)
	memberCtrl := controller.NewMemberController(memberUC, config.Validate, config.Log)
	tenantTransactionCtrl := controller.NewTenantTransactionController(tenantTransactionUC, config.Log)
	uploadCtrl := controller.NewUploadController(uploadUC, config.Log, config.Validate)
	broadcastCtrl := controller.NewBroadcastController(broadcastUC, config.Log, config.Validate)
	migrationMemberCtrl := controller.NewMigrationMemberController(migrationMemberUC, config.Log, config.Validate)

	// Rate Limiters
	globalLimiter := ratelimit.New(config.Redis, ratelimit.Config{
		Capacity:   2000,
		RefillRate: 2000,
		KeyPrefix:  "rl:global",
		Next:       ratelimit.SkipUnprotectedPaths,
	})
	ipLimiter := ratelimit.New(config.Redis, ratelimit.Config{
		Capacity:   100,
		RefillRate: 10,
		KeyPrefix:  "rl:ip:",
		KeyFunc:    func(c fiber.Ctx) string { return ratelimit.ClientIP(c) },
		Next:       ratelimit.SkipUnprotectedPaths,
	})
	adminAuthLimiter := ratelimit.New(config.Redis, ratelimit.Config{
		Capacity:   20,
		RefillRate: 2,
		KeyPrefix:  "rl:auth:admin:",
		KeyFunc:    func(c fiber.Ctx) string { return ratelimit.ClientIP(c) },
	})
	tenantAuthLimiter := ratelimit.New(config.Redis, ratelimit.Config{
		Capacity:   20,
		RefillRate: 2,
		KeyPrefix:  "rl:auth:tenant:",
		KeyFunc:    func(c fiber.Ctx) string { return ratelimit.ClientIP(c) },
	})

	config.App.Use(globalLimiter.Middleware())
	config.App.Use(ipLimiter.Middleware())

	// Routes
	adminRoute := &route.AdminRouteConfig{
		App:                       config.App,
		Log:                       config.Log,
		AdminAuthRateLimiter:      adminAuthLimiter,
		AdminAuthController:       adminAuthCtrl,
		AdminRoleController:       adminRoleCtrl,
		AdminPermissionController: adminPermissionCtrl,
		AdminUserController:       adminUserCtrl,
		AdminClientController:     adminClientCtrl,
		AdminClientUserController: adminClientUserCtrl,
		PlatformPlanController:    planCtrl,
		ClientBillingController:   billingCtrl,
		AuditLogController:        auditLogCtrl,
		AdminAuthMiddleware:       middleware.AdminAuth(config.Jwt),
	}
	adminRoute.Setup()

	tenantRoute := &route.TenantRouteConfig{
		App:                         config.App,
		Log:                         config.Log,
		TenantAuthRateLimiter:       tenantAuthLimiter,
		TenantAuthController:        tenantAuthCtrl,
		TelegramBotController:       botCtrl,
		TelegramGroupController:     groupCtrl,
		PackageController:           packageCtrl,
		MemberDiscountController:    memberDiscountCtrl,
		MemberController:            memberCtrl,
		TenantTransactionController: tenantTransactionCtrl,
		TenantAnalyticsController:   tenantAnalyticsCtrl,
		AuditLogController:          auditLogCtrl,
		ClientBillingController:     billingCtrl,
		TenantProfileController:     tenantProfileCtrl,
		CustomCommandController:     customCommandCtrl,
		UploadController:            uploadCtrl,
		BroadcastController:         broadcastCtrl,
		MigrationMemberController:   migrationMemberCtrl,
		TenantAuthMiddleware:        middleware.TenantAuth(config.Jwt),
		FeatureGateUseCase:          featureGateUC,
	}
	tenantRoute.Setup()

	publicRoute := &route.PublicRouteConfig{
		App:                       config.App,
		Log:                       config.Log,
		TelegramWebhookController: webhookCtrl,
		ClientBillingController:   billingCtrl,
		MemberOrderController:     memberOrderCtrl,
		PlatformPlanController:    planCtrl,
	}
	publicRoute.Setup()
}

// BootstrapWorker wires dependencies for the background worker.
// Flow: Repositories → UseCases → Consumer Handlers → Register to Consumer.
func BootstrapWorker(config *BootstrapConfig) {
	subscriptionRepo := repository.NewSubscriptionRepository()
	orderRepo := repository.NewOrderRepository()
	outboxRepo := repository.NewOutboxRepository()
	packageRepo := repository.NewPackageRepository()
	botRepo := repository.NewTelegramBotRepository()
	groupRepo := repository.NewTelegramGroupRepository()
	clientRepo := repository.NewClientRepository(config.Log)
	telegramUserRepo := repository.NewTelegramUserRepository()

	// PDF generator client
	pdfClient := pdf.NewClient(&pdf.Config{}, config.Log)

	notificationHandler := deliveryMsg.NewNotificationHandler(config.SMTPMailer, config.Log)
	telegramActionHandler := deliveryMsg.NewTelegramActionHandler(
		config.DB.Gorm, packageRepo, botRepo, groupRepo,
		config.TelegramFactory, config.Config.App.EncryptionKey, config.Log,
		orderRepo, clientRepo, telegramUserRepo, pdfClient, config.S3,
	)
	gatekeepingHandler := deliveryMsg.NewGatekeepingHandler(config.DB.Gorm, subscriptionRepo, botRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	enforcerHandler := deliveryMsg.NewEnforcerHandler(config.DB.Gorm, botRepo, subscriptionRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)

	config.Consumer.RegisterHandler(rabbitmq.QueueTelegramAction, telegramActionHandler.Handle)
	config.Consumer.RegisterHandler(rabbitmq.QueueTelegramActionHigh, telegramActionHandler.Handle)
	config.Consumer.RegisterHandler(rabbitmq.QueueNotification, notificationHandler.Handle)
	config.Consumer.RegisterHandler(rabbitmq.QueueGatekeeping, gatekeepingHandler.Handle)
	config.Consumer.RegisterHandler(rabbitmq.QueueEnforcer, enforcerHandler.Handle)

	// Instantiate OutboxWorker for background event polling/publishing in background worker
	config.OutboxWorker = deliveryMsg.NewOutboxWorker(config.DB, outboxRepo, config.Publisher, config.Log)

	// Instantiate OrderCleanupWorker
	discountRepo := repository.NewMemberDiscountRepository()
	memberDiscountUC := usecase.NewMemberDiscountUseCase(config.DB, discountRepo, config.Log)
	config.OrderCleanupWorker = deliveryMsg.NewOrderCleanupWorker(config.DB.Gorm, orderRepo, memberDiscountUC, config.Log)

	// Instantiate EnforcerWorker
	config.EnforcerWorker = deliveryMsg.NewEnforcerWorker(config.DB.Gorm, subscriptionRepo, groupRepo, outboxRepo, config.Log)

	// Instantiate GroupSyncWorker
	config.GroupSyncWorker = deliveryMsg.NewGroupSyncWorker(config.DB.Gorm, groupRepo, botRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)

	// Instantiate ExpiryReminderWorker
	config.ExpiryReminderWorker = deliveryMsg.NewExpiryReminderWorker(config.DB.Gorm, subscriptionRepo, outboxRepo, config.Log)

	// Register ExpiryReminderHandler as consumer
	expiryReminderHandler := deliveryMsg.NewExpiryReminderHandler(config.DB.Gorm, packageRepo, botRepo, groupRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	config.Consumer.RegisterHandler(rabbitmq.QueueExpiryReminder, expiryReminderHandler.Handle)

	// Register BroadcastHandler as consumer
	broadcastRepoWorker := repository.NewBroadcastRepository()
	broadcastHandler := deliveryMsg.NewBroadcastHandler(config.DB.Gorm, broadcastRepoWorker, botRepo, config.TelegramFactory, config.Config.App.EncryptionKey, config.Log)
	config.Consumer.RegisterHandler(rabbitmq.QueueBroadcast, broadcastHandler.Handle)

	// Instantiate BroadcastSchedulerWorker
	billingRepo := repository.NewClientBillingRepository()
	broadcastUC := usecase.NewBroadcastUseCase(config.DB, broadcastRepoWorker, botRepo, groupRepo, outboxRepo, billingRepo, config.Log)
	config.BroadcastSchedulerWorker = deliveryMsg.NewBroadcastSchedulerWorker(config.DB.Gorm, broadcastUC, config.Log)
}

// Shutdown gracefully closes all infrastructure connections.
// Shutdown order is reverse of initialization to ensure in-flight work finishes first.
func (b *BootstrapConfig) Shutdown() {
	b.Log.Info("bootstrap: shutting down...")

	// 1. Stop consumer first (stop receiving new messages)
	if b.Consumer != nil {
		b.Consumer.Stop()
	}

	// 2. Shutdown Fiber (stop receiving new HTTP requests)
	if b.App != nil {
		if err := b.App.Shutdown(); err != nil {
			b.Log.Error("bootstrap: failed to shutdown fiber", zap.Error(err))
		}
	}

	// 3. Close publisher
	if b.Publisher != nil {
		if err := b.Publisher.Close(); err != nil {
			b.Log.Error("bootstrap: failed to close publisher", zap.Error(err))
		}
	}

	// 4. Close RabbitMQ
	if b.RabbitMQ != nil {
		if err := b.RabbitMQ.Close(); err != nil {
			b.Log.Error("bootstrap: failed to close rabbitmq", zap.Error(err))
		}
	}

	// 5. Close Redis
	if b.Redis != nil {
		if err := b.Redis.Close(); err != nil {
			b.Log.Error("bootstrap: failed to close redis", zap.Error(err))
		}
	}

	// 6. Close Database (last — in-flight queries should finish first)
	CloseDatabase(b.DB, b.Log)

	b.Log.Info("bootstrap: shutdown complete")

	// Flush any buffered log entries
	_ = b.Log.Sync()
}
