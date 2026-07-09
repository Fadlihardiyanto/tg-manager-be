package pdf

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{name: "first is non-empty", vals: []string{"a", "b", "c"}, want: "a"},
		{name: "first is empty", vals: []string{"", "b", "c"}, want: "b"},
		{name: "all empty", vals: []string{"", "", ""}, want: ""},
		{name: "single value", vals: []string{"hello"}, want: "hello"},
		{name: "empty slice", vals: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.vals...)
			if got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.vals, got, tt.want)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{name: "normal ASCII", s: "Hello World!", want: "Hello World!"},
		{name: "Latin-1 extended", s: "caf\u00e9", want: "caf\u00e9"},
		{name: "non-printable replaced", s: "abc\x00def", want: "abc?def"},
		{name: "emoji replaced", s: "hello\u2764world", want: "hello?world"},
		{name: "empty string", s: "", want: ""},
		{name: "newlines and tabs", s: "line1\nline2\tend", want: "line1?line2?end"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize(tt.s)
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{name: "short string", s: "hello", max: 10, want: "hello"},
		{name: "exact fit", s: "hello", max: 5, want: "hello"},
		{name: "truncate with ellipsis", s: "hello world", max: 8, want: "hello..."},
		{name: "max is 3", s: "hello", max: 3, want: "hel"},
		{name: "max is 0", s: "hello", max: 0, want: ""},
		{name: "empty string", s: "", max: 10, want: ""},
		{name: "long text", s: strings.Repeat("a", 100), max: 10, want: "aaaaaaa..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestPaymentTypeLabel(t *testing.T) {
	tests := []struct {
		name string
		pt   string
		want string
	}{
		{name: "bank_transfer", pt: "bank_transfer", want: "Transfer Bank"},
		{name: "gopay", pt: "gopay", want: "GoPay"},
		{name: "shopeepay", pt: "shopeepay", want: "ShopeePay"},
		{name: "qris", pt: "qris", want: "QRIS"},
		{name: "credit_card", pt: "credit_card", want: "Kartu Kredit"},
		{name: "cstore", pt: "cstore", want: "Convenience Store"},
		{name: "echannel", pt: "echannel", want: "Mandiri Bill"},
		{name: "unknown type returns raw", pt: "bca_va", want: "bca_va"},
		{name: "empty string", pt: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaymentTypeLabel(tt.pt)
			if got != tt.want {
				t.Errorf("PaymentTypeLabel(%q) = %q, want %q", tt.pt, got, tt.want)
			}
		})
	}
}

func TestGeneratedAt(t *testing.T) {
	t.Run("non-zero time returned as-is", func(t *testing.T) {
		now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
		got := generatedAt(now)
		if !got.Equal(now) {
			t.Errorf("generatedAt(%v) = %v, want %v", now, got, now)
		}
	})

	t.Run("zero time returns current time", func(t *testing.T) {
		before := time.Now()
		got := generatedAt(time.Time{})
		after := time.Now()
		if got.Before(before) || got.After(after) {
			t.Errorf("generatedAt(zero) = %v, want between %v and %v", got, before, after)
		}
	})
}

func TestFormatIDR(t *testing.T) {
	tests := []struct {
		name string
		d    decimal.Decimal
		want string
	}{
		{name: "simple thousand", d: decimal.NewFromInt(1000), want: "1.000"},
		{name: "ten thousand", d: decimal.NewFromInt(10000), want: "10.000"},
		{name: "hundred thousand", d: decimal.NewFromInt(100000), want: "100.000"},
		{name: "million", d: decimal.NewFromInt(1000000), want: "1.000.000"},
		{name: "zero", d: decimal.NewFromInt(0), want: "0"},
		{name: "small number", d: decimal.NewFromInt(500), want: "500"},
		{name: "negative", d: decimal.NewFromInt(-5000), want: "-5.000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIDR(tt.d)
			if got != tt.want {
				t.Errorf("formatIDR(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatDecimal(t *testing.T) {
	d := decimal.NewFromFloat(12345.6789)
	got := formatDecimal(d, 2)
	want := "12345.68"
	if got != want {
		t.Errorf("formatDecimal(%v, 2) = %q, want %q", d, got, want)
	}
}
