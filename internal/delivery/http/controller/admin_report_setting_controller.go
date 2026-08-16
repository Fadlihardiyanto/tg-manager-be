package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type AdminReportSettingController struct {
	reportUC  usecase.IReportSettingUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewAdminReportSettingController(uc usecase.IReportSettingUseCase, log *zap.Logger, v *validator.Validate) *AdminReportSettingController {
	return &AdminReportSettingController{reportUC: uc, log: log, validator: v}
}

// Get godoc
// GET /admin/v1/report-settings
func (c *AdminReportSettingController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	clientID := middleware.GetTenantClientID(ctx)

	setting, err := c.reportUC.Get(ctx.Context(), clientID)
	if err != nil {
		log.Error("report setting get failed", zap.Error(err))
		return err
	}

	resp := &model.ReportSettingResponse{
		Enabled:      setting.Enabled,
		TargetChatID: setting.TargetChatID,
		BotID:        setting.BotID,
		ReportTime:   setting.ReportTime,
	}
	return helper.Success(ctx, "Pengaturan laporan berhasil diambil", resp)
}

// Upsert godoc
// PUT /admin/v1/report-settings
func (c *AdminReportSettingController) Upsert(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	clientID := middleware.GetTenantClientID(ctx)

	var req model.ReportSettingRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Payload tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, &req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	setting, err := c.reportUC.Upsert(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("report setting upsert failed", zap.Error(err))
		return err
	}

	resp := &model.ReportSettingResponse{
		Enabled:      setting.Enabled,
		TargetChatID: setting.TargetChatID,
		BotID:        setting.BotID,
		ReportTime:   setting.ReportTime,
	}
	return helper.Success(ctx, "Pengaturan laporan berhasil disimpan", resp)
}

// ListFailures godoc
// GET /admin/v1/report-failures?date=YYYY-MM-DD
func (c *AdminReportSettingController) ListFailures(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	clientID := middleware.GetTenantClientID(ctx)

	items, err := c.reportUC.ListFailures(ctx.Context(), clientID, ctx.Query("date"))
	if err != nil {
		log.Error("report failures list failed", zap.Error(err))
		return err
	}
	return helper.Success(ctx, "Daftar kegagalan berhasil diambil", items)
}
