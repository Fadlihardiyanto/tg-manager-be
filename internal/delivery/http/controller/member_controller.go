package controller

import (
	"strconv"
	"strings"
	"time"

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

type MemberController struct {
	memberUC  usecase.IMemberUseCase
	validator *validator.Validate
	log       *zap.Logger
}

func NewMemberController(uc usecase.IMemberUseCase, validate *validator.Validate, log *zap.Logger) *MemberController {
	return &MemberController{
		memberUC:  uc,
		validator: validate,
		log:       log,
	}
}

// List godoc
// GET /api/v1/tenant/members
func (c *MemberController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	// Parse query params
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	status := ctx.Query("status", "all")
	search := ctx.Query("search", "")

	var packageID uuid.UUID
	if pid := ctx.Query("package_id", ""); pid != "" {
		parsed, err := uuid.Parse(pid)
		if err != nil {
			log.Warn("member controller list invalid package_id", zap.String("package_id", pid))
			return helper.BadRequest(ctx, "package_id tidak valid")
		}
		packageID = parsed
	}

	filter := model.MemberFilterRequest{
		Page:      page,
		Limit:     limit,
		Status:    status,
		Search:    search,
		PackageID: packageID,
	}

	joinedStart, joinedEnd := parseTimeRangeMs(ctx.Query("joined", ""))
	filter.JoinedStart = joinedStart
	filter.JoinedEnd = joinedEnd

	expiredStart, expiredEnd := parseTimeRangeMs(ctx.Query("expired", ""))
	filter.ExpiredStart = expiredStart
	filter.ExpiredEnd = expiredEnd

	nearestExpiryStart, nearestExpiryEnd := parseTimeRangeMs(ctx.Query("nearest_expiry", ""))
	filter.NearestExpiryStart = nearestExpiryStart
	filter.NearestExpiryEnd = nearestExpiryEnd

	result, total, err := c.memberUC.FindAll(ctx.Context(), clientID, filter)
	if err != nil {
		log.Error("member controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar member", result, meta)
}

func parseTimeRangeMs(val string) (*time.Time, *time.Time) {
	if val == "" {
		return nil, nil
	}
	parts := strings.Split(val, ",")
	if len(parts) != 2 {
		return nil, nil
	}

	startMs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, nil
	}
	endMs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, nil
	}

	start := time.UnixMilli(startMs)
	end := time.UnixMilli(endMs)
	return &start, &end
}

// Get godoc
// GET /api/v1/tenant/members/:id
func (c *MemberController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller get request")

	clientID := middleware.GetTenantClientID(ctx)

	userID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("member controller get invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID member tidak valid")
	}

	result, err := c.memberUC.FindByID(ctx.Context(), clientID, userID)
	if err != nil {
		log.Error("member controller get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil detail member", result)
}

// Kick godoc
// POST /api/v1/tenant/members/:id/kick
func (c *MemberController) Kick(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller kick request")

	clientID := middleware.GetTenantClientID(ctx)

	userID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("member controller kick invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID member tidak valid")
	}

	var subscriptionID *uuid.UUID
	if sid := ctx.Query("subscription_id", ""); sid != "" {
		parsed, err := uuid.Parse(sid)
		if err != nil {
			log.Warn("member controller kick invalid subscription_id", zap.String("subscription_id", sid))
			return helper.BadRequest(ctx, "subscription_id tidak valid")
		}
		subscriptionID = &parsed
	}

	if err := c.memberUC.KickMember(ctx.Context(), clientID, userID, subscriptionID); err != nil {
		log.Error("member controller kick failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil memberhentikan akses member", nil)
}

// Extend godoc
// POST /api/v1/tenant/members/:id/extend
func (c *MemberController) Extend(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller extend request")

	clientID := middleware.GetTenantClientID(ctx)

	userID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("member controller extend invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID member tidak valid")
	}

	var req model.ExtendMemberRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("member controller extend invalid body", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	if err := c.memberUC.ExtendMember(ctx.Context(), clientID, userID, &req); err != nil {
		log.Error("member controller extend failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil memperpanjang akses member", nil)
}

// Sync godoc
// POST /api/v1/tenant/members/:id/sync
func (c *MemberController) Sync(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller sync request")

	clientID := middleware.GetTenantClientID(ctx)

	userID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("member controller sync invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID member tidak valid")
	}

	if err := c.memberUC.SyncMember(ctx.Context(), clientID, userID); err != nil {
		log.Error("member controller sync failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengirim permintaan sinkronisasi akses member", nil)
}

// ResendLink godoc
// POST /api/v1/tenant/members/:id/resend-link
func (c *MemberController) ResendLink(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("member controller resend link request")

	clientID := middleware.GetTenantClientID(ctx)

	userID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("member controller resend link invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID member tidak valid")
	}

	if err := c.memberUC.ResendLink(ctx.Context(), clientID, userID); err != nil {
		log.Error("member controller resend link failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengirim ulang link undangan member", nil)
}
