package pdf

import (
	"testing"

	"github.com/shopspring/decimal"
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
		{
			name:    "valid data",
			data:    validData,
			wantErr: false,
		},
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
				d.TotalPaid = decimal.NewFromInt(0)
				return d
			}(),
			wantErr: false,
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
		{
			name:         "IDR positive",
			amount:       decimal.NewFromInt(150000),
			currencyCode: "IDR",
			want:         "IDR 150.000",
		},
		{
			name:         "IDR negative",
			amount:       decimal.NewFromInt(-50000),
			currencyCode: "IDR",
			want:         "-IDR 50.000",
		},
		{
			name:         "IDR default if empty",
			amount:       decimal.NewFromInt(1000),
			currencyCode: "",
			want:         "IDR 1.000",
		},
		{
			name:         "USD positive with decimals",
			amount:       decimal.NewFromFloat(1250.50),
			currencyCode: "USD",
			want:         "USD 1,250.50",
		},
		{
			name:         "USD positive exact integer",
			amount:       decimal.NewFromInt(100),
			currencyCode: "USD",
			want:         "USD 100.00",
		},
		{
			name:         "USD negative",
			amount:       decimal.NewFromFloat(-15.75),
			currencyCode: "USD",
			want:         "-USD 15.75",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCurrency(tt.amount, tt.currencyCode)
			if got != tt.want {
				t.Errorf("FormatCurrency() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusLabel(t *testing.T) {
	if got := StatusLabel("settlement"); got != "LUNAS" {
		t.Errorf("StatusLabel(settlement) = %v, want LUNAS", got)
	}
	if got := StatusLabel("pending"); got != "MENUNGGU" {
		t.Errorf("StatusLabel(pending) = %v, want MENUNGGU", got)
	}
	if got := StatusLabel("unknown_status"); got != "unknown_status" {
		t.Errorf("StatusLabel(unknown_status) = %v, want unknown_status", got)
	}
}
