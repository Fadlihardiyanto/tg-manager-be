package helper

import (
	"crypto/sha256"
	"encoding/hex"
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

// HashIdentifier returns a truncated sha256 hex of a PII value (email, phone)
// for correlation in logs without exposing the raw value.
func HashIdentifier(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

var (
	pBrTag       = regexp.MustCompile(`<br\s*/?>`)
	pOpenP       = regexp.MustCompile(`<p[^>]*>`)
	pCloseP      = regexp.MustCompile(`</p>`)
	pLinkAttrs   = regexp.MustCompile(`\s+(target|rel)="[^"]*"`)
	pUnsupported = regexp.MustCompile(`</?(div|span|ul|ol|li|h[1-6]|font|table|tr|td|th|tbody|thead|section|article|header|footer|figure|figcaption|blockquote|hr|img|video|audio|iframe|script|style)[^>]*>`)
)

// SanitizeTelegramHTML strips tags that Telegram's HTML parse mode does not
// support (<p>, <div>, <ul>, ...) and converts line-break tags to newlines,
// so rich-text input from admins cannot make the Bot API reject the message.
func SanitizeTelegramHTML(text string) string {
	text = pOpenP.ReplaceAllString(text, "")
	text = pCloseP.ReplaceAllString(text, "\n")
	text = pBrTag.ReplaceAllString(text, "\n")
	text = pUnsupported.ReplaceAllString(text, "")
	text = pLinkAttrs.ReplaceAllString(text, "")
	return text
}
