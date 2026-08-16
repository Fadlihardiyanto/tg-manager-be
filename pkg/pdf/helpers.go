package pdf

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func FormatCurrency(d decimal.Decimal, code string) string {
	switch strings.ToUpper(code) {
	case "USD":
		return fmt.Sprintf("$ %s", formatDecimal(d, 2))
	default:
		return "Rp " + formatIDR(d)
	}
}

func formatIDR(d decimal.Decimal) string {
	intVal := d.IntPart()
	negative := intVal < 0
	if negative {
		intVal = -intVal
	}

	s := fmt.Sprintf("%d", intVal)
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(ch)
	}

	result := b.String()
	if negative {
		return "-" + result
	}
	return result
}

func formatDecimal(d decimal.Decimal, places int32) string {
	return d.StringFixed(places)
}

func generatedAt(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 {
			return r
		}
		if r >= 0x00A0 && r <= 0x00FF {
			return r
		}
		return '?'
	}, s)
}

func truncate(s string, max int) string {
	// Rune-aware: len() menghitung bytes — slicing byte bisa memotong UTF-8
	// rune (emoji/aksen di nama item) → invalid UTF-8 → korupsi '?'.
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func PaymentTypeLabel(pt string) string {
	m := map[string]string{
		"bank_transfer": "Transfer Bank",
		"gopay":         "GoPay",
		"shopeepay":     "ShopeePay",
		"qris":          "QRIS",
		"credit_card":   "Kartu Kredit",
		"cstore":        "Convenience Store",
		"echannel":      "Mandiri Bill",
	}
	if v, ok := m[pt]; ok {
		return v
	}
	return pt
}

func StatusLabel(s string) string {
	_, _, label := statusColors(s)
	return label
}
