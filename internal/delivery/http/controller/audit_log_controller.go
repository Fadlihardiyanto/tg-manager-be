package controller

import (
	"strconv"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type AuditLogController struct {
	auditUC usecase.IAuditLogUseCase
	log     *zap.Logger
}

func NewAuditLogController(auditUC usecase.IAuditLogUseCase, log *zap.Logger) *AuditLogController {
	return &AuditLogController{
		auditUC: auditUC,
		log:     log,
	}
}

func (c *AuditLogController) ListTenantLogs(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("get tenant audit logs request received")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	logs, total, err := c.auditUC.GetLogsByClient(ctx.Context(), clientID, page, limit)
	if err != nil {
		c.log.Error("failed to get tenant audit logs", zap.Error(err))
		return helper.InternalError(ctx, "Gagal mengambil log aktivitas")
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil log aktivitas", logs, meta)
}

func (c *AuditLogController) ListPlatformLogs(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("get platform audit logs request received")

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	logs, total, err := c.auditUC.GetPlatformLogs(ctx.Context(), page, limit)
	if err != nil {
		c.log.Error("failed to get platform audit logs", zap.Error(err))
		return helper.InternalError(ctx, "Gagal mengambil log aktivitas platform")
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil log aktivitas platform", logs, meta)
}
