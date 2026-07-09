package usecase

import (
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/google/uuid"
)

func TestMapMemberOrderWebhookStatus(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(n *midtrans.WebhookNotification)
		want     string
		wantHandled bool
	}{
		{
			name: "capture + accept = paid",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "capture"
				n.FraudStatus = "accept"
			},
			want:        "paid",
			wantHandled: true,
		},
		{
			name: "settlement = paid",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "settlement"
			},
			want:        "paid",
			wantHandled: true,
		},
		{
			name: "expire = expired",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "expire"
			},
			want:        "expired",
			wantHandled: true,
		},
		{
			name: "deny = failed",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "deny"
			},
			want:        "failed",
			wantHandled: true,
		},
		{
			name: "cancel = failed",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "cancel"
			},
			want:        "failed",
			wantHandled: true,
		},
		{
			name: "pending = unhandled",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "pending"
			},
			want:        "",
			wantHandled: false,
		},
		{
			name: "authorize = unhandled",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "authorize"
			},
			want:        "",
			wantHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &midtrans.WebhookNotification{}
			tt.setup(n)
			got, handled := mapMemberOrderWebhookStatus(n)
			if handled != tt.wantHandled {
				t.Errorf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if got != tt.want {
				t.Errorf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapClientBillingWebhookStatus(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(n *midtrans.WebhookNotification)
		want        string
		wantHandled bool
	}{
		{
			name: "capture + accept = active",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "capture"
				n.FraudStatus = "accept"
			},
			want:        "active",
			wantHandled: true,
		},
		{
			name: "settlement = active",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "settlement"
			},
			want:        "active",
			wantHandled: true,
		},
		{
			name: "expire = past_due",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "expire"
			},
			want:        "past_due",
			wantHandled: true,
		},
		{
			name: "deny = cancelled",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "deny"
			},
			want:        "cancelled",
			wantHandled: true,
		},
		{
			name: "pending = unhandled",
			setup: func(n *midtrans.WebhookNotification) {
				n.TransactionStatus = "pending"
			},
			want:        "",
			wantHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &midtrans.WebhookNotification{}
			tt.setup(n)
			got, handled := mapClientBillingWebhookStatus(n)
			if handled != tt.wantHandled {
				t.Errorf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if got != tt.want {
				t.Errorf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateMemberSubscriptionDates(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	duration := 30

	t.Run("new subscription", func(t *testing.T) {
		subID, activatedAt, expiredAt := calculateMemberSubscriptionDates(nil, duration, now)
		if subID == uuid.Nil {
			t.Error("expected non-nil subID")
		}
		if !activatedAt.Equal(now) {
			t.Errorf("activatedAt = %v, want %v", activatedAt, now)
		}
		wantExpiry := now.AddDate(0, 0, duration)
		if !expiredAt.Equal(wantExpiry) {
			t.Errorf("expiredAt = %v, want %v", expiredAt, wantExpiry)
		}
	})

	t.Run("stacking onto existing subscription", func(t *testing.T) {
		existingID := uuid.New()
		existingActivated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		existingExpiry := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		existing := &entity.Subscription{
			ID:          existingID,
			ActivatedAt: existingActivated,
			ExpiredAt:   existingExpiry,
		}

		subID, activatedAt, expiredAt := calculateMemberSubscriptionDates(existing, duration, now)
		if subID != existingID {
			t.Errorf("subID = %v, want %v", subID, existingID)
		}
		if !activatedAt.Equal(existingActivated) {
			t.Errorf("activatedAt = %v, want %v", activatedAt, existingActivated)
		}
		wantExpiry := existingExpiry.AddDate(0, 0, duration)
		if !expiredAt.Equal(wantExpiry) {
			t.Errorf("expiredAt = %v, want %v", expiredAt, wantExpiry)
		}
	})

	t.Run("stacking does not use now for activatedAt", func(t *testing.T) {
		existing := &entity.Subscription{
			ID:          uuid.New(),
			ActivatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			ExpiredAt:   time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		}
		_, activatedAt, _ := calculateMemberSubscriptionDates(existing, duration, now)
		if activatedAt.Equal(now) {
			t.Error("stacking should preserve original ActivatedAt, not use now")
		}
	})

	t.Run("duration days applied correctly", func(t *testing.T) {
		subID, _, expiredAt := calculateMemberSubscriptionDates(nil, 90, now)
		if subID == uuid.Nil {
			t.Error("expected non-nil subID")
		}
		want := now.AddDate(0, 0, 90)
		if !expiredAt.Equal(want) {
			t.Errorf("expiredAt = %v, want %v", expiredAt, want)
		}
	})
}
