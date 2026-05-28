package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TenantAnalyticsController struct {
	analyticsUC usecase.ITenantAnalyticsUseCase
	log         *zap.Logger
}

func NewTenantAnalyticsController(analyticsUC usecase.ITenantAnalyticsUseCase, log *zap.Logger) *TenantAnalyticsController {
	return &TenantAnalyticsController{
		analyticsUC: analyticsUC,
		log:         log,
	}
}

func (c *TenantAnalyticsController) GetOverview(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("get tenant analytics overview request received")

	clientIDStr := ctx.Locals("ClientID").(string)
	clientID, _ := uuid.Parse(clientIDStr)

	overview, err := c.analyticsUC.GetOverview(ctx.Context(), clientID)
	if err != nil {
		c.log.Error("failed to get analytics overview", zap.Error(err))
		return helper.InternalError(ctx, "Gagal mengambil data analitik")
	}

	return helper.Success(ctx, "Berhasil mengambil analitik", overview)
}
