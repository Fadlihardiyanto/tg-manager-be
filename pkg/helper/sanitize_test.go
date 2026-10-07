package helper

import "testing"

func TestSanitizeTelegramHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strips p tags, keeps supported formatting",
			in:   `<p>1. Jangan spam <b>oke</b><br>2. Tanya aja</p>`,
			want: `1. Jangan spam <b>oke</b>
2. Tanya aja
`,
		},
		{
			name: "strips div ul li",
			in:   `<div><ul><li>satu</li></ul></div>`,
			want: `satu`,
		},
		{
			name: "strips target/rel attrs from links",
			in:   `<a href="https://x.com" target="_blank" rel="noopener">x</a>`,
			want: `<a href="https://x.com">x</a>`,
		},
		{
			name: "plain text untouched",
			in:   "Halo dunia",
			want: "Halo dunia",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeTelegramHTML(tt.in); got != tt.want {
				t.Fatalf("SanitizeTelegramHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
