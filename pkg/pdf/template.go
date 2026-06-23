package pdf

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"go.uber.org/zap"
)

// ─────────────────────────────────────────────────────────────
// Design Tokens
// ─────────────────────────────────────────────────────────────

const (
	pageW, pageH            = 210.0, 297.0 // A4 mm
	marginLeft, marginRight = 20.0, 20.0
	marginTop, marginBottom = 15.0, 15.0
	contentW                = pageW - marginLeft - marginRight // 170mm
	lineH                   = 6.0
	sectionSpacing          = 8.0
	headerHeight            = 30.0
	statusBadgeH            = 8.0
	tableRowH               = 7.0
	tableHeaderH            = 8.0

	// Color palette (RGB 0–255)
	primaryR, primaryG, primaryB       = 37, 99, 235   // blue-600
	primaryLtR, primaryLtG, primaryLtB = 219, 234, 254 // blue-100
	successR, successG, successB       = 22, 163, 74   // green-600
	warningR, warningG, warningB       = 234, 179, 8   // yellow-500
	dangerR, dangerG, dangerB          = 220, 38, 38   // red-600
	textDarkR, textDarkG, textDarkB    = 15, 23, 42    // slate-900
	textMidR, textMidG, textMidB       = 100, 116, 139 // slate-500
	borderR, borderG, borderB          = 226, 232, 240 // slate-200
	altRowR, altRowG, altRowB          = 248, 250, 252 // slate-50
)

// ─────────────────────────────────────────────────────────────
// Template
// ─────────────────────────────────────────────────────────────

// receiptTemplate encapsulates the PDF rendering state for a single receipt.
type receiptTemplate struct {
	pdf    *gofpdf.Fpdf
	config *Config
	logger *zap.Logger
}

// newReceiptTemplate creates a template with a fresh, A4 portrait PDF.
func newReceiptTemplate(cfg *Config, logger *zap.Logger) *receiptTemplate {
	pdf := gofpdf.New("P", "mm", "A4", cfg.FontDir)
	pdf.SetMargins(marginLeft, marginTop, marginRight)
	pdf.SetAutoPageBreak(false, marginBottom)

	// Register Helvetica as UTF-8 capable (best-effort for Latin-1 chars).
	// For full Unicode, add a TTF font via pdf.AddUTF8FontFromBytes().
	pdf.AddPage()

	return &receiptTemplate{
		pdf:    pdf,
		config: cfg,
		logger: logger,
	}
}

// Render draws the complete receipt onto the PDF canvas.
func (t *receiptTemplate) Render(data *ReceiptData) error {
	y := marginTop
	contentX := marginLeft

	// ── Header ────────────────────────────────────────────────
	y = t.drawHeader(contentX, y, data)
	y += sectionSpacing

	// ── Status Banner ─────────────────────────────────────────
	y = t.ensureSpace(y, statusBadgeH+sectionSpacing)
	y = t.drawStatusBanner(contentX, y, data.Status)
	y += sectionSpacing

	// ── Transaction Info ──────────────────────────────────────
	y = t.ensureSpace(y, 50)
	y = t.drawSectionHeader(contentX, y, "DETAIL TRANSAKSI")
	y = t.drawInfoRow(contentX, y, "No. Pesanan", data.OrderID)
	if data.TransactionID != "" {
		y = t.drawInfoRow(contentX, y, "ID Transaksi", data.TransactionID)
	}
	y = t.drawInfoRow(contentX, y, "Tanggal", data.TransactionTime)
	y = t.drawInfoRow(contentX, y, "Metode Pembayaran", data.PaymentMethod)
	if data.PaymentType != "" {
		y = t.drawInfoRow(contentX, y, "Tipe Pembayaran", PaymentTypeLabel(data.PaymentType))
	}
	y += sectionSpacing

	// ── Customer Info ─────────────────────────────────────────
	y = t.ensureSpace(y, 40)
	y = t.drawSectionHeader(contentX, y, "DETAIL PELANGGAN")
	y = t.drawInfoRow(contentX, y, "Nama", data.CustomerName)
	if data.CustomerTelegram != "" {
		y = t.drawInfoRow(contentX, y, "Telegram", data.CustomerTelegram)
	}
	if data.CustomerEmail != "" {
		y = t.drawInfoRow(contentX, y, "Email", data.CustomerEmail)
	}
	if data.CustomerPhone != "" {
		y = t.drawInfoRow(contentX, y, "Telepon", data.CustomerPhone)
	}
	y += sectionSpacing

	// ── Items Table ───────────────────────────────────────────
	minTableH := tableHeaderH + float64(len(data.Items))*tableRowH + sectionSpacing
	y = t.ensureSpace(y, minTableH)
	y = t.drawSectionHeader(contentX, y, "DETAIL PESANAN")
	y = t.drawItemsTable(contentX, y, data)
	y += sectionSpacing

	// ── Payment Summary ───────────────────────────────────────
	y = t.ensureSpace(y, 50)
	y = t.drawPaymentSummary(contentX, y, data)
	y += sectionSpacing

	// ── Notes ─────────────────────────────────────────────────
	if data.Notes != "" {
		y = t.ensureSpace(y, 30)
		y = t.drawSectionHeader(contentX, y, "CATATAN")
		y = t.drawNotes(contentX, y, data.Notes)
		y += sectionSpacing
	}

	// ── Footer ────────────────────────────────────────────────
	t.drawFooter(contentX, data)

	return nil
}

// Output writes the rendered PDF to the given writer.
func (t *receiptTemplate) Output(w io.Writer) error {
	return t.pdf.Output(w)
}

// OutputFile writes the rendered PDF to a file on disk.
func (t *receiptTemplate) OutputFile(path string) error {
	return t.pdf.OutputFileAndClose(path)
}

// ─────────────────────────────────────────────────────────────
// Section Renderers
// ─────────────────────────────────────────────────────────────

// drawHeader renders the logo (if available) + merchant name + receipt title.
func (t *receiptTemplate) drawHeader(x, y float64, data *ReceiptData) float64 {
	logoData := data.MerchantLogo
	logoExt := data.MerchantLogoExt
	if logoData == nil {
		logoData = t.config.LogoData
		logoExt = t.config.LogoExt
	}

	logoW := 25.0
	textX := x

	if logoData != nil && logoExt != "" {
		opt := gofpdf.ImageOptions{ImageType: logoExt, ReadDpi: false}
		name := fmt.Sprintf("logo_%s", data.OrderID) // unique per doc
		t.pdf.RegisterImageOptionsReader(name, opt, bytes.NewReader(logoData))
		info := t.pdf.GetImageInfo(name)
		if info != nil {
			// Scale proportionally, cap at 25mm wide and 20mm tall,
			// maintaining aspect ratio in both constraints.
			w, h := info.Width(), info.Height()
			if w > logoW {
				ratio := logoW / w
				w = logoW
				h *= ratio
			}
			if h > 20 {
				ratio := 20.0 / h
				h = 20
				w *= ratio
			}
			t.pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")
			textX = x + logoW + 5
		}
	}

	// Merchant name
	t.pdf.SetFont("Helvetica", "B", 16)
	t.pdf.SetTextColor(primaryR, primaryG, primaryB)
	t.pdf.SetXY(textX, y)
	t.pdf.CellFormat(0, 8, data.MerchantName, "", 1, "L", false, 0, "")

	// Merchant address (if any)
	if data.MerchantAddress != "" {
		t.pdf.SetFont("Helvetica", "", 8)
		t.pdf.SetTextColor(textMidR, textMidG, textMidB)
		t.pdf.SetXY(textX, y+9)
		t.pdf.CellFormat(0, 5, data.MerchantAddress, "", 1, "L", false, 0, "")
	}

	// "BUKTI PEMBAYARAN" title, right-aligned
	titleY := y
	t.pdf.SetFont("Helvetica", "B", 12)
	t.pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	t.pdf.SetXY(x, titleY+18)
	t.pdf.CellFormat(contentW, 7, "BUKTI PEMBAYARAN", "", 1, "C", false, 0, "")

	// Receipt number
	if data.ReceiptNumber != "" {
		t.pdf.SetFont("Helvetica", "", 8)
		t.pdf.SetTextColor(textMidR, textMidG, textMidB)
		t.pdf.SetXY(x, titleY+26)
		t.pdf.CellFormat(contentW, 5, "No: "+data.ReceiptNumber, "", 1, "C", false, 0, "")
	}

	return y + headerHeight
}

// drawStatusBanner draws a colored bar indicating payment status.
func (t *receiptTemplate) drawStatusBanner(x, y float64, status string) float64 {
	label := StatusLabel(status)

	// Pick colors based on status.
	var bgR, bgG, bgB, fgR, fgG, fgB int
	switch status {
	case "paid", "settlement":
		bgR, bgG, bgB = 220, 252, 231 // green-100
		fgR, fgG, fgB = successR, successG, successB
	case "pending":
		bgR, bgG, bgB = 254, 249, 195 // yellow-100
		fgR, fgG, fgB = 161, 98, 7    // amber-800
	case "expired":
		bgR, bgG, bgB = 254, 226, 226 // red-100
		fgR, fgG, fgB = dangerR, dangerG, dangerB
	default:
		bgR, bgG, bgB = 254, 226, 226
		fgR, fgG, fgB = dangerR, dangerG, dangerB
	}

	// Background fill
	t.pdf.SetFillColor(bgR, bgG, bgB)
	t.pdf.Rect(x, y, contentW, statusBadgeH, "F")

	// Status text centered
	t.pdf.SetFont("Helvetica", "B", 10)
	t.pdf.SetTextColor(fgR, fgG, fgB)
	t.pdf.SetXY(x, y)
	t.pdf.CellFormat(contentW, statusBadgeH, fmt.Sprintf("Status: %s", label), "", 1, "C", false, 0, "")

	return y + statusBadgeH
}

// drawSectionHeader draws a colored section title with a bottom border.
func (t *receiptTemplate) drawSectionHeader(x, y float64, title string) float64 {
	// Light blue background strip
	t.pdf.SetFillColor(primaryLtR, primaryLtG, primaryLtB)
	t.pdf.Rect(x, y, contentW, 7, "F")

	t.pdf.SetFont("Helvetica", "B", 9)
	t.pdf.SetTextColor(primaryR, primaryG, primaryB)
	t.pdf.SetXY(x+3, y)
	t.pdf.CellFormat(contentW-6, 7, title, "", 1, "L", false, 0, "")

	return y + 9 // 7mm bar + 2mm spacing
}

// drawInfoRow renders a "Label : Value" pair on a single line.
func (t *receiptTemplate) drawInfoRow(x, y float64, label, value string) float64 {
	labelW := 50.0
	valueW := contentW - labelW

	// Sanitize for Helvetica Latin-1 compatibility (#5)
	value = sanitize(value)

	// Label
	t.pdf.SetFont("Helvetica", "", 9)
	t.pdf.SetTextColor(textMidR, textMidG, textMidB)
	t.pdf.SetXY(x+3, y)
	t.pdf.CellFormat(labelW, lineH, label, "", 0, "L", false, 0, "")

	// Separator colon
	t.pdf.SetXY(x+3+labelW-5, y)
	t.pdf.CellFormat(5, lineH, ":", "", 0, "L", false, 0, "")

	// Value
	t.pdf.SetFont("Helvetica", "", 9)
	t.pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	t.pdf.SetXY(x+3+labelW, y)
	t.pdf.CellFormat(valueW, lineH, value, "", 1, "L", false, 0, "")

	return y + lineH + 1
}

// drawItemsTable renders the line items in a bordered table with alternating rows.
// Automatically adds new pages when rows would overflow the footer zone (#1, #2).
func (t *receiptTemplate) drawItemsTable(x, y float64, data *ReceiptData) float64 {
	colW := []float64{55, 30, 15, 35, 35}
	colLabel := []string{"Item", "Durasi", "Qty", "Harga", "Subtotal"}

	y = t.drawTableHeader(x, y, colW, colLabel)

	// Table rows
	t.pdf.SetFont("Helvetica", "", 8)
	for idx, item := range data.Items {
		// Check if this row would overlap the footer zone (#1, #2)
		if y+tableRowH > pageH-marginBottom-20 {
			t.pdf.AddPage()
			y = marginTop
			y = t.drawTableHeader(x, y, colW, colLabel)
		}

		// Alternating row background
		if idx%2 == 1 {
			t.pdf.SetFillColor(altRowR, altRowG, altRowB)
		} else {
			t.pdf.SetFillColor(255, 255, 255)
		}
		t.pdf.SetTextColor(textDarkR, textDarkG, textDarkB)

		rowData := []string{
			sanitize(truncate(item.Name, 30)),
			sanitize(item.Duration),
			fmt.Sprintf("%d", item.Qty),
			FormatCurrency(item.Price, data.CurrencyCode),
			FormatCurrency(item.Subtotal, data.CurrencyCode),
		}

		cx := x
		for i, val := range rowData {
			align := "L"
			if i >= 2 {
				align = "R"
			}
			t.pdf.SetXY(cx, y)
			fill := idx%2 == 1
			t.pdf.CellFormat(colW[i], tableRowH, val, "1", 0, align, fill, 0, "")
			cx += colW[i]
		}
		t.pdf.Ln(-1)
		y += tableRowH
	}

	return y
}

// drawPaymentSummary renders the subtotal / discount / total section.
func (t *receiptTemplate) drawPaymentSummary(x, y float64, data *ReceiptData) float64 {
	rightLabelW := 80.0
	rightValueW := contentW - rightLabelW
	summaryX := x + rightLabelW

	// Subtotal
	t.pdf.SetFont("Helvetica", "", 9)
	t.pdf.SetTextColor(textMidR, textMidG, textMidB)
	t.pdf.SetXY(summaryX, y)
	t.pdf.CellFormat(rightLabelW, lineH, "Subtotal", "", 0, "R", false, 0, "")
	t.pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	t.pdf.SetXY(summaryX+rightLabelW, y)
	t.pdf.CellFormat(rightValueW, lineH, " "+FormatCurrency(data.Subtotal, data.CurrencyCode), "", 1, "L", false, 0, "")
	y += lineH + 2

	// Discount (only if > 0)
	if data.DiscountAmount.IsPositive() {
		t.pdf.SetFont("Helvetica", "", 9)
		t.pdf.SetTextColor(textMidR, textMidG, textMidB)
		t.pdf.SetXY(summaryX, y)
		label := "Diskon"
		if data.DiscountLabel != "" {
			label = data.DiscountLabel
		}
		t.pdf.CellFormat(rightLabelW, lineH, label, "", 0, "R", false, 0, "")
		t.pdf.SetTextColor(dangerR, dangerG, dangerB)
		t.pdf.SetXY(summaryX+rightLabelW, y)
		t.pdf.CellFormat(rightValueW, lineH, " -"+FormatCurrency(data.DiscountAmount, data.CurrencyCode), "", 1, "L", false, 0, "")
		y += lineH + 2
	}

	// Divider line
	t.pdf.SetDrawColor(borderR, borderG, borderB)
	t.pdf.SetLineWidth(0.3)
	t.pdf.Line(summaryX, y, summaryX+rightLabelW+rightValueW, y)
	y += 3

	// Total Paid (bold, large)
	t.pdf.SetFont("Helvetica", "B", 12)
	t.pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	t.pdf.SetXY(summaryX, y)
	t.pdf.CellFormat(rightLabelW, 9, "TOTAL", "", 0, "R", false, 0, "")
	t.pdf.SetTextColor(primaryR, primaryG, primaryB)
	t.pdf.SetXY(summaryX+rightLabelW, y)
	t.pdf.CellFormat(rightValueW, 9, " "+FormatCurrency(data.TotalPaid, data.CurrencyCode), "", 1, "L", false, 0, "")
	y += 12

	return y
}

// drawNotes renders free-form notes text.
// Handles long notes that may exceed a single page by pre-calculating
// the required height and moving to a new page if needed.
func (t *receiptTemplate) drawNotes(x, y float64, notes string) float64 {
	sanitized := sanitize(notes)
	t.pdf.SetFont("Helvetica", "", 8)
	t.pdf.SetTextColor(textMidR, textMidG, textMidB)

	// Pre-calculate the height MultiCell will need.
	lines := t.pdf.SplitLines([]byte(sanitized), contentW-6)
	lineH := 4.0
	notesH := float64(len(lines))*lineH + 4 // small buffer

	// If notes won't fit on current page, move to a new page first.
	footerY := pageH - marginBottom - 20
	if y+notesH > footerY {
		t.pdf.AddPage()
		y = marginTop
	}

	t.pdf.SetXY(x+3, y)
	t.pdf.MultiCell(contentW-6, lineH, sanitized, "", "L", false)
	return t.pdf.GetY() + 2
}

// drawFooter renders the generation timestamp and a thank-you line at the bottom.
func (t *receiptTemplate) drawFooter(x float64, data *ReceiptData) {
	footerY := pageH - marginBottom - 15

	// Divider
	t.pdf.SetDrawColor(borderR, borderG, borderB)
	t.pdf.SetLineWidth(0.2)
	t.pdf.Line(x, footerY, x+contentW, footerY)
	footerY += 3

	// Generation timestamp
	genTime := data.GeneratedAt
	if genTime.IsZero() {
		genTime = time.Now()
	}
	timestamp := fmt.Sprintf("Dokumen ini digenerate secara otomatis pada %s",
		genTime.Format("02 Januari 2006, 15:04 WIB"))

	t.pdf.SetFont("Helvetica", "I", 7)
	t.pdf.SetTextColor(textMidR, textMidG, textMidB)
	t.pdf.SetXY(x, footerY)
	t.pdf.CellFormat(contentW, 4, timestamp, "", 1, "C", false, 0, "")

	// Thank you
	t.pdf.SetFont("Helvetica", "", 8)
	t.pdf.SetXY(x, footerY+5)
	t.pdf.CellFormat(contentW, 5, "Terima kasih atas pembayaran Anda.", "", 1, "C", false, 0, "")
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

// ensureSpace checks if there's enough vertical space on the current page.
// If not, it adds a new page and returns the top margin as the new Y position.
// This prevents content overflow and footer collision (#1, #2).
func (t *receiptTemplate) ensureSpace(currentY, neededH float64) float64 {
	footerY := pageH - marginBottom - 20 // 20mm footer reservation
	if currentY+neededH > footerY {
		t.pdf.AddPage()
		return marginTop
	}
	return currentY
}

// drawTableHeader draws the items table header row. Extracted to allow
// re-rendering after automatic page breaks (#1).
func (t *receiptTemplate) drawTableHeader(x, y float64, colW []float64, colLabel []string) float64 {
	t.pdf.SetFillColor(primaryR, primaryG, primaryB)
	t.pdf.SetFont("Helvetica", "B", 8)
	t.pdf.SetTextColor(255, 255, 255)

	cx := x
	for i, label := range colLabel {
		align := "L"
		if i >= 2 {
			align = "R"
		}
		t.pdf.SetXY(cx, y)
		t.pdf.CellFormat(colW[i], tableHeaderH, label, "1", 0, align, true, 0, "")
		cx += colW[i]
	}
	t.pdf.Ln(-1)
	return y + tableHeaderH
}

// sanitize strips non-ASCII characters from a string to ensure compatibility
// with Helvetica (Latin-1 / cp1252 only). This prevents garbled output or
// panics from Telegram usernames containing emojis, Cyrillic, CJK, etc. (#5).
func sanitize(s string) string {
	// Map common Unicode punctuation to ASCII equivalents
	s = strings.ReplaceAll(s, "–", "-")   // en-dash
	s = strings.ReplaceAll(s, "—", "-")   // em-dash
	s = strings.ReplaceAll(s, "‘", "'")   // left single quote
	s = strings.ReplaceAll(s, "’", "'")   // right single quote
	s = strings.ReplaceAll(s, "“", "\"")  // left double quote
	s = strings.ReplaceAll(s, "”", "\"")  // right double quote
	s = strings.ReplaceAll(s, "•", "-")   // bullet
	s = strings.ReplaceAll(s, "…", "...") // ellipsis

	return strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 {
			return r // printable ASCII
		}
		if r >= 0x00A0 && r <= 0x00FF {
			return r // Latin-1 supplement (accented chars, etc.)
		}
		return '?' // replace unsupported chars
	}, s)
}

// truncate clips a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
