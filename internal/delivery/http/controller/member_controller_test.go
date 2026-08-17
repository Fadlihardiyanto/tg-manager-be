package controller

import (
	"testing"
	"time"
)

func TestParseTimeRangeMs(t *testing.T) {
	// start == end → di-expand jadi satu hari penuh (24 jam)
	ms := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC).UnixMilli()
	val := formatMs(ms) + "," + formatMs(ms)
	start, end, err := parseTimeRangeMs(val)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start == nil || end == nil {
		t.Fatal("expected non-nil start/end")
	}
	diff := end.Sub(*start)
	if diff.Hours() < 23 || diff.Hours() > 25 {
		t.Fatalf("expected ~1 day range for equal start/end, got %v", diff)
	}
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		t.Fatalf("expected start at day boundary, got %v", start)
	}

	// start != end → tidak diubah
	ms2 := ms + 3600_000
	start2, end2, err := parseTimeRangeMs(formatMs(ms) + "," + formatMs(ms2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start2 == nil || end2 == nil {
		t.Fatal("expected non-nil start/end")
	}
	if !start2.Equal(time.UnixMilli(ms)) || !end2.Equal(time.UnixMilli(ms2)) {
		t.Fatal("start != end must not be modified")
	}

	// format salah → error
	if _, _, err := parseTimeRangeMs("123"); err == nil {
		t.Fatal("expected error for malformed input")
	}
	if _, _, err := parseTimeRangeMs("abc,def"); err == nil {
		t.Fatal("expected error for non-numeric input")
	}
	if _, _, err := parseTimeRangeMs(""); err != nil {
		t.Fatal("empty input must not error")
	}
}

func formatMs(ms int64) string {
	return itoa(ms)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
