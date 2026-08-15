package controller

import (
	"errors"
	"strconv"

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
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		switch fiberErr.Code {
		case fiber.StatusUnauthorized:
			return helper.Unauthorized(ctx, fiberErr.Message)
		case fiber.StatusForbidden:
			return helper.Forbidden(ctx, fiberErr.Message)
		case fiber.StatusNotFound:
			return helper.NotFound(ctx, fiberErr.Message)
		case fiber.StatusConflict:
			return helper.Conflict(ctx, fiberErr.Message)
		}
	}

	var errBadRequest *helper.ErrBadRequest
	if errors.As(err, &errBadRequest) {
		return helper.BadRequest(ctx, errBadRequest.Error())
	}
	var errNotFound *helper.ErrNotFound
	if errors.As(err, &errNotFound) {
		return helper.NotFound(ctx, errNotFound.Error())
	}
	var errConflict *helper.ErrConflict
	if errors.As(err, &errConflict) {
		return helper.Conflict(ctx, errConflict.Error())
	}
	var errForbidden *helper.ErrForbidden
	if errors.As(err, &errForbidden) {
		return helper.Forbidden(ctx, errForbidden.Error())
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

// BulkDelete godoc
// DELETE /api/v1/tenant/discounts/bulk
func (c *MemberDiscountController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	clientID := middleware.GetTenantClientID(ctx)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("member discount controller bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validate, req); errs != nil {
		log.Warn("member discount controller bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.discountUC.BulkDelete(ctx.Context(), clientID, req.IDs)
	log.Info("member discount controller bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete diskon selesai", result)
}
