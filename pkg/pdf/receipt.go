//go:build oldreceipt

package pdf

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/linestyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"go.uber.org/zap"
)

type Config struct {
	LogoData []byte
	LogoExt  string
}

type Client struct {
	config *Config
	logger *zap.Logger
}

type receiptPDF struct {
	m      core.Maroto
	cfg    *Config
	logger *zap.Logger
}

func NewClient(cfg *Config, logger *zap.Logger) *Client {
	if cfg == nil {
		cfg = &Config{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Client{config: cfg, logger: logger}
}

func (c *Client) GenerateReceipt(data *ReceiptData) ([]byte, error) {
	pdfBytes, err := GenerateReceipt(data, c.config, c.logger)
	if err != nil {
		return nil, err
	}
	c.logger.Info("pdf: receipt generated",
		zap.String("order_id", data.OrderID),
		zap.Int("bytes", len(pdfBytes)),
	)
	return pdfBytes, nil
}

func (c *Client) GenerateReceiptFile(data *ReceiptData, filePath string) error {
	if err := GenerateReceiptToFile(data, c.config, c.logger, filePath); err != nil {
		return err
	}
	c.logger.Info("pdf: receipt saved to file",
		zap.String("order_id", data.OrderID),
		zap.String("path", filePath),
	)
	return nil
}

func newReceiptPDF(cfg *Config, logger *zap.Logger) *receiptPDF {
	m := maroto.New(config.NewBuilder().
		WithLeftMargin(14).
		WithTopMargin(0).
		WithRightMargin(14).
		WithBottomMargin(14).
		Build())
	return &receiptPDF{m: m, cfg: cfg, logger: logger}
}

func GenerateReceipt(data *ReceiptData, cfg *Config, logger *zap.Logger) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("pdf: receipt data is required")
	}
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("pdf: invalid receipt data: %w", err)
	}
	if data.GeneratedAt.IsZero() {
		data.GeneratedAt = time.Now()
	}

	r := newReceiptPDF(cfg, logger)
	if err := r.render(data); err != nil {
		return nil, fmt.Errorf("pdf render: %w", err)
	}
	doc, err := r.m.Generate()
	if err != nil {
		return nil, fmt.Errorf("pdf generate: %w", err)
	}
	return doc.GetBytes(), nil
}

func GenerateReceiptToWriter(data *ReceiptData, cfg *Config, logger *zap.Logger, w io.Writer) error {
	b, err := GenerateReceipt(data, cfg, logger)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(b))
	return err
}

func GenerateReceiptToFile(data *ReceiptData, cfg *Config, logger *zap.Logger, path string) error {
	b, err := GenerateReceipt(data, cfg, logger)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (r *receiptPDF) render(data *ReceiptData) error {
	r.renderHeader(data)
	r.renderStatusBanner(data.Status)
	r.renderTransactionSection(data)
	r.renderCustomerSection(data)
	r.renderItemsSection(data)
	r.renderSummarySection(data)
	if data.Notes != "" {
		r.renderNotesSection(data.Notes)
	}
	return r.registerFooter(data)
}

func (r *receiptPDF) renderHeader(data *ReceiptData) {
	logoBytes, logoExt := resolveLogo(data, r.cfg)
	ext, hasLogo := imageExt(logoExt)

	logoCol := col.New(2)
	if hasLogo && len(logoBytes) > 0 {
		logoCol = image.NewFromBytesCol(2, logoBytes, ext, props.Rect{
			Center:  true,
			Percent: 75,
		})
	}

	titleCol := col.New(10).Add(
		text.New(strings.ToUpper(sanitize(firstNonEmpty(data.MerchantName, "Receipt"))), props.Text{
			Top:   6,
			Size:  9,
			Style: fontstyle.Bold,
			Color: mutedLight,
			Align: align.Right,
		}),
		text.New("Bukti Pembayaran", props.Text{
			Top:   11,
			Size:  16,
			Style: fontstyle.Bold,
			Color: white,
			Align: align.Right,
		}),
	)
	if data.MerchantAddress != "" {
		titleCol.Add(text.New(sanitize(data.MerchantAddress), props.Text{
			Top:   22,
			Size:  7,
			Color: mutedLight,
			Align: align.Right,
		}))
	}

	r.m.AddRow(30, logoCol, titleCol).WithStyle(&props.Cell{BackgroundColor: navyHeader})

	cur := firstNonEmpty(data.CurrencyCode, "IDR")
	r.m.AddRow(22,
		col.New(6).Add(
			text.New("TOTAL PEMBAYARAN", props.Text{
				Top:   5,
				Left:  4,
				Size:  7,
				Style: fontstyle.Bold,
				Color: mutedLight,
			}),
			text.New(FormatCurrency(data.TotalPaid, cur), props.Text{
				Top:   10,
				Left:  4,
				Size:  20,
				Style: fontstyle.Bold,
				Color: white,
			}),
		),
		col.New(6).Add(
			text.New("ORDER ID", props.Text{
				Top:   5,
				Size:  7,
				Style: fontstyle.Bold,
				Color: mutedLight,
				Align: align.Right,
			}),
			text.New(sanitize(data.OrderID), props.Text{
				Top:   10,
				Size:  8,
				Color: white,
				Align: align.Right,
				Style: fontstyle.Bold,
			}),
			text.New(sanitize(firstNonEmpty(
				data.TransactionTime,
				generatedAt(data.PaidAt).Format("02 Jan 2006, 15:04 WIB"),
			)), props.Text{
				Top:   17,
				Size:  7,
				Color: mutedLight,
				Align: align.Right,
			}),
		),
	).WithStyle(&props.Cell{BackgroundColor: navyMid})
}

func (r *receiptPDF) renderStatusBanner(status string) {
	bg, fg, label := statusColors(status)
	r.m.AddRow(8,
		text.NewCol(12, label, props.Text{
			Top:   2,
			Size:  9,
			Style: fontstyle.Bold,
			Align: align.Center,
			Color: fg,
		}),
	).WithStyle(&props.Cell{BackgroundColor: bg})
}

func (r *receiptPDF) renderSectionLabel(title string) {
	r.m.AddRow(7,
		text.NewCol(12, strings.ToUpper(title), props.Text{
			Top:   2,
			Left:  3,
			Size:  8,
			Style: fontstyle.Bold,
			Color: blue,
		}),
	).WithStyle(&props.Cell{BackgroundColor: surfaceGray})
	r.m.AddRows(line.NewRow(1, props.Line{
		Color:     borderGray,
		Thickness: 0.3,
		Style:     linestyle.Solid,
	}))
}

func (r *receiptPDF) renderInfoRow(label, value string) {
	if value == "" {
		return
	}
	r.m.AddRow(6,
		text.NewCol(4, label, props.Text{Top: 1.5, Size: 8, Color: muted}),
		text.NewCol(8, sanitize(value), props.Text{
			Top:   1.5,
			Size:  8,
			Color: slate,
			Style: fontstyle.Bold,
		}),
	)
}

func (r *receiptPDF) renderInfoRowAccent(label, value string) {
	if value == "" {
		return
	}
	r.m.AddRow(6,
		text.NewCol(4, label, props.Text{Top: 1.5, Size: 8, Color: muted}),
		text.NewCol(8, sanitize(value), props.Text{
			Top:   1.5,
			Size:  8,
			Color: blue,
			Style: fontstyle.Bold,
		}),
	)
}

func (r *receiptPDF) renderTransactionSection(data *ReceiptData) {
	r.renderSectionLabel("Detail Transaksi")
	r.renderInfoRow("No. Pesanan", data.OrderID)
	r.renderInfoRow("ID Transaksi", data.TransactionID)
	r.renderInfoRow("No. Kwitansi", data.ReceiptNumber)
	r.renderInfoRow("Tanggal", firstNonEmpty(
		data.TransactionTime,
		generatedAt(data.PaidAt).Format("02 Januari 2006, 15:04 WIB"),
	))
	r.renderInfoRow("Metode Pembayaran", firstNonEmpty(data.PaymentMethod, PaymentTypeLabel(data.PaymentType)))
	r.m.AddRows(row.New(3))
}

func (r *receiptPDF) renderCustomerSection(data *ReceiptData) {
	r.renderSectionLabel("Detail Pelanggan")
	r.renderInfoRow("Nama", data.CustomerName)
	r.renderInfoRowAccent("Telegram", data.CustomerTelegram)
	r.renderInfoRow("Email", data.CustomerEmail)
	r.renderInfoRow("Telepon", data.CustomerPhone)
	r.m.AddRows(row.New(3))
}

func (r *receiptPDF) renderItemsSection(data *ReceiptData) {
	r.renderSectionLabel("Item Pesanan")
	r.m.AddRow(7,
		text.NewCol(5, "ITEM", props.Text{
			Top: 2, Left: 2, Size: 7, Style: fontstyle.Bold, Color: white,
		}),
		text.NewCol(3, "DURASI", props.Text{
			Top: 2, Size: 7, Style: fontstyle.Bold, Align: align.Center, Color: white,
		}),
		text.NewCol(1, "QTY", props.Text{
			Top: 2, Size: 7, Style: fontstyle.Bold, Align: align.Center, Color: white,
		}),
		text.NewCol(3, "SUBTOTAL", props.Text{
			Top: 2, Size: 7, Style: fontstyle.Bold, Align: align.Right, Color: white,
		}),
	).WithStyle(&props.Cell{BackgroundColor: navyHeader})

	cur := firstNonEmpty(data.CurrencyCode, "IDR")
	for i, item := range data.Items {
		bg := white
		if i%2 == 1 {
			bg = surfaceGray
		}
		r.m.AddRow(10,
			col.New(5).Add(
				text.New(sanitize(truncate(item.Name, 36)), props.Text{
					Top: 1.5, Left: 2, Size: 8, Style: fontstyle.Bold, Color: slate,
				}),
				text.New(FormatCurrency(item.Price, cur)+"/unit", props.Text{
					Top: 6.5, Left: 2, Size: 7, Color: mutedLight,
				}),
			),
			text.NewCol(3, sanitize(item.Duration), props.Text{
				Top: 3, Size: 8, Align: align.Center, Color: muted,
			}),
			text.NewCol(1, fmt.Sprintf("%d", item.Qty), props.Text{
				Top: 3, Size: 8, Align: align.Center, Color: slate,
			}),
			text.NewCol(3, FormatCurrency(item.Subtotal, cur), props.Text{
				Top: 3, Size: 8, Style: fontstyle.Bold, Align: align.Right, Color: slate,
			}),
		).WithStyle(&props.Cell{BackgroundColor: bg})
	}
	r.m.AddRows(row.New(2))
}

func (r *receiptPDF) renderSummarySection(data *ReceiptData) {
	cur := firstNonEmpty(data.CurrencyCode, "IDR")
	r.m.AddRow(7,
		col.New(6),
		text.NewCol(3, "Subtotal", props.Text{
			Top: 2, Size: 8, Align: align.Right, Color: muted,
		}),
		text.NewCol(3, FormatCurrency(data.Subtotal, cur), props.Text{
			Top: 2, Size: 8, Align: align.Right, Color: slate,
		}),
	)

	if data.DiscountAmount.IsPositive() {
		label := firstNonEmpty(data.DiscountLabel, "Diskon")
		r.m.AddRow(7,
			col.New(6),
			text.NewCol(3, sanitize(label), props.Text{
				Top: 2, Size: 8, Align: align.Right, Color: muted,
			}),
			text.NewCol(3, "-"+FormatCurrency(data.DiscountAmount, cur), props.Text{
				Top:   2,
				Size:  8,
				Style: fontstyle.Bold,
				Align: align.Right,
				Color: greenText,
			}),
		)
	}

	r.m.AddRows(line.NewRow(2, props.Line{
		Color:     borderGray,
		Thickness: 0.3,
		Style:     linestyle.Solid,
	}))
	r.m.AddRow(14,
		col.New(6).Add(text.New("TOTAL PEMBAYARAN", props.Text{
			Top: 5, Left: 4, Size: 7, Style: fontstyle.Bold, Color: mutedLight,
		})),
		col.New(6).Add(text.New(FormatCurrency(data.TotalPaid, cur), props.Text{
			Top: 3, Size: 16, Style: fontstyle.Bold, Align: align.Right, Color: white,
		})),
	).WithStyle(&props.Cell{
		BackgroundColor: navyHeader,
		BorderType:      border.Full,
		BorderColor:     navyHeader,
	})
	r.m.AddRows(row.New(4))
}

func (r *receiptPDF) renderNotesSection(notes string) {
	r.renderSectionLabel("Catatan")
	r.m.AddRows(text.NewAutoRow(sanitize(notes), props.Text{
		Size: 8, Color: muted, Bottom: 4,
	}))
}

func (r *receiptPDF) registerFooter(data *ReceiptData) error {
	ts := generatedAt(data.GeneratedAt).Format("02 Januari 2006, 15:04 WIB")
	return r.m.RegisterFooter(
		row.New(1).Add(col.New(12).Add(line.New(props.Line{
			Color:     borderGray,
			Thickness: 0.3,
		}))),
		row.New(10).Add(col.New(12).Add(
			text.New("Terima kasih telah menggunakan layanan kami.", props.Text{
				Top: 2, Size: 8, Align: align.Center, Color: muted,
			}),
			text.New("Dokumen ini sah tanpa tanda tangan.", props.Text{
				Top: 6, Size: 7, Align: align.Center, Style: fontstyle.Italic, Color: mutedLight,
			}),
			text.New("Digenerate otomatis pada "+ts, props.Text{
				Top: 10, Size: 7, Align: align.Center, Style: fontstyle.Italic, Color: mutedLight,
			}),
		)),
	)
}

func resolveLogo(data *ReceiptData, cfg *Config) ([]byte, string) {
	if len(data.MerchantLogo) > 0 && data.MerchantLogoExt != "" {
		return data.MerchantLogo, data.MerchantLogoExt
	}
	if cfg != nil {
		return cfg.LogoData, cfg.LogoExt
	}
	return nil, ""
}

func imageExt(ext string) (extension.Type, bool) {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg":
		return extension.Jpg, true
	case "jpeg":
		return extension.Jpeg, true
	case "png":
		return extension.Png, true
	}
	return "", false
}
