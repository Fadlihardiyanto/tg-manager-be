package s3

import "testing"

func TestExtractS3Key(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "custom domain url",
			url:  "https://storage.urator.com/tenant_uploads/3e8eb876-47d3-48d0-af83-0695803ab5aa/60bfd8fd_1787065090.pdf",
			want: "tenant_uploads/3e8eb876-47d3-48d0-af83-0695803ab5aa/60bfd8fd_1787065090.pdf",
		},
		{
			name: "r2 endpoint url with bucket prefix",
			url:  "https://6fd55011063c4756bc4bc2ed5e45f91d.r2.cloudflarestorage.com/tg-manager-storage/tenant_uploads/a/b.pdf",
			want: "tenant_uploads/a/b.pdf",
		},
		{
			name: "empty",
			url:  "",
			want: "",
		},
		{
			name: "no tenant_uploads marker",
			url:  "https://example.com/some/other/path.pdf",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractS3Key(tt.url); got != tt.want {
				t.Errorf("ExtractS3Key(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
