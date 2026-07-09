package helper

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"local indonesian", "08123456789", "+628123456789"},
		{"already international", "+628123456789", "+628123456789"},
		{"62 prefix without +", "628123456789", "+628123456789"},
		{"with dashes", "0812-3456-789", "+628123456789"},
		{"with spaces", "0812 3456 789", "+628123456789"},
		{"with parentheses", "(0812) 3456-789", "+628123456789"},
		{"with leading plus and spaces", "+62 812 3456 789", "+628123456789"},
		{"with leading plus and dashes", "+62-812-3456-789", "+628123456789"},
		{"non-digit characters mixed", "0812-abc-3456-def-789", "+628123456789"},
		{"short number", "081", "+6281"},
		{"62 prefix with non-digits", "62-812-3456-789", "+628123456789"},
		{"empty string", "", ""},
		{"whitespace only", "  ", ""},
		{"leading/trailing spaces", "  08123456789  ", "+628123456789"},
		{"without prefix 0 or 62, assume local", "8123456789", "+628123456789"},
		{"with plus and non-digits", "+62  8123  4567  89", "+628123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePhone(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"case insensitive match", "Hello World", "world", true},
		{"exact match", "Hello", "Hello", true},
		{"no match", "Hello", "world", false},
		{"empty substring", "Hello", "", true},
		{"empty both", "", "", true},
		{"case insensitive uppercase", "hello", "HELLO", true},
		{"partial match", "This is a long string", "long", true},
		{"substring not found", "This is a long string", "short", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
