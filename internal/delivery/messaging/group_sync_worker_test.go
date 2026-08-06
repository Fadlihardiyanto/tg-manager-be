package messaging

import (
	"errors"
	"testing"
)

func TestMapGroupInactiveReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "bot is not a member",
			err:  errors.New("get chat members count API call: Bad Request: Forbidden: bot is not a member of the group chat"),
			want: "Bot tidak lagi menjadi member di grup ini",
		},
		{
			name: "forbidden generic",
			err:  errors.New("Forbidden: bot is not allowed to access the group chat"),
			want: "Bot tidak lagi menjadi member di grup ini",
		},
		{
			name: "bot kicked",
			err:  errors.New("bot was kicked from the group"),
			want: "Bot dikeluarkan dari grup",
		},
		{
			name: "chat not found",
			err:  errors.New("Bad Request: chat not found"),
			want: "Grup tidak ditemukan atau sudah dihapus",
		},
		{
			name: "not enough rights",
			err:  errors.New("Bad Request: not enough rights to send text messages to the chat"),
			want: "Bot tidak memiliki izin yang cukup di grup",
		},
		{
			name: "user deactivated",
			err:  errors.New("Forbidden: user is deactivated"),
			want: "Akun bot dinonaktifkan",
		},
		{
			name: "unknown fallback",
			err:  errors.New("some random network error"),
			want: "Gagal sinkronisasi grup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGroupInactiveReason(tt.err)
			if got != tt.want {
				t.Errorf("mapGroupInactiveReason() = %q, want %q", got, tt.want)
			}
		})
	}
}
