package controller

import (
	"strconv"
	"strings"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MemberDiscountController struct {
	discountUC usecase.IMemberDiscountUseCase
	log        *zap.Logger
	validate   *validator.Validate
}

func NewMemberDiscountController(discountUC usecase.IMemberDiscountUseCase, log *zap.Logger, validate *validator.Validate) *MemberDiscountController {
	return &MemberDiscountController{
		discountUC: discountUC,
		log:        log,
		validate:   validate,
	}
}

func (c *MemberDiscountController) handleError(ctx fiber.Ctx, err error) error {
	if e, ok := err.(*fiber.Error); ok {
		switch e.Code {
		case fiber.StatusUnauthorized:
			return helper.Unauthorized(ctx, e.Message)
		case fiber.StatusForbidden:
			return helper.Forbidden(ctx, e.Message)
		case fiber.StatusNotFound:
			return helper.NotFound(ctx, e.Message)
		case fiber.StatusConflict:
			return helper.Conflict(ctx, e.Message)
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "tidak ditemukan"):
		return helper.NotFound(ctx, msg)
	case strings.Contains(msg, "sudah digunakan"):
		return helper.Conflict(ctx, msg)
	case strings.Contains(msg, "forbidden"):
		return helper.Forbidden(ctx, msg)
	}

	c.log.Error("unhandled error in member discount controller", zap.Error(err))
	return helper.InternalError(ctx, "Terjadi kesalahan pada server")
}

func (c *MemberDiscountController) List(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("list member discounts request received")

	clientID := middleware.GetTenantClientID(ctx)

	onlyActive := ctx.Query("active") == "true"

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	discounts, total, err := c.discountUC.ListByClient(ctx.Context(), clientID, onlyActive, page, limit)
	if err != nil {
		return c.handleError(ctx, err)
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar diskon", discounts, meta)
}

func (c *MemberDiscountController) Create(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("create member discount request received")

	var req model.CreateMemberDiscountRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid: "+err.Error())
	}

	clientID := middleware.GetTenantClientID(ctx)
	req.ClientID = clientID

	if errors := helper.ValidateStruct(c.validate, &req); errors != nil {
		return helper.UnprocessableEntity(ctx, errors)
	}

	res, err := c.discountUC.Create(ctx.Context(), &req)
	if err != nil {
		return c.handleError(ctx, err)
	}

	return helper.Created(ctx, "Berhasil membuat diskon", res)
}

func (c *MemberDiscountController) Update(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("update member discount request received")

	idStr := ctx.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return helper.BadRequest(ctx, "ID diskon tidak valid")
	}

	var req model.UpdateMemberDiscountRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid: "+err.Error())
	}

	req.DiscountID = id
	clientID := middleware.GetTenantClientID(ctx)
	req.ClientID = clientID

	if errors := helper.ValidateStruct(c.validate, &req); errors != nil {
		return helper.UnprocessableEntity(ctx, errors)
	}

	res, err := c.discountUC.Update(ctx.Context(), &req)
	if err != nil {
		return c.handleError(ctx, err)
	}

	return helper.Success(ctx, "Berhasil memperbarui diskon", res)
}

func (c *MemberDiscountController) Delete(ctx fiber.Ctx) error {
	logger.FromContext(ctx, c.log).Info("delete member discount request received")

	idStr := ctx.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return helper.BadRequest(ctx, "ID diskon tidak valid")
	}

	clientID := middleware.GetTenantClientID(ctx)

	if err := c.discountUC.Delete(ctx.Context(), id, clientID); err != nil {
		return c.handleError(ctx, err)
	}

	return helper.Success(ctx, "Berhasil menghapus diskon", nil)
}
