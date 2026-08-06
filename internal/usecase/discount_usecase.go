package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	pkgDiscount "github.com/Fadlihardiyanto/telegram-management-app/pkg/discount"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Platform Discount Usecase ─────────────────────────────────

type IPlatformDiscountUseCase interface {
	List(ctx context.Context, onlyActive bool, callerPermissions []string) ([]model.PlatformDiscountResponse, error)
	GetByID(ctx context.Context, id uuid.UUID, callerPermissions []string) (*model.PlatformDiscountResponse, error)
	Create(ctx context.Context, req *model.CreatePlatformDiscountRequest) (*model.PlatformDiscountResponse, error)
	Update(ctx context.Context, req *model.UpdatePlatformDiscountRequest) (*model.PlatformDiscountResponse, error)
	Delete(ctx context.Context, id uuid.UUID, callerPermissions []string) error
	BulkDelete(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult

	ApplyByCode(ctx context.Context, req *model.ApplyPlatformDiscountRequest) (*model.DiscountPreviewResponse, error)
	ApplyAuto(ctx context.Context, clientID, planID uuid.UUID, amount decimal.Decimal) (*model.DiscountPreviewResponse, error)
}

type platformDiscountUseCase struct {
	db           *entity.Database
	discountRepo repository.IPlatformDiscountRepository
	log          *zap.Logger
}

func NewPlatformDiscountUseCase(
	db *entity.Database,
	discountRepo repository.IPlatformDiscountRepository,
	log *zap.Logger,
) IPlatformDiscountUseCase {
	return &platformDiscountUseCase{db: db, discountRepo: discountRepo, log: log}
}

func (uc *platformDiscountUseCase) List(ctx context.Context, onlyActive bool, callerPermissions []string) ([]model.PlatformDiscountResponse, error) {
	if !rbac.HasPermission(callerPermissions, "billing.read") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.read' permission")
	}
	discounts, err := uc.discountRepo.FindAll(ctx, uc.db.Gorm, onlyActive)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil data diskon")
	}
	result := make([]model.PlatformDiscountResponse, len(discounts))
	for i, d := range discounts {
		result[i] = toPlatformDiscountResponse(&d)
	}
	return result, nil
}

func (uc *platformDiscountUseCase) GetByID(ctx context.Context, id uuid.UUID, callerPermissions []string) (*model.PlatformDiscountResponse, error) {
	if !rbac.HasPermission(callerPermissions, "billing.read") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.read' permission")
	}
	d, err := uc.discountRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil || d == nil {
		return nil, helper.NewNotFound("diskon tidak ditemukan")
	}
	resp := toPlatformDiscountResponse(d)
	return &resp, nil
}

func (uc *platformDiscountUseCase) Create(ctx context.Context, req *model.CreatePlatformDiscountRequest) (*model.PlatformDiscountResponse, error) {
	if !rbac.HasPermission(req.CallerPermissions, "billing.manage") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	// Cek duplikasi kode jika ada
	if req.Code != nil && *req.Code != "" {
		existing, _ := uc.discountRepo.FindByCode(ctx, uc.db.Gorm, *req.Code)
		if existing != nil {
			return nil, helper.NewConflict(fmt.Sprintf("kode diskon '%s' sudah digunakan", *req.Code))
		}
	}

	// Convert applicable IDs ke string array untuk pq
	planIDs := uuidsToStrings(req.ApplicablePlanIDs)
	clientIDs := uuidsToStrings(req.ApplicableClientIDs)

	now := time.Now()
	d := &entity.PlatformDiscount{
		ID:                  uuid.New(),
		Name:                req.Name,
		Code:                req.Code,
		Type:                req.Type,
		Value:               req.Value,
		MaxDiscount:         req.MaxDiscount,
		MinPurchase:         req.MinPurchase,
		MaxUsage:            req.MaxUsage,
		ApplicablePlanIDs:   planIDs,
		ApplicableClientIDs: clientIDs,
		ValidFrom:           req.ValidFrom,
		ValidUntil:          req.ValidUntil,
		IsActive:            true,
		CreatedBy:           &req.AdminID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := uc.discountRepo.Create(ctx, uc.db.Gorm, d); err != nil {
		uc.log.Error("platform discount: create", zap.Error(err))
		return nil, fmt.Errorf("gagal membuat diskon")
	}

	resp := toPlatformDiscountResponse(d)
	return &resp, nil
}

func (uc *platformDiscountUseCase) Update(ctx context.Context, req *model.UpdatePlatformDiscountRequest) (*model.PlatformDiscountResponse, error) {
	if !rbac.HasPermission(req.CallerPermissions, "billing.manage") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	d, err := uc.discountRepo.FindByID(ctx, uc.db.Gorm, req.DiscountID)
	if err != nil || d == nil {
		return nil, helper.NewNotFound("diskon tidak ditemukan")
	}

	if req.Name != "" {
		d.Name = req.Name
	}
	if req.Value != nil {
		d.Value = *req.Value
	}
	if req.MaxDiscount != nil {
		d.MaxDiscount = req.MaxDiscount
	}
	if req.MinPurchase != nil {
		d.MinPurchase = *req.MinPurchase
	}
	if req.MaxUsage != nil {
		d.MaxUsage = *req.MaxUsage
	}
	if req.ValidUntil != nil {
		d.ValidUntil = req.ValidUntil
	}
	if req.IsActive != nil {
		d.IsActive = *req.IsActive
	}
	d.UpdatedAt = time.Now()

	if err := uc.discountRepo.Update(ctx, uc.db.Gorm, d); err != nil {
		return nil, fmt.Errorf("gagal update diskon")
	}

	resp := toPlatformDiscountResponse(d)
	return &resp, nil
}

func (uc *platformDiscountUseCase) Delete(ctx context.Context, id uuid.UUID, callerPermissions []string) error {
	if !rbac.HasPermission(callerPermissions, "billing.manage") {
		return helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}
	d, err := uc.discountRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil || d == nil {
		return helper.NewNotFound("diskon tidak ditemukan")
	}
	if d.UsedCount > 0 {
		// Nonaktifkan saja, jangan hapus — jaga audit trail
		d.IsActive = false
		d.UpdatedAt = time.Now()
		return uc.discountRepo.Update(ctx, uc.db.Gorm, d)
	}
	return uc.discountRepo.SoftDelete(ctx, uc.db.Gorm, id)
}

func (uc *platformDiscountUseCase) BulkDelete(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.Delete(ctx, id, callerPermissions)
	})
}

func (uc *platformDiscountUseCase) ApplyByCode(ctx context.Context, req *model.ApplyPlatformDiscountRequest) (*model.DiscountPreviewResponse, error) {
	d, err := uc.discountRepo.FindByCode(ctx, uc.db.Gorm, req.Code)
	if err != nil || d == nil {
		return nil, helper.NewBadRequest("kode diskon tidak valid atau sudah kadaluarsa")
	}
	return uc.applyDiscount(d, req.PlanID, req.Amount)
}

func (uc *platformDiscountUseCase) ApplyAuto(ctx context.Context, clientID, planID uuid.UUID, amount decimal.Decimal) (*model.DiscountPreviewResponse, error) {
	d, err := uc.discountRepo.FindAutoApplicable(ctx, uc.db.Gorm, clientID, planID, amount)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil // Tidak ada diskon otomatis — bukan error
	}
	return uc.applyDiscount(d, planID, amount)
}

func (uc *platformDiscountUseCase) applyDiscount(d *entity.PlatformDiscount, planID uuid.UUID, amount decimal.Decimal) (*model.DiscountPreviewResponse, error) {
	pkgD := &pkgDiscount.Discount{
		Type:        pkgDiscount.Type(d.Type),
		Value:       d.Value,
		MaxDiscount: d.MaxDiscount,
		MinPurchase: d.MinPurchase,
		MaxUsage:    d.MaxUsage,
		UsedCount:   d.UsedCount,
		ValidFrom:   d.ValidFrom,
		ValidUntil:  d.ValidUntil,
		IsActive:    d.IsActive,
	}

	result, err := pkgDiscount.Calculate(pkgD, amount)
	if err != nil {
		return nil, err
	}

	return &model.DiscountPreviewResponse{
		DiscountID:     d.ID,
		DiscountName:   d.Name,
		OriginalAmount: result.OriginalAmount,
		DiscountAmount: result.DiscountAmount,
		FinalAmount:    result.FinalAmount,
	}, nil
}

// ── Member Discount Usecase ───────────────────────────────────

type IMemberDiscountUseCase interface {
	ListByClient(ctx context.Context, clientID uuid.UUID, onlyActive bool, page, limit int) ([]model.MemberDiscountResponse, int64, error)
	Create(ctx context.Context, req *model.CreateMemberDiscountRequest) (*model.MemberDiscountResponse, error)
	Update(ctx context.Context, req *model.UpdateMemberDiscountRequest) (*model.MemberDiscountResponse, error)
	Delete(ctx context.Context, id, clientID uuid.UUID) error
	BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult

	// Dipakai saat member checkout paket
	ApplyByCode(ctx context.Context, req *model.ApplyMemberDiscountRequest) (*model.DiscountPreviewResponse, error)
	ApplyAuto(ctx context.Context, req *model.ApplyMemberDiscountRequest) (*model.DiscountPreviewResponse, error)

	// Dipanggil saat order sukses untuk commit usage
	CommitUsage(ctx context.Context, tx *gorm.DB, discountID, telegramUserID, orderID uuid.UUID, discountAmount decimal.Decimal) error
	RollbackUsage(ctx context.Context, tx *gorm.DB, orderID, discountID uuid.UUID) error
}

type memberDiscountUseCase struct {
	db           *entity.Database
	discountRepo repository.IMemberDiscountRepository
	log          *zap.Logger
}

func NewMemberDiscountUseCase(
	db *entity.Database,
	discountRepo repository.IMemberDiscountRepository,
	log *zap.Logger,
) IMemberDiscountUseCase {
	return &memberDiscountUseCase{db: db, discountRepo: discountRepo, log: log}
}

func (uc *memberDiscountUseCase) ListByClient(ctx context.Context, clientID uuid.UUID, onlyActive bool, page, limit int) ([]model.MemberDiscountResponse, int64, error) {
	discounts, err := uc.discountRepo.FindAllByClient(ctx, uc.db.Gorm, clientID, onlyActive, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal mengambil data diskon")
	}
	total, err := uc.discountRepo.CountAllByClient(ctx, uc.db.Gorm, clientID, onlyActive)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung jumlah diskon")
	}
	result := make([]model.MemberDiscountResponse, len(discounts))
	for i, d := range discounts {
		result[i] = toMemberDiscountResponse(&d)
	}
	return result, total, nil
}

func (uc *memberDiscountUseCase) Create(ctx context.Context, req *model.CreateMemberDiscountRequest) (*model.MemberDiscountResponse, error) {
	// Cek duplikasi kode dalam client yang sama
	if req.Code != nil && *req.Code != "" {
		existing, _ := uc.discountRepo.FindByCode(ctx, uc.db.Gorm, req.ClientID, *req.Code)
		if existing != nil {
			return nil, helper.NewConflict(fmt.Sprintf("kode diskon '%s' sudah digunakan", *req.Code))
		}
	}

	pkgIDs := uuidsToStrings(req.ApplicablePackageIDs)

	now := time.Now()
	d := &entity.MemberDiscount{
		ID:                   uuid.New(),
		ClientID:             req.ClientID,
		Name:                 req.Name,
		Code:                 req.Code,
		Type:                 req.Type,
		Value:                req.Value,
		MaxDiscount:          req.MaxDiscount,
		MinPurchase:          req.MinPurchase,
		MaxUsage:             req.MaxUsage,
		MaxUsagePerUser:      req.MaxUsagePerUser,
		ApplicablePackageIDs: pkgIDs,
		ValidFrom:            req.ValidFrom,
		ValidUntil:           req.ValidUntil,
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := uc.discountRepo.Create(ctx, uc.db.Gorm, d); err != nil {
		uc.log.Error("member discount: create", zap.Error(err))
		return nil, fmt.Errorf("gagal membuat diskon")
	}

	resp := toMemberDiscountResponse(d)
	return &resp, nil
}

func (uc *memberDiscountUseCase) Update(ctx context.Context, req *model.UpdateMemberDiscountRequest) (*model.MemberDiscountResponse, error) {
	d, err := uc.discountRepo.FindByID(ctx, uc.db.Gorm, req.DiscountID)
	if err != nil || d == nil {
		return nil, helper.NewNotFound("diskon tidak ditemukan")
	}

	// Pastikan discount milik client ini
	if d.ClientID != req.ClientID {
		return nil, helper.NewForbidden("forbidden: diskon tidak ditemukan")
	}

	if req.Name != "" {
		d.Name = req.Name
	}
	if req.Value != nil {
		d.Value = *req.Value
	}
	if req.MaxDiscount != nil {
		d.MaxDiscount = req.MaxDiscount
	}
	if req.MinPurchase != nil {
		d.MinPurchase = *req.MinPurchase
	}
	if req.MaxUsage != nil {
		d.MaxUsage = *req.MaxUsage
	}
	if req.MaxUsagePerUser != nil {
		d.MaxUsagePerUser = *req.MaxUsagePerUser
	}
	if req.ValidUntil != nil {
		d.ValidUntil = req.ValidUntil
	}
	if req.IsActive != nil {
		d.IsActive = *req.IsActive
	}
	d.UpdatedAt = time.Now()

	if err := uc.discountRepo.Update(ctx, uc.db.Gorm, d); err != nil {
		return nil, fmt.Errorf("gagal update diskon")
	}

	resp := toMemberDiscountResponse(d)
	return &resp, nil
}

func (uc *memberDiscountUseCase) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	d, err := uc.discountRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil || d == nil {
		return helper.NewNotFound("diskon tidak ditemukan")
	}
	if d.ClientID != clientID {
		return helper.NewForbidden("forbidden: diskon tidak ditemukan")
	}
	if d.UsedCount > 0 {
		d.IsActive = false
		d.UpdatedAt = time.Now()
		return uc.discountRepo.Update(ctx, uc.db.Gorm, d)
	}
	return uc.discountRepo.SoftDelete(ctx, uc.db.Gorm, id)
}

func (uc *memberDiscountUseCase) BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.Delete(ctx, id, clientID)
	})
}

func (uc *memberDiscountUseCase) ApplyByCode(ctx context.Context, req *model.ApplyMemberDiscountRequest) (*model.DiscountPreviewResponse, error) {
	if req.Code == nil || *req.Code == "" {
		return nil, helper.NewBadRequest("kode promo wajib diisi")
	}

	d, err := uc.discountRepo.FindByCode(ctx, uc.db.Gorm, req.ClientID, *req.Code)
	if err != nil || d == nil {
		return nil, helper.NewBadRequest("kode promo tidak valid atau sudah kadaluarsa")
	}

	return uc.applyMemberDiscount(ctx, d, req)
}

func (uc *memberDiscountUseCase) ApplyAuto(ctx context.Context, req *model.ApplyMemberDiscountRequest) (*model.DiscountPreviewResponse, error) {
	d, err := uc.discountRepo.FindAutoApplicable(ctx, uc.db.Gorm, req.ClientID, req.PackageID, req.Amount)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil // Tidak ada diskon otomatis
	}
	return uc.applyMemberDiscount(ctx, d, req)
}

func (uc *memberDiscountUseCase) CommitUsage(ctx context.Context, tx *gorm.DB, discountID, telegramUserID, orderID uuid.UUID, discountAmount decimal.Decimal) error {
	if tx == nil {
		tx = uc.db.Gorm
	}

	usage := &entity.MemberDiscountUsage{
		ID:             uuid.New(),
		DiscountID:     discountID,
		TelegramUserID: telegramUserID,
		OrderID:        orderID,
		DiscountAmount: discountAmount,
		UsedAt:         time.Now(),
	}
	if err := uc.discountRepo.CreateUsage(ctx, tx, usage); err != nil {
		return fmt.Errorf("gagal menyimpan usage diskon")
	}
	return uc.discountRepo.IncrementUsage(ctx, tx, discountID)
}

func (uc *memberDiscountUseCase) RollbackUsage(ctx context.Context, tx *gorm.DB, orderID, discountID uuid.UUID) error {
	rows, err := uc.discountRepo.DeleteUsageByOrderID(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("gagal menghapus usage diskon: %w", err)
	}
	if rows == 0 {
		return nil
	}
	return uc.discountRepo.DecrementUsage(ctx, tx, discountID)
}

// ── Private helpers ───────────────────────────────────────────

func (uc *memberDiscountUseCase) applyMemberDiscount(ctx context.Context, d *entity.MemberDiscount, req *model.ApplyMemberDiscountRequest) (*model.DiscountPreviewResponse, error) {
	// Cek usage per user
	if d.MaxUsagePerUser > 0 {
		usageCount, err := uc.discountRepo.CountUsageByUser(ctx, uc.db.Gorm, d.ID, req.TelegramUserID)
		if err != nil {
			return nil, fmt.Errorf("gagal cek penggunaan diskon")
		}
		if int(usageCount) >= d.MaxUsagePerUser {
			return nil, helper.NewBadRequest(fmt.Sprintf("Anda sudah menggunakan diskon ini sebanyak %d kali", d.MaxUsagePerUser))
		}
	}

	pkgD := &pkgDiscount.Discount{
		Type:        pkgDiscount.Type(d.Type),
		Value:       d.Value,
		MaxDiscount: d.MaxDiscount,
		MinPurchase: d.MinPurchase,
		MaxUsage:    d.MaxUsage,
		UsedCount:   d.UsedCount,
		ValidFrom:   d.ValidFrom,
		ValidUntil:  d.ValidUntil,
		IsActive:    d.IsActive,
	}

	result, err := pkgDiscount.Calculate(pkgD, req.Amount)
	if err != nil {
		return nil, err
	}

	return &model.DiscountPreviewResponse{
		DiscountID:     d.ID,
		DiscountName:   d.Name,
		OriginalAmount: result.OriginalAmount,
		DiscountAmount: result.DiscountAmount,
		FinalAmount:    result.FinalAmount,
	}, nil
}

func toPlatformDiscountResponse(d *entity.PlatformDiscount) model.PlatformDiscountResponse {
	return model.PlatformDiscountResponse{
		ID:          d.ID,
		Name:        d.Name,
		Code:        d.Code,
		Type:        d.Type,
		Value:       d.Value,
		MaxDiscount: d.MaxDiscount,
		MinPurchase: d.MinPurchase,
		MaxUsage:    d.MaxUsage,
		UsedCount:   d.UsedCount,
		ValidFrom:   d.ValidFrom,
		ValidUntil:  d.ValidUntil,
		IsActive:    d.IsActive,
		CreatedAt:   d.CreatedAt,
	}
}

func toMemberDiscountResponse(d *entity.MemberDiscount) model.MemberDiscountResponse {
	return model.MemberDiscountResponse{
		ID:              d.ID,
		Name:            d.Name,
		Code:            d.Code,
		Type:            d.Type,
		Value:           d.Value,
		MaxDiscount:     d.MaxDiscount,
		MinPurchase:     d.MinPurchase,
		MaxUsage:        d.MaxUsage,
		UsedCount:       d.UsedCount,
		MaxUsagePerUser: d.MaxUsagePerUser,
		ValidFrom:       d.ValidFrom,
		ValidUntil:      d.ValidUntil,
		IsActive:        d.IsActive,
		CreatedAt:       d.CreatedAt,
	}
}

func uuidsToStrings(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}
