package pdf

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type ReceiptItem struct {
	Name     string
	Duration string
	Qty      int
	Price    decimal.Decimal
	Subtotal decimal.Decimal
}

type ReceiptData struct {
	MerchantName    string
	MerchantAddress string
	MerchantLogo    []byte
	MerchantLogoExt string

	OrderID         string
	TransactionID   string
	TransactionTime string
	PaymentType     string
	PaymentMethod   string
	Status          string
	PaidAt          time.Time

	CustomerName     string
	CustomerEmail    string
	CustomerTelegram string
	CustomerPhone    string

	Items []ReceiptItem

	Subtotal       decimal.Decimal
	DiscountAmount decimal.Decimal
	DiscountLabel  string
	TotalPaid      decimal.Decimal

	Notes         string
	GeneratedAt   time.Time
	ReceiptNumber string
	CurrencyCode  string
}

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
