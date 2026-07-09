package pdf

import (
	"testing"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func TestReceiptData_Validate(t *testing.T) {
	validData := ReceiptData{
		OrderID:      "ORD-123",
		MerchantName: "Toko Bahagia",
		CustomerName: "Budi",
		Items: []ReceiptItem{
			{Name: "Item 1"},
		},
		TotalPaid: decimal.NewFromInt(10000),
	}

	tests := []struct {
		name    string
		data    ReceiptData
		wantErr bool
	}{
		{name: "valid data", data: validData},
		{
			name: "missing order_id",
			data: func() ReceiptData {
				d := validData
				d.OrderID = ""
				return d
			}(),
			wantErr: true,
		},
		{
			name: "missing merchant_name",
			data: func() ReceiptData {
				d := validData
				d.MerchantName = ""
				return d
			}(),
			wantErr: true,
		},
		{
			name: "missing customer_name",
			data: func() ReceiptData {
				d := validData
				d.CustomerName = ""
				return d
			}(),
			wantErr: true,
		},
		{
			name: "empty items",
			data: func() ReceiptData {
				d := validData
				d.Items = nil
				return d
			}(),
			wantErr: true,
		},
		{
			name: "negative total_paid",
			data: func() ReceiptData {
				d := validData
				d.TotalPaid = decimal.NewFromInt(-1000)
				return d
			}(),
			wantErr: true,
		},
		{
			name: "zero total_paid is allowed",
			data: func() ReceiptData {
				d := validData
				d.TotalPaid = decimal.Zero
				return d
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReceiptData.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name         string
		amount       decimal.Decimal
		currencyCode string
		want         string
	}{
		{name: "IDR positive", amount: decimal.NewFromInt(150000), currencyCode: "IDR", want: "Rp 150.000"},
		{name: "IDR negative", amount: decimal.NewFromInt(-50000), currencyCode: "IDR", want: "Rp -50.000"},
		{name: "IDR default if empty", amount: decimal.NewFromInt(1000), currencyCode: "", want: "Rp 1.000"},
		{name: "USD positive", amount: decimal.NewFromFloat(1250.5), currencyCode: "USD", want: "$ 1250.50"},
		{name: "USD negative", amount: decimal.NewFromFloat(-15.75), currencyCode: "USD", want: "$ -15.75"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCurrency(tt.amount, tt.currencyCode); got != tt.want {
				t.Errorf("FormatCurrency() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "settlement", want: "LUNAS"},
		{status: "pending", want: "MENUNGGU PEMBAYARAN"},
		{status: "expired", want: "KADALUARSA"},
		{status: "unknown_status", want: "UNKNOWN_STATUS"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := StatusLabel(tt.status); got != tt.want {
				t.Errorf("StatusLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GenerateReceipt(t *testing.T) {
	client := NewClient(nil, zap.NewNop())
	data := &ReceiptData{
		OrderID:         "ORD-TEST-123",
		MerchantName:    "Test Store",
		CustomerName:    "John Doe 🚀",
		TransactionTime: "12 Dec 2026",
		Status:          "paid",
		Items: []ReceiptItem{
			{
				Name:     "Produk A",
				Qty:      1,
				Price:    decimal.NewFromInt(10000),
				Subtotal: decimal.NewFromInt(10000),
			},
			{
				Name:     "Produk B",
				Qty:      2,
				Price:    decimal.NewFromInt(25000),
				Subtotal: decimal.NewFromInt(50000),
			},
		},
		Subtotal:  decimal.NewFromInt(60000),
		TotalPaid: decimal.NewFromInt(60000),
		Notes:     "Terima kasih.",
	}

	pdfBytes, err := client.GenerateReceipt(data)
	if err != nil {
		t.Fatalf("GenerateReceipt() error = %v", err)
	}
	if len(pdfBytes) < 4 {
		t.Fatal("GenerateReceipt() returned too few bytes")
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("GenerateReceipt() did not return a valid PDF signature, got %q", pdfBytes[:4])
	}
}
