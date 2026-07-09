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
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
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

type FintechReceipt struct {
	Brand         string
	Status        string
	OrderID       string
	TransactionID string
	Date          string
	GeneratedAt   string

	CustomerName string
	Telegram     string
	Email        string
	Method       string

	Items    []FintechReceiptItem
	Subtotal string
	Discount string
	Total    string
}

type FintechReceiptItem struct {
	Name     string
	Meta     string
	Duration string
	Subtotal string
}

var (
	receiptNavy      = &props.Color{Red: 15, Green: 23, Blue: 42}
	receiptBlue      = &props.Color{Red: 29, Green: 78, Blue: 216}
	receiptSlate900  = &props.Color{Red: 15, Green: 23, Blue: 42}
	receiptSlate800  = &props.Color{Red: 30, Green: 41, Blue: 59}
	receiptSlate500  = &props.Color{Red: 100, Green: 116, Blue: 139}
	receiptSlate400  = &props.Color{Red: 148, Green: 163, Blue: 184}
	receiptSlate300  = &props.Color{Red: 203, Green: 213, Blue: 225}
	receiptSlate100  = &props.Color{Red: 241, Green: 245, Blue: 249}
	receiptSlate50   = &props.Color{Red: 248, Green: 250, Blue: 252}
	receiptGreen     = &props.Color{Red: 22, Green: 163, Blue: 74}
	receiptGreenSoft = &props.Color{Red: 34, Green: 197, Blue: 94}
)

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

	return generateFintechReceiptBytes(toFintechReceipt(data, cfg))
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

func GenerateFintechReceipt(path string, data FintechReceipt) error {
	b, err := generateFintechReceiptBytes(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func generateFintechReceiptBytes(data FintechReceipt) ([]byte, error) {
	data.Telegram = displayDash(data.Telegram)
	data.Method = displayPaymentMethod(data.Method)

	cfg := config.NewBuilder().
		WithDimensions(110, 170).
		WithLeftMargin(0).
		WithRightMargin(0).
		WithTopMargin(0).
		WithBottomMargin(0).
		Build()

	m := maroto.New(cfg)
	m.AddRows(buildCardHeader(data))
	m.AddRows(buildCardBody(data)...)
	m.AddRows(buildCardFooter(data))

	document, err := m.Generate()
	if err != nil {
		return nil, err
	}
	return document.GetBytes(), nil
}

func toFintechReceipt(data *ReceiptData, cfg *Config) FintechReceipt {
	cur := firstNonEmpty(data.CurrencyCode, "IDR")
	items := make([]FintechReceiptItem, 0, len(data.Items))
	for _, item := range data.Items {
		qty := item.Qty
		if qty <= 0 {
			qty = 1
		}
		items = append(items, FintechReceiptItem{
			Name:     sanitize(truncate(item.Name, 30)),
			Meta:     fmt.Sprintf("%dx %s", qty, FormatCurrency(item.Price, cur)),
			Duration: item.Duration,
			Subtotal: FormatCurrency(item.Subtotal, cur),
		})
	}

	discount := ""
	if data.DiscountAmount.IsPositive() {
		discount = "- " + FormatCurrency(data.DiscountAmount, cur)
	}

	return FintechReceipt{
		Brand:         strings.ToUpper(firstNonEmpty(data.MerchantName, "RECEIPT")) + " - RECEIPT",
		Status:        firstNonEmpty(StatusLabel(data.Status), "LUNAS"),
		OrderID:       sanitize(data.OrderID),
		TransactionID: sanitize(data.TransactionID),
		Date: sanitize(firstNonEmpty(
			data.TransactionTime,
			generatedAt(data.PaidAt).Format("02 Jan 2006, 15:04"),
		)),
		GeneratedAt:  generatedAt(data.GeneratedAt).Format("02 Jan 2006 - 15:04 WIB"),
		CustomerName: sanitize(data.CustomerName),
		Telegram:     displayDash(sanitize(data.CustomerTelegram)),
		Method: displayPaymentMethod(sanitize(firstNonEmpty(
			data.PaymentMethod,
			data.PaymentType,
		))),
		Items:    items,
		Subtotal: FormatCurrency(data.Subtotal, cur),
		Discount: discount,
		Total:    FormatCurrency(data.TotalPaid, cur),
	}
}

func buildCardHeader(data FintechReceipt) core.Row {
	header := col.New(12).WithStyle(&props.Cell{BackgroundColor: receiptNavy})

	return row.New(48).Add(
		header.Add(
			text.New(data.Brand, props.Text{
				Top:   8.5,
				Left:  8,
				Size:  8,
				Style: fontstyle.Bold,
				Color: receiptSlate400,
				Align: align.Left,
			}),
			text.New(data.Status, props.Text{
				Top:   8,
				Right: 8,
				Size:  8,
				Style: fontstyle.Bold,
				Color: receiptGreenSoft,
				Align: align.Right,
			}),
			text.New("TANGGAL", props.Text{
				Top:   16,
				Right: 8,
				Size:  6,
				Color: receiptSlate500,
				Align: align.Right,
			}),
			text.New(data.Date, props.Text{
				Top:   20,
				Right: 8,
				Size:  6.5,
				Color: receiptSlate300,
				Align: align.Right,
			}),
			text.New("TOTAL PEMBAYARAN", props.Text{
				Top:   20,
				Left:  8,
				Size:  7,
				Color: receiptSlate500,
				Align: align.Left,
			}),
			text.New("Rp", props.Text{
				Top:   28,
				Left:  8,
				Size:  11,
				Color: receiptSlate400,
				Align: align.Left,
			}),
			text.New(stripRp(data.Total), props.Text{
				Top:   25,
				Left:  16,
				Size:  26,
				Style: fontstyle.Bold,
				Color: white,
				Align: align.Left,
			}),
			text.New("ORDER ID", props.Text{
				Top:   39,
				Left:  8,
				Size:  6,
				Color: receiptSlate500,
				Align: align.Left,
			}),
			text.New(data.OrderID, props.Text{
				Top:   43,
				Left:  8,
				Size:  5.2,
				Color: receiptSlate300,
				Align: align.Left,
			}),
		),
	)
}

func buildCardBody(data FintechReceipt) []core.Row {
	rows := []core.Row{
		spacer(6),
		buildSectionLabel("MEMBER"),
		buildCustomerGrid(data),
		spacer(4),
		buildDivider(),
		spacer(4),
		buildSectionLabel("ITEM PESANAN"),
		buildItemsHeader(),
	}

	for _, item := range data.Items {
		rows = append(rows, buildItemRow(item))
	}

	rows = append(rows,
		spacer(3),
		buildSummaryRow("Subtotal", data.Subtotal, receiptSlate500),
	)
	if data.Discount != "" {
		rows = append(rows, buildSummaryRow("Diskon", data.Discount, receiptGreen))
	}
	rows = append(rows,
		buildTotalRow(data.Total),
		spacer(6),
	)

	return rows
}

func buildSectionLabel(label string) core.Row {
	return row.New(6).Add(
		text.NewCol(12, label, props.Text{
			Top:   1,
			Left:  8,
			Size:  6.5,
			Style: fontstyle.Bold,
			Color: receiptSlate400,
			Align: align.Left,
		}),
	)
}

func buildCustomerGrid(data FintechReceipt) core.Row {
	namaColWidth := 6
	telegramColWidth := 6
	if data.Telegram == "-" {
		namaColWidth = 12
		telegramColWidth = 0
	}

	r := row.New(22).Add(
		col.New(namaColWidth).Add(
			text.New("Nama", props.Text{Top: 1, Left: 8, Size: 7, Color: receiptSlate400}),
			text.New(data.CustomerName, props.Text{
				Top:   6,
				Left:  8,
				Size:  8,
				Style: fontstyle.Bold,
				Color: receiptSlate800,
			}),
			text.New("Metode", props.Text{Top: 13, Left: 8, Size: 7, Color: receiptSlate400}),
			text.New(data.Method, props.Text{
				Top:   18,
				Left:  8,
				Size:  8,
				Style: fontstyle.Bold,
				Color: receiptSlate800,
			}),
		),
	)

	if telegramColWidth > 0 {
		r.Add(
			col.New(telegramColWidth).Add(
				text.New("Telegram", props.Text{Top: 1, Left: 2, Size: 7, Color: receiptSlate400}),
				text.New(data.Telegram, props.Text{
					Top:   6,
					Left:  2,
					Size:  8,
					Style: fontstyle.Bold,
					Color: receiptBlue,
				}),
			),
		)
	}

	return r
}

func buildDivider() core.Row {
	return line.NewRow(2, props.Line{
		Color:       receiptSlate100,
		Thickness:   0.3,
		SizePercent: 86,
	})
}

func buildItemsHeader() core.Row {
	return row.New(7).Add(
		text.NewCol(6, "ITEM", props.Text{
			Top:   1,
			Left:  8,
			Size:  6,
			Style: fontstyle.Bold,
			Color: receiptSlate400,
			Align: align.Left,
		}),
		text.NewCol(2, "DUR.", props.Text{
			Top:   1,
			Size:  6,
			Style: fontstyle.Bold,
			Color: receiptSlate400,
			Align: align.Center,
		}),
		text.NewCol(4, "SUBTOTAL", props.Text{
			Top:   1,
			Right: 8,
			Size:  6,
			Style: fontstyle.Bold,
			Color: receiptSlate400,
			Align: align.Right,
		}),
	)
}

func buildItemRow(item FintechReceiptItem) core.Row {
	durationLines := receiptTextLines(item.Duration, 34)
	duration := "-"
	if len(durationLines) > 0 {
		duration = durationLines[0]
	}
	details := []string{}
	if len(durationLines) > 1 {
		details = durationLines[1:]
	}
	height := 12.0
	if len(details) > 0 {
		height += float64(len(details)) * 4
	}

	itemCol := col.New(6).Add(
		text.New(item.Name, props.Text{
			Top:   1,
			Left:  8,
			Size:  8,
			Style: fontstyle.Bold,
			Color: receiptSlate800,
		}),
		text.New(item.Meta, props.Text{
			Top:   6,
			Left:  8,
			Size:  7,
			Color: receiptSlate400,
		}),
	)
	for i, detail := range details {
		itemCol.Add(text.New(detail, props.Text{
			Top:   10 + float64(i)*4,
			Left:  8,
			Size:  6.2,
			Color: receiptSlate400,
		}))
	}

	return row.New(height).Add(
		itemCol,
		text.NewCol(2, duration, props.Text{
			Top:   3,
			Size:  7,
			Color: receiptSlate400,
			Align: align.Center,
		}),
		text.NewCol(4, item.Subtotal, props.Text{
			Top:   3,
			Right: 8,
			Size:  8,
			Style: fontstyle.Bold,
			Color: receiptSlate800,
			Align: align.Right,
		}),
	)
}

func buildSummaryRow(label, amount string, color *props.Color) core.Row {
	return row.New(6).Add(
		text.NewCol(7, label, props.Text{
			Top:   1,
			Left:  8,
			Size:  8,
			Color: color,
			Align: align.Left,
		}),
		text.NewCol(5, amount, props.Text{
			Top:   1,
			Right: 8,
			Size:  8,
			Color: color,
			Align: align.Right,
		}),
	)
}

func buildTotalRow(total string) core.Row {
	return row.New(14).Add(
		col.New(12).Add(
			line.New(props.Line{
				Color:         receiptSlate100,
				Thickness:     0.3,
				OffsetPercent: 5,
				SizePercent:   86,
			}),
			text.New("TOTAL", props.Text{
				Top:   6,
				Left:  8,
				Size:  7,
				Style: fontstyle.Bold,
				Color: receiptSlate900,
				Align: align.Left,
			}),
			text.New(total, props.Text{
				Top:   4,
				Right: 8,
				Size:  15,
				Style: fontstyle.Bold,
				Color: receiptBlue,
				Align: align.Right,
			}),
		),
	)
}

func buildCardFooter(data FintechReceipt) core.Row {
	return row.New(18).Add(
		col.New(12).
			WithStyle(&props.Cell{
				BackgroundColor: receiptSlate50,
				BorderColor:     receiptSlate100,
				BorderType:      border.Top,
				BorderThickness: 0.2,
			}).
			Add(
				text.New("Dokumen sah tanpa tanda tangan.", props.Text{
					Top:   4,
					Left:  8,
					Size:  7,
					Color: receiptSlate400,
					Align: align.Left,
				}),
				text.New("Terima kasih atas kepercayaan Anda.", props.Text{
					Top:   9,
					Left:  8,
					Size:  7,
					Color: receiptSlate400,
					Align: align.Left,
				}),
				text.New(truncate("www.urator.com", 28), props.Text{
					Top:   4,
					Right: 8,
					Size:  5.5,
					Color: receiptSlate300,
					Align: align.Right,
				}),
				text.New(data.GeneratedAt, props.Text{
					Top:   9,
					Right: 8,
					Size:  6.5,
					Color: receiptSlate300,
					Align: align.Right,
				}),
			),
	)
}

func spacer(height float64) core.Row {
	return row.New(height).Add(col.New(12))
}

func stripRp(amount string) string {
	return strings.TrimPrefix(amount, "Rp ")
}

func displayDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func displayPaymentMethod(value string) string {
	label := PaymentTypeLabel(strings.TrimSpace(value))
	if label == "" {
		return "-"
	}
	return label
}

func receiptTextLines(value string, maxChars int) []string {
	lines := []string{}
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, raw := range strings.Split(normalized, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lines = append(lines, wrapReceiptLine(sanitize(line), maxChars)...)
	}
	return lines
}

func wrapReceiptLine(line string, maxChars int) []string {
	if len(line) <= maxChars {
		return []string{line}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return nil
	}

	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > maxChars {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	return append(lines, current)
}
