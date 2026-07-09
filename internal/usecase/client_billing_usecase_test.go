package usecase

import (
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCalculateBilling(t *testing.T) {
	uc := &clientBillingUseCase{}
	plan := &entity.PlatformPlan{
		PriceMonthly: decimal.NewFromInt(100000),
		PriceYearly:  decimal.NewFromInt(1000000),
	}

	t.Run("monthly cycle", func(t *testing.T) {
		before := time.Now()
		amount, expiredAt := uc.calculateBilling(plan, "monthly")
		after := time.Now()

		if !amount.Equal(decimal.NewFromInt(100000)) {
			t.Errorf("calculateBilling() amount = %v, want 100000", amount)
		}
		if expiredAt.Before(before.AddDate(0, 1, 0).Add(-1*time.Second)) || expiredAt.After(after.AddDate(0, 1, 0).Add(1*time.Second)) {
			t.Errorf("calculateBilling() expiredAt = %v, want ~1 month from now", expiredAt)
		}
	})

	t.Run("yearly cycle", func(t *testing.T) {
		before := time.Now()
		amount, expiredAt := uc.calculateBilling(plan, "yearly")
		after := time.Now()

		if !amount.Equal(decimal.NewFromInt(1000000)) {
			t.Errorf("calculateBilling() amount = %v, want 1000000", amount)
		}
		if expiredAt.Before(before.AddDate(1, 0, 0).Add(-1*time.Second)) || expiredAt.After(after.AddDate(1, 0, 0).Add(1*time.Second)) {
			t.Errorf("calculateBilling() expiredAt = %v, want ~1 year from now", expiredAt)
		}
	})
}

func TestCalculateBillingWithTransition(t *testing.T) {
	uc := &clientBillingUseCase{}
	basePlan := &entity.PlatformPlan{
		PriceMonthly: decimal.NewFromInt(100000),
		PriceYearly:  decimal.NewFromInt(1000000),
	}
	premiumPlan := &entity.PlatformPlan{
		PriceMonthly: decimal.NewFromInt(200000),
		PriceYearly:  decimal.NewFromInt(2000000),
	}
	budgetPlan := &entity.PlatformPlan{
		PriceMonthly: decimal.NewFromInt(50000),
		PriceYearly:  decimal.NewFromInt(500000),
	}

	t.Run("no existing billing - monthly", func(t *testing.T) {
		before := time.Now()
		amount, startedAt, expiredAt := uc.calculateBillingWithTransition(basePlan, "monthly", nil)
		after := time.Now()

		if !amount.Equal(decimal.NewFromInt(100000)) {
			t.Errorf("amount = %v, want 100000", amount)
		}
		if startedAt.Before(before.Add(-1*time.Second)) || startedAt.After(after.Add(1*time.Second)) {
			t.Errorf("startedAt = %v, want ~now", startedAt)
		}
		if expiredAt.Before(before.AddDate(0, 1, 0).Add(-1*time.Second)) || expiredAt.After(after.AddDate(0, 1, 0).Add(1*time.Second)) {
			t.Errorf("expiredAt = %v, want ~1 month from now", expiredAt)
		}
	})

	t.Run("no existing billing - yearly", func(t *testing.T) {
		before := time.Now()
		amount, _, expiredAt := uc.calculateBillingWithTransition(basePlan, "yearly", nil)
		after := time.Now()

		if !amount.Equal(decimal.NewFromInt(1000000)) {
			t.Errorf("amount = %v, want 1000000", amount)
		}
		if expiredAt.Before(before.AddDate(1, 0, 0).Add(-1*time.Second)) || expiredAt.After(after.AddDate(1, 0, 0).Add(1*time.Second)) {
			t.Errorf("expiredAt = %v, want ~1 year from now", expiredAt)
		}
	})

	t.Run("upgrade activates immediately", func(t *testing.T) {
		existingExpiry := time.Now().Add(15 * 24 * time.Hour)
		existing := &entity.ClientBilling{
			Plan:      *basePlan,
			ExpiredAt: existingExpiry,
		}

		before := time.Now()
		_, startedAt, _ := uc.calculateBillingWithTransition(premiumPlan, "monthly", existing)
		after := time.Now()

		if startedAt.Before(before.Add(-1*time.Second)) || startedAt.After(after.Add(1*time.Second)) {
			t.Errorf("upgrade: startedAt = %v, want ~now (immediate activation)", startedAt)
		}
	})

	t.Run("downgrade starts after existing expiry", func(t *testing.T) {
		existingExpiry := time.Now().Add(15 * 24 * time.Hour)
		existing := &entity.ClientBilling{
			Plan:      *premiumPlan,
			ExpiredAt: existingExpiry,
		}

		_, startedAt, _ := uc.calculateBillingWithTransition(basePlan, "monthly", existing)

		if !startedAt.Equal(existingExpiry) {
			t.Errorf("downgrade: startedAt = %v, want existing.ExpiredAt = %v", startedAt, existingExpiry)
		}
	})

	t.Run("same price activates immediately", func(t *testing.T) {
		samePlan := &entity.PlatformPlan{
			PriceMonthly: decimal.NewFromInt(100000),
			PriceYearly:  decimal.NewFromInt(1000000),
		}
		existing := &entity.ClientBilling{
			Plan:      *basePlan,
			ExpiredAt: time.Now().Add(15 * 24 * time.Hour),
		}

		before := time.Now()
		_, startedAt, _ := uc.calculateBillingWithTransition(samePlan, "monthly", existing)
		after := time.Now()

		if startedAt.Before(before.Add(-1*time.Second)) || startedAt.After(after.Add(1*time.Second)) {
			t.Errorf("same price: startedAt = %v, want ~now", startedAt)
		}
	})

	t.Run("upgrade - yearly", func(t *testing.T) {
		existing := &entity.ClientBilling{
			Plan:      *basePlan,
			ExpiredAt: time.Now().Add(30 * 24 * time.Hour),
		}

		before := time.Now()
		amount, startedAt, expiredAt := uc.calculateBillingWithTransition(premiumPlan, "yearly", existing)
		after := time.Now()

		if !amount.Equal(decimal.NewFromInt(2000000)) {
			t.Errorf("yearly upgrade amount = %v, want 2000000", amount)
		}
		if startedAt.Before(before.Add(-1*time.Second)) || startedAt.After(after.Add(1*time.Second)) {
			t.Errorf("yearly upgrade startedAt = %v, want ~now", startedAt)
		}
		if expiredAt.Before(before.AddDate(1, 0, 0).Add(-1*time.Second)) || expiredAt.After(after.AddDate(1, 0, 0).Add(1*time.Second)) {
			t.Errorf("yearly upgrade expiredAt = %v, want ~1 year from now", expiredAt)
		}
	})

	t.Run("downgrade - yearly", func(t *testing.T) {
		existingExpiry := time.Now().Add(20 * 24 * time.Hour)
		existing := &entity.ClientBilling{
			Plan:      *premiumPlan,
			ExpiredAt: existingExpiry,
		}

		amount, startedAt, expiredAt := uc.calculateBillingWithTransition(budgetPlan, "yearly", existing)

		if !amount.Equal(decimal.NewFromInt(500000)) {
			t.Errorf("yearly downgrade amount = %v, want 500000", amount)
		}
		if !startedAt.Equal(existingExpiry) {
			t.Errorf("yearly downgrade startedAt = %v, want %v", startedAt, existingExpiry)
		}
		if !expiredAt.Equal(existingExpiry.AddDate(1, 0, 0)) {
			t.Errorf("yearly downgrade expiredAt = %v, want %v", expiredAt, existingExpiry.AddDate(1, 0, 0))
		}
	})

	t.Run("existing billing without plan preload should not panic", func(t *testing.T) {
		existing := &entity.ClientBilling{
			ExpiredAt: time.Now().Add(10 * 24 * time.Hour),
		}

		// Should handle zero-value Plan (PriceMonthly = 0)
		_, _, _ = uc.calculateBillingWithTransition(premiumPlan, "monthly", existing)
	})

	t.Run("plan without entity uuid", func(t *testing.T) {
		_ = uuid.New()
		emptyPlan := &entity.PlatformPlan{
			PriceMonthly: decimal.NewFromInt(0),
			PriceYearly:  decimal.NewFromInt(0),
		}

		amount, _, _ := uc.calculateBillingWithTransition(emptyPlan, "monthly", nil)
		if !amount.IsZero() {
			t.Errorf("free plan amount = %v, want 0", amount)
		}
	})
}

func TestCalculateBillingAt(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	plan := &entity.PlatformPlan{
		PriceMonthly: decimal.NewFromInt(50000),
		PriceYearly:  decimal.NewFromInt(500000),
	}

	t.Run("monthly", func(t *testing.T) {
		amount, expiredAt := calculateBillingAt(plan, "monthly", now)
		if !amount.Equal(decimal.NewFromInt(50000)) {
			t.Errorf("amount = %v, want 50000", amount)
		}
		want := now.AddDate(0, 1, 0)
		if !expiredAt.Equal(want) {
			t.Errorf("expiredAt = %v, want %v", expiredAt, want)
		}
	})

	t.Run("yearly", func(t *testing.T) {
		amount, expiredAt := calculateBillingAt(plan, "yearly", now)
		if !amount.Equal(decimal.NewFromInt(500000)) {
			t.Errorf("amount = %v, want 500000", amount)
		}
		want := now.AddDate(1, 0, 0)
		if !expiredAt.Equal(want) {
			t.Errorf("expiredAt = %v, want %v", expiredAt, want)
		}
	})
}
