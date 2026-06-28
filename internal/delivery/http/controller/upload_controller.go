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

type UploadController struct {
	uploadUC  usecase.IUploadUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewUploadController(uc usecase.IUploadUseCase, log *zap.Logger, v *validator.Validate) *UploadController {
	return &UploadController{
		uploadUC:  uc,
		log:       log,
		validator: v,
	}
}

// GetPresignedURL godoc
// POST /api/v1/tenant/upload/presign
func (c *UploadController) GetPresignedURL(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("upload controller get presigned url request")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.UploadPresignRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("upload controller bind request failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("upload controller validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.uploadUC.GeneratePresignedURL(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("upload controller generate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Presigned URL berhasil dibuat", result)
}
