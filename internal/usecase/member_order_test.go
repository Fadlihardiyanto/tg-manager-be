package usecase

import (
	"testing"
)

func TestBuildCustomerName(t *testing.T) {
	tests := []struct {
		name           string
		firstName      string
		lastName       string
		username       string
		telegramUserID int64
		want           string
	}{
		{
			name:           "first and last name",
			firstName:      "Budi",
			lastName:       "Santoso",
			username:       "budi123",
			telegramUserID: 12345,
			want:           "Budi Santoso",
		},
		{
			name:           "first name only",
			firstName:      "Budi",
			lastName:       "",
			username:       "budi123",
			telegramUserID: 12345,
			want:           "Budi",
		},
		{
			name:           "no first name, fallback to username",
			firstName:      "",
			lastName:       "",
			username:       "budi123",
			telegramUserID: 12345,
			want:           "budi123",
		},
		{
			name:           "empty first name with last name only",
			firstName:      "",
			lastName:       "Santoso",
			username:       "budi123",
			telegramUserID: 12345,
			want:           "Santoso",
		},
		{
			name:           "everything empty, fallback to User-ID",
			firstName:      "",
			lastName:       "",
			username:       "",
			telegramUserID: 12345,
			want:           "User-12345",
		},
		{
			name:           "empty everything with zero ID",
			firstName:      "",
			lastName:       "",
			username:       "",
			telegramUserID: 0,
			want:           "User-0",
		},
		{
			name:           "only last name",
			firstName:      "",
			lastName:       "Wijaya",
			username:       "",
			telegramUserID: 999999,
			want:           "Wijaya",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCustomerName(tt.firstName, tt.lastName, tt.username, tt.telegramUserID)
			if got != tt.want {
				t.Errorf("buildCustomerName(%q, %q, %q, %d) = %q, want %q",
					tt.firstName, tt.lastName, tt.username, tt.telegramUserID, got, tt.want)
			}
		})
	}
}
