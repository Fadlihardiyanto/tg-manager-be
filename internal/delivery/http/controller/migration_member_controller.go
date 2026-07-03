package controller

import (
	"fmt"
	"strconv"
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

type MigrationMemberController struct {
	migrationUC usecase.IMigrationMemberUseCase
	log         *zap.Logger
	validator   *validator.Validate
}

func NewMigrationMemberController(uc usecase.IMigrationMemberUseCase, log *zap.Logger, v *validator.Validate) *MigrationMemberController {
	return &MigrationMemberController{
		migrationUC: uc,
		log:         log,
		validator:   v,
	}
}

// DownloadTemplate returns a CSV template file.
// GET /api/v1/tenant/migration-members/template
func (c *MigrationMemberController) DownloadTemplate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("migration member download template")

	csvContent, err := c.migrationUC.GenerateTemplate(ctx.Context())
	if err != nil {
		log.Error("migration member download template failed", zap.Error(err))
		return helper.InternalError(ctx, "Gagal membuat template")
	}

	ctx.Set("Content-Type", "text/csv")
	ctx.Set("Content-Disposition", "attachment; filename=migration_template.csv")
	return ctx.SendString(csvContent)
}

// Import bulk inserts migration members from CSV data.
// POST /api/v1/tenant/migration-members/import
func (c *MigrationMemberController) Import(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("migration member import")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.MigrationMemberImportRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("migration member import bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("migration member import validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.migrationUC.ImportMembers(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("migration member import failed", zap.Error(err))
		return err
	}

	return helper.Created(ctx, fmt.Sprintf("Berhasil mengimpor %d member", result.Imported), result)
}

// List returns paginated list of migration members.
// GET /api/v1/tenant/migration-members
func (c *MigrationMemberController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("migration member list")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := model.MigrationMemberFilterRequest{
		Page:      page,
		Limit:     limit,
		Status:    ctx.Query("status", ""),
		Search:    ctx.Query("search", ""),
		PackageID: parseUUID(ctx.Query("package_id", "")),
	}

	result, total, err := c.migrationUC.List(ctx.Context(), clientID, filter)
	if err != nil {
		log.Error("migration member list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar migrasi member", result, meta)
}

// ExportCSV downloads the full migration status report as CSV.
// GET /api/v1/tenant/migration-members/export
func (c *MigrationMemberController) ExportCSV(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("migration member export csv")

	clientID := middleware.GetTenantClientID(ctx)

	csvContent, err := c.migrationUC.ExportCSV(ctx.Context(), clientID)
	if err != nil {
		log.Error("migration member export csv failed", zap.Error(err))
		return err
	}

	filename := fmt.Sprintf("migration_report_%s.csv", time.Now().Format("20060102_150405"))
	ctx.Set("Content-Type", "text/csv")
	ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	return ctx.SendString(csvContent)
}

func parseUUID(s string) uuid.UUID {
	if s == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}
