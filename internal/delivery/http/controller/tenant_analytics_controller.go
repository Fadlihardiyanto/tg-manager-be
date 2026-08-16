package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
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

	clientID := middleware.GetTenantClientID(ctx)

	overview, err := c.analyticsUC.GetOverview(ctx.Context(), clientID)
	if err != nil {
		// Central handler di config/fiber.go memetakan helper.Err* ke status
		// yang benar — jangan flatten semua jadi 500.
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil analitik", overview)
}
