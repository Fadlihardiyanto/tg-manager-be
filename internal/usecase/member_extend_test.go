package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
)

func TestBulkExtendMembers_Success(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	base := time.Now().Add(30 * 24 * time.Hour)
	insertKickFixture(t, db, clientID, userID, subID, base, "active")

	result := uc.BulkExtendMembers(context.Background(), clientID, []model.BulkMemberExtendItem{
		{MemberID: userID, SubscriptionID: subID, AdditionalDays: 7},
	})

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", result.Failed)
	}

	var sub entity.Subscription
	if err := db.Gorm.Where("id = ?", subID).First(&sub).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	want := base.AddDate(0, 0, 7)
	if !sub.ExpiredAt.Equal(want) {
		t.Errorf("ExpiredAt = %v, want %v", sub.ExpiredAt, want)
	}

	var auditCount int64
	db.Gorm.Model(&entity.AuditLog{}).Where("action = ?", "extend_expiry").Count(&auditCount)
	if auditCount != 1 {
		t.Errorf("audit logs = %d, want 1", auditCount)
	}
}

func TestBulkExtendMembers_SkipsNotFoundSubscription(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(30*24*time.Hour), "active")

	// subscription belongs to member but a different client → should skip
	otherClient := uuid.New()
	result := uc.BulkExtendMembers(context.Background(), otherClient, []model.BulkMemberExtendItem{
		{MemberID: userID, SubscriptionID: subID, AdditionalDays: 7},
	})

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0 (skipped)", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none (silent skip)", result.Failed)
	}
}

func TestBulkExtendMembers_SkipsNotActive(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(30*24*time.Hour), "cancelled")

	result := uc.BulkExtendMembers(context.Background(), clientID, []model.BulkMemberExtendItem{
		{MemberID: userID, SubscriptionID: subID, AdditionalDays: 7},
	})

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0 (not active skipped)", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", result.Failed)
	}
}

func TestBulkExtendMembers_BestEffortMix(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	validUser := uuid.New()
	validSub := uuid.New()
	base := time.Now().Add(30 * 24 * time.Hour)
	insertKickFixture(t, db, clientID, validUser, validSub, base, "active")

	otherClient := uuid.New()
	foreignUser := uuid.New()
	foreignSub := uuid.New()
	insertKickFixture(t, db, otherClient, foreignUser, foreignSub, time.Now().Add(30*24*time.Hour), "active")

	result := uc.BulkExtendMembers(context.Background(), clientID, []model.BulkMemberExtendItem{
		{MemberID: validUser, SubscriptionID: validSub, AdditionalDays: 7},
		{MemberID: foreignUser, SubscriptionID: foreignSub, AdditionalDays: 7}, // foreign → skip
	})

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none (skip, not fail)", result.Failed)
	}

	var sub entity.Subscription
	if err := db.Gorm.Where("id = ?", validSub).First(&sub).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	want := base.AddDate(0, 0, 7)
	if !sub.ExpiredAt.Equal(want) {
		t.Errorf("ExpiredAt = %v, want %v", sub.ExpiredAt, want)
	}
}
