package discount

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Type string

const (
	TypePercentage Type = "percentage"
	TypeFixed      Type = "fixed"
)

type Discount struct {
	Type        Type
	Value       decimal.Decimal
	MaxDiscount *decimal.Decimal // Cap untuk percentage
	MinPurchase decimal.Decimal
	MaxUsage    int
	UsedCount   int
	ValidFrom   time.Time
	ValidUntil  *time.Time
	IsActive    bool
}

type CalculateResult struct {
	OriginalAmount decimal.Decimal
	DiscountAmount decimal.Decimal
	FinalAmount    decimal.Decimal
}

// Calculate menghitung diskon berdasarkan amount
func Calculate(d *Discount, amount decimal.Decimal) (*CalculateResult, error) {
	if err := validate(d, amount); err != nil {
		return nil, err
	}

	var discountAmount decimal.Decimal

	switch d.Type {
	case TypePercentage:
		discountAmount = amount.Mul(d.Value.Div(decimal.NewFromInt(100)))
		// Apply cap jika ada
		if d.MaxDiscount != nil && discountAmount.GreaterThan(*d.MaxDiscount) {
			discountAmount = *d.MaxDiscount
		}
	case TypeFixed:
		discountAmount = d.Value
		// Diskon flat tidak boleh melebihi amount
		if discountAmount.GreaterThan(amount) {
			discountAmount = amount
		}
	default:
		return nil, fmt.Errorf("discount: unknown type '%s'", d.Type)
	}

	finalAmount := amount.Sub(discountAmount)
	if finalAmount.LessThan(decimal.Zero) {
		finalAmount = decimal.Zero
	}

	return &CalculateResult{
		OriginalAmount: amount,
		DiscountAmount: discountAmount,
		FinalAmount:    finalAmount,
	}, nil
}

// validate memastikan diskon valid untuk digunakan
func validate(d *Discount, amount decimal.Decimal) error {
	if !d.IsActive {
		return fmt.Errorf("diskon tidak aktif")
	}

	now := time.Now()
	if now.Before(d.ValidFrom) {
		return fmt.Errorf("diskon belum berlaku")
	}
	if d.ValidUntil != nil && now.After(*d.ValidUntil) {
		return fmt.Errorf("diskon sudah kadaluarsa")
	}

	if d.MaxUsage != -1 && d.UsedCount >= d.MaxUsage {
		return fmt.Errorf("kuota diskon sudah habis")
	}

	if amount.LessThan(d.MinPurchase) {
		return fmt.Errorf("minimum pembelian Rp%s untuk menggunakan diskon ini", d.MinPurchase.String())
	}

	// Validasi nilai diskon (ditempatkan terakhir: error validasi yang lebih
	// spesifik dilaporkan lebih dulu). Nilai <= 0 atau persentase > 100% akan
	// membuat finalAmount > amount / clamped ke 0 — checkout gagal secara
	// membingungkan; tolak sejak awal dengan pesan jelas.
	if d.Value.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("nilai diskon tidak valid")
	}
	if d.Type == TypePercentage && d.Value.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("persentase diskon tidak valid")
	}

	return nil
}
