package usecase

import (
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestToPlatformDiscountResponse(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	id := uuid.New()
	value := decimal.NewFromInt(10000)
	maxDiscount := decimal.NewFromInt(5000)
	minPurchase := decimal.NewFromInt(50000)
	code := "PROMO10"

	d := &entity.PlatformDiscount{
		ID:                  id,
		Name:                "Promo 10rb",
		Code:                &code,
		Type:                "nominal",
		Value:               value,
		MaxDiscount:         &maxDiscount,
		MinPurchase:         minPurchase,
		MaxUsage:            100,
		UsedCount:           5,
		ApplicablePlanIDs:   []string{"a", "b"},
		ApplicableClientIDs: []string{"c", "d"},
		ValidFrom:           now,
		ValidUntil:          ptrTime(now.Add(30 * 24 * time.Hour)),
		IsActive:            true,
		CreatedBy:           &id,
		CreatedAt:           now,
	}

	resp := toPlatformDiscountResponse(d)

	if resp.ID != id {
		t.Errorf("ID = %v, want %v", resp.ID, id)
	}
	if resp.Name != "Promo 10rb" {
		t.Errorf("Name = %q, want %q", resp.Name, "Promo 10rb")
	}
	if *resp.Code != code {
		t.Errorf("Code = %q, want %q", *resp.Code, code)
	}
	if resp.Type != "nominal" {
		t.Errorf("Type = %q, want %q", resp.Type, "nominal")
	}
	if !resp.Value.Equal(value) {
		t.Errorf("Value = %v, want %v", resp.Value, value)
	}
	if !resp.MaxDiscount.Equal(maxDiscount) {
		t.Errorf("MaxDiscount = %v, want %v", resp.MaxDiscount, maxDiscount)
	}
	if !resp.MinPurchase.Equal(minPurchase) {
		t.Errorf("MinPurchase = %v, want %v", resp.MinPurchase, minPurchase)
	}
	if resp.MaxUsage != 100 {
		t.Errorf("MaxUsage = %d, want %d", resp.MaxUsage, 100)
	}
	if resp.UsedCount != 5 {
		t.Errorf("UsedCount = %d, want %d", resp.UsedCount, 5)
	}
	if !resp.ValidFrom.Equal(now) {
		t.Errorf("ValidFrom = %v, want %v", resp.ValidFrom, now)
	}
	if resp.IsActive != true {
		t.Errorf("IsActive = %v, want %v", resp.IsActive, true)
	}
}

func TestToPlatformDiscountResponse_NilFields(t *testing.T) {
	d := &entity.PlatformDiscount{
		ID:    uuid.New(),
		Name:  "Test",
		Value: decimal.Zero,
	}

	resp := toPlatformDiscountResponse(d)

	if resp.Code != nil {
		t.Error("expected nil Code")
	}
	if resp.MaxDiscount != nil {
		t.Error("expected nil MaxDiscount")
	}
}

func TestToMemberDiscountResponse(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	id := uuid.New()
	clientID := uuid.New()
	value := decimal.NewFromInt(10000)
	code := "MEMBER10"

	d := &entity.MemberDiscount{
		ID:                   id,
		ClientID:             clientID,
		Name:                 "Member Promo",
		Code:                 &code,
		Type:                 "percentage",
		Value:                value,
		MaxDiscount:          nil,
		MinPurchase:          decimal.Zero,
		MaxUsage:             50,
		UsedCount:            3,
		MaxUsagePerUser:      1,
		ApplicablePackageIDs: []string{"x", "y"},
		ValidFrom:            now,
		ValidUntil:           ptrTime(now.Add(30 * 24 * time.Hour)),
		IsActive:             true,
		CreatedAt:            now,
	}

	resp := toMemberDiscountResponse(d)

	if resp.ID != id {
		t.Errorf("ID = %v, want %v", resp.ID, id)
	}
	if resp.Name != "Member Promo" {
		t.Errorf("Name = %q, want %q", resp.Name, "Member Promo")
	}
	if *resp.Code != code {
		t.Errorf("Code = %q, want %q", *resp.Code, code)
	}
	if resp.Type != "percentage" {
		t.Errorf("Type = %q, want %q", resp.Type, "percentage")
	}
	if resp.MaxUsagePerUser != 1 {
		t.Errorf("MaxUsagePerUser = %d, want %d", resp.MaxUsagePerUser, 1)
	}
	if resp.IsActive != true {
		t.Errorf("IsActive = %v, want %v", resp.IsActive, true)
	}
}

func TestToMemberDiscountResponse_NilCode(t *testing.T) {
	d := &entity.MemberDiscount{
		ID:    uuid.New(),
		Value: decimal.Zero,
	}

	resp := toMemberDiscountResponse(d)

	if resp.Code != nil {
		t.Error("expected nil Code")
	}
}

func TestUuidsToStrings(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	tests := []struct {
		name string
		ids  []uuid.UUID
		want int
	}{
		{"multiple uuids", []uuid.UUID{id1, id2, id3}, 3},
		{"single uuid", []uuid.UUID{id1}, 1},
		{"empty slice", []uuid.UUID{}, 0},
		{"nil slice", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uuidsToStrings(tt.ids)
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
			if tt.want > 0 {
				for i, s := range got {
					if s != tt.ids[i].String() {
						t.Errorf("got[%d] = %q, want %q", i, s, tt.ids[i].String())
					}
				}
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
