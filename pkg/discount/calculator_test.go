package discount

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCalculate(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	tests := []struct {
		name           string
		discount       func() *Discount
		amount         decimal.Decimal
		wantDiscount   decimal.Decimal
		wantFinal      decimal.Decimal
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "percentage 20% off",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(20), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(100000),
			wantDiscount: decimal.NewFromInt(20000),
			wantFinal:    decimal.NewFromInt(80000),
		},
		{
			name: "percentage with cap",
			discount: func() *Discount {
				max := decimal.NewFromInt(10000)
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(50), MaxDiscount: &max, IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(100000),
			wantDiscount: decimal.NewFromInt(10000),
			wantFinal:    decimal.NewFromInt(90000),
		},
		{
			name: "percentage below cap",
			discount: func() *Discount {
				max := decimal.NewFromInt(50000)
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), MaxDiscount: &max, IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(100000),
			wantDiscount: decimal.NewFromInt(10000),
			wantFinal:    decimal.NewFromInt(90000),
		},
		{
			name: "fixed discount",
			discount: func() *Discount {
				return &Discount{Type: TypeFixed, Value: decimal.NewFromInt(25000), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(100000),
			wantDiscount: decimal.NewFromInt(25000),
			wantFinal:    decimal.NewFromInt(75000),
		},
		{
			name: "fixed discount clamped to amount",
			discount: func() *Discount {
				return &Discount{Type: TypeFixed, Value: decimal.NewFromInt(50000), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(30000),
			wantDiscount: decimal.NewFromInt(30000),
			wantFinal:    decimal.NewFromInt(0),
		},
		{
			name: "fixed discount larger than amount",
			discount: func() *Discount {
				return &Discount{Type: TypeFixed, Value: decimal.NewFromInt(100), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(50),
			wantDiscount: decimal.NewFromInt(50),
			wantFinal:    decimal.NewFromInt(0),
		},
		{
			name: "zero amount",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(0),
			wantDiscount: decimal.NewFromInt(0),
			wantFinal:    decimal.NewFromInt(0),
		},
		{
			name: "100% percentage discount",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(100), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:       decimal.NewFromInt(50000),
			wantDiscount: decimal.NewFromInt(50000),
			wantFinal:    decimal.NewFromInt(0),
		},
		{
			name: "unknown discount type",
			discount: func() *Discount {
				return &Discount{Type: "unknown", Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: -1}
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "unknown type",
		},
		{
			name: "inactive discount",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: false, ValidFrom: past, MaxUsage: -1}
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "tidak aktif",
		},
		{
			name: "not yet valid",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: future, MaxUsage: -1}
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "belum berlaku",
		},
		{
			name: "expired discount",
			discount: func() *Discount {
				expired := past
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past.Add(-2 * time.Hour), ValidUntil: &expired, MaxUsage: -1}
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "kadaluarsa",
		},
		{
			name: "max usage exhausted",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: 5, UsedCount: 5}
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "kuota",
		},
		{
			name: "unlimited usage not exhausted",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: -1, UsedCount: 999}
			},
			amount:       decimal.NewFromInt(100000),
			wantDiscount: decimal.NewFromInt(10000),
			wantFinal:    decimal.NewFromInt(90000),
		},
		{
			name: "below min purchase",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: -1, MinPurchase: decimal.NewFromInt(50000)}
			},
			amount:         decimal.NewFromInt(30000),
			wantErr:        true,
			wantErrContain: "minimum",
		},
		{
			name: "exact min purchase",
			discount: func() *Discount {
				return &Discount{Type: TypePercentage, Value: decimal.NewFromInt(10), IsActive: true, ValidFrom: past, MaxUsage: -1, MinPurchase: decimal.NewFromInt(50000)}
			},
			amount:       decimal.NewFromInt(50000),
			wantDiscount: decimal.NewFromInt(5000),
			wantFinal:    decimal.NewFromInt(45000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calculate(tt.discount(), tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("Calculate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.wantErrContain != "" && err != nil {
					if !contains(err.Error(), tt.wantErrContain) {
						t.Errorf("Calculate() error %q does not contain %q", err.Error(), tt.wantErrContain)
					}
				}
				return
			}
			if !got.DiscountAmount.Equal(tt.wantDiscount) {
				t.Errorf("Calculate() DiscountAmount = %v, want %v", got.DiscountAmount, tt.wantDiscount)
			}
			if !got.FinalAmount.Equal(tt.wantFinal) {
				t.Errorf("Calculate() FinalAmount = %v, want %v", got.FinalAmount, tt.wantFinal)
			}
			if !got.OriginalAmount.Equal(tt.amount) {
				t.Errorf("Calculate() OriginalAmount = %v, want %v", got.OriginalAmount, tt.amount)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	tests := []struct {
		name           string
		discount       *Discount
		amount         decimal.Decimal
		wantErr        bool
		wantErrContain string
	}{
		{
			name:           "inactive discount",
			discount:       &Discount{IsActive: false, ValidFrom: past, MaxUsage: -1},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "tidak aktif",
		},
		{
			name:           "not yet valid",
			discount:       &Discount{IsActive: true, ValidFrom: future, MaxUsage: -1},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "belum berlaku",
		},
		{
			name: "expired",
			discount: &Discount{
				IsActive:  true,
				ValidFrom: past.Add(-2 * time.Hour),
				ValidUntil: func() *time.Time { e := past; return &e }(),
				MaxUsage: -1,
			},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "kadaluarsa",
		},
		{
			name:           "usage exhausted",
			discount:       &Discount{IsActive: true, ValidFrom: past, MaxUsage: 3, UsedCount: 3},
			amount:         decimal.NewFromInt(100000),
			wantErr:        true,
			wantErrContain: "kuota",
		},
		{
			name:           "below min purchase",
			discount:       &Discount{IsActive: true, ValidFrom: past, MaxUsage: -1, MinPurchase: decimal.NewFromInt(50000)},
			amount:         decimal.NewFromInt(30000),
			wantErr:        true,
			wantErrContain: "minimum",
		},
		{
			name:     "valid discount",
			discount: &Discount{IsActive: true, ValidFrom: past, MaxUsage: -1},
			amount:   decimal.NewFromInt(100000),
		},
		{
			name:     "unlimited usage not exhausted",
			discount: &Discount{IsActive: true, ValidFrom: past, MaxUsage: -1, UsedCount: 999},
			amount:   decimal.NewFromInt(100000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.discount, tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrContain != "" && err != nil {
				if !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validate() error %q does not contain %q", err.Error(), tt.wantErrContain)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
