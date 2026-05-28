package helper

import (
	"regexp"
	"strings"
)

func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

var nonDigit = regexp.MustCompile(`\D`)

func NormalizePhone(phone string) string {
	p := strings.TrimSpace(phone)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "+") {
		// keep leading +, remove non-digits after it
		return "+" + nonDigit.ReplaceAllString(p[1:], "")
	}
	// remove non-digits
	p = nonDigit.ReplaceAllString(p, "")
	if strings.HasPrefix(p, "0") {
		// local Indonesian format -> +62...
		return "+62" + p[1:]
	}
	if strings.HasPrefix(p, "62") {
		return "+" + p
	}
	// fallback: assume local and add +62
	return "+62" + p
}
