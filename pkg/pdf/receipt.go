package pdf

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────
// Domain Models
// ─────────────────────────────────────────────────────────────

// ReceiptItem represents a single line item on the receipt.
type ReceiptItem struct {
	Name     string          // e.g. "Premium Plan"
	Duration string          // e.g. "30 Hari"
	Qty      int             // quantity (usually 1 for subscriptions)
	Price    decimal.Decimal // unit price
	Subtotal decimal.Decimal // Qty * Price
}

// ReceiptData holds all information required to render a payment receipt.
// Construct it from entity.Order, entity.Package, entity.TelegramUser,
// entity.Client, and Midtrans webhook data.
type ReceiptData struct {
	// ── Merchant / Brand ────────────────────────────────────────
	MerchantName    string // displayed prominently in the header
	MerchantAddress string // optional, shown below merchant name
	MerchantLogo    []byte // optional override; falls back to Config.LogoData
	MerchantLogoExt string // "png" or "jpg" (required when MerchantLogo is set)

	// ── Transaction ─────────────────────────────────────────────
	OrderID         string    // e.g. "ORD-20260622-ABCD12"
	TransactionID   string    // Midtrans transaction_id
	TransactionTime string    // formatted date, e.g. "22 Juni 2026, 14:30 WIB"
	PaymentType     string    // e.g. "bank_transfer", "gopay", "credit_card"
	PaymentMethod   string    // display label, e.g. "BCA Virtual Account"
	Status          string    // "paid", "pending", "expired", "failed"
	PaidAt          time.Time // when payment was confirmed

	// ── Customer ────────────────────────────────────────────────
	CustomerName     string // full name
	CustomerEmail    string
	CustomerTelegram string // @username
	CustomerPhone    string

	// ── Line Items ──────────────────────────────────────────────
	Items []ReceiptItem

	// ── Payment Summary ─────────────────────────────────────────
	Subtotal       decimal.Decimal // sum of item subtotals
	DiscountAmount decimal.Decimal // discount applied (0 if none)
	DiscountLabel  string          // e.g. "Diskon Early Bird (10%)"
	TotalPaid      decimal.Decimal // final amount paid

	// ── Optional Fields ─────────────────────────────────────────
	Notes         string    // free-form notes rendered at the bottom
	GeneratedAt   time.Time // stamped automatically if zero
	ReceiptNumber string    // optional custom receipt number

	// ── Currency ────────────────────────────────────────────────
	CurrencyCode string // "IDR" (default), "USD", etc.
}

// ─────────────────────────────────────────────────────────────
// Validation
// ─────────────────────────────────────────────────────────────

// Validate checks that the minimum required fields are present.
func (d *ReceiptData) Validate() error {
	if d.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if d.MerchantName == "" {
		return fmt.Errorf("merchant_name is required")
	}
	if d.CustomerName == "" {
		return fmt.Errorf("customer_name is required")
	}
	if len(d.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}
	if d.TotalPaid.IsNegative() {
		return fmt.Errorf("total_paid must not be negative")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

// StatusLabel returns a human-readable Indonesian label for the given status.
func StatusLabel(status string) string {
	switch status {
	case "paid", "settlement":
		return "LUNAS"
	case "pending":
		return "MENUNGGU"
	case "expired":
		return "KEDALUWARSA"
	case "failed", "deny", "cancel", "failure":
		return "GAGAL"
	default:
		return status
	}
}

// FormatCurrency formats a decimal amount as Indonesian Rupiah by default,
// or using the provided currency code.
// IDR: dot as thousand separator, no decimals (e.g. "Rp 150.000").
// Non-IDR: comma as thousand separator, dot for decimals (e.g. "USD 1,250.50").
func FormatCurrency(amount decimal.Decimal, currencyCode string) string {
	if currencyCode == "" {
		currencyCode = "IDR"
	}

	// IDR has no sub-unit — use dot as thousand separator.
	if currencyCode == "IDR" {
		intVal := amount.IntPart()
		if intVal < 0 {
			return fmt.Sprintf("-%s %s", currencyCode, formatThousandSeparator(-intVal))
		}
		return fmt.Sprintf("%s %s", currencyCode, formatThousandSeparator(intVal))
	}

	// Non-IDR currencies: comma as thousand separator, dot for decimals.
	inCents := amount.Mul(decimal.NewFromInt(100)).IntPart()
	negative := inCents < 0
	if negative {
		inCents = -inCents
	}

	whole := inCents / 100
	cents := inCents % 100
	formatted := fmt.Sprintf("%s.%02d", formatThousandSepComma(whole), cents)

	if negative {
		return fmt.Sprintf("-%s %s", currencyCode, formatted)
	}
	return fmt.Sprintf("%s %s", currencyCode, formatted)
}

// formatThousandSeparator inserts dots as thousand separators (Indonesian style).
// Used for IDR currency.
func formatThousandSeparator(n int64) string {
	return insertDots(fmt.Sprintf("%d", n))
}

// formatThousandSepComma inserts commas as thousand separators (international style).
// Used for non-IDR currencies (e.g. USD 1,250.50).
func formatThousandSepComma(n int64) string {
	return insertCommas(fmt.Sprintf("%d", n))
}

// insertDots adds thousand-separator dots to a numeric string.
func insertDots(s string) string {
	if len(s) <= 3 {
		return s
	}

	remainder := len(s) % 3
	var result []byte
	result = append(result, s[:remainder]...)
	for i := remainder; i < len(s); i += 3 {
		if len(result) > 0 {
			result = append(result, '.')
		}
		result = append(result, s[i:i+3]...)
	}
	return string(result)
}

// insertCommas adds thousand-separator commas to a numeric string.
func insertCommas(s string) string {
	if len(s) <= 3 {
		return s
	}

	remainder := len(s) % 3
	var result []byte
	result = append(result, s[:remainder]...)
	for i := remainder; i < len(s); i += 3 {
		if len(result) > 0 {
			result = append(result, ',')
		}
		result = append(result, s[i:i+3]...)
	}
	return string(result)
}

// PaymentTypeLabel returns a display-friendly label for Midtrans payment types.
func PaymentTypeLabel(paymentType string) string {
	switch paymentType {
	case "bank_transfer":
		return "Transfer Bank"
	case "credit_card":
		return "Kartu Kredit"
	case "gopay":
		return "GoPay"
	case "shopeepay":
		return "ShopeePay"
	case "qris":
		return "QRIS"
	case "cstore":
		return "Convenience Store"
	case "echannel":
		return "Mandiri Bill Payment"
	case "bca_klikbca", "bca_klikpay":
		return "BCA KlikPay"
	case "bri_epay":
		return "BRI e-Pay"
	case "danamon_online":
		return "Danamon Online"
	case "akulaku":
		return "Akulaku"
	default:
		return paymentType
	}
}
