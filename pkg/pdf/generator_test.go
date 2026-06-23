package pdf

import (
	"testing"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func TestClient_GenerateReceipt(t *testing.T) {
	// Gunakan No-Op logger agar console tidak kotor saat unit test
	logger := zap.NewNop()
	client := NewClient(nil, logger)

	data := &ReceiptData{
		OrderID:         "ORD-TEST-123",
		MerchantName:    "Test Store",
		CustomerName:    "John Doe 🚀", // Emoji untuk mengetes fungsi sanitize
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
		Notes:     "Terima kasih. Ini adalah teks yang sangat panjang untuk menguji fitur multi-cell word wrap dan memastikan tidak menabrak footer PDF maupun memecah page break secara normal.",
	}

	// Tes eksekusi render ke memory bytes
	pdfBytes, err := client.GenerateReceipt(data)
	if err != nil {
		t.Fatalf("GenerateReceipt() error = %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("GenerateReceipt() returned empty byte slice")
	}

	// Validasi Magic Number format file PDF (selalu diawali '%PDF')
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("GenerateReceipt() did not return a valid PDF signature, got %q", pdfBytes[:4])
	}
}
