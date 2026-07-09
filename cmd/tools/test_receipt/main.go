package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/pdf"
)

func main() {
	out := flag.String("out", "receipt-test.pdf", "output PDF path")
	flag.Parse()

	data := pdf.FintechReceipt{
		Brand:         "TG MANAGER - RECEIPT",
		Status:        "LUNAS",
		OrderID:       "ORDER-7954499305-c092cc05-1783434288582279700",
		TransactionID: "ORDER-7954499305-c092cc05-1783434288582279700",
		Date:          "07 Januari 2026, 21:25 WIB",
		GeneratedAt:   "07 Januari 2026 - 21:25 WIB",

		CustomerName: "Budi Santoso",
		Telegram:     "",
		Email:        "budi@email.com",
		Method:       "bank_transfer",

		Items: []pdf.FintechReceiptItem{
			{
				Name:     "Paket C",
				Meta:     "1x Rp 150.000",
				Duration: "30 Hari\nSisa Langganan sebelumnya: 207 Hari\nAktif s.d. 01 Mar 2027",
				Subtotal: "Rp 150.000",
			},
		},

		Subtotal: "Rp 150.000",
		Discount: "Rp 0",
		Total:    "Rp 150.000",
	}

	if err := pdf.GenerateFintechReceipt(*out, data); err != nil {
		log.Fatal(err)
	}

	fmt.Println(*out)
}
