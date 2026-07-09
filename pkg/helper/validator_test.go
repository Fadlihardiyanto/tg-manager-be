package helper

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

type testStruct struct {
	Name     string  `validate:"required"`
	Email    string  `validate:"email"`
	Age      int     `validate:"gte=18"`
	Phone    string  `validate:"no_hp"`
	Role     string  `validate:"oneof=admin manager viewer"`
	Website  string  `validate:"url"`
	UUID     string  `validate:"uuid4"`
	Username string  `validate:"alphanum"`
	MinLen   string  `validate:"min=3"`
	MaxLen   string  `validate:"max=10"`
	Len      string  `validate:"len=5"`
	Amount   float64 `validate:"gt=0"`
}

func validStruct() testStruct {
	return testStruct{
		Name:     "Test User",
		Email:    "test@example.com",
		Age:      25,
		Phone:    "081234567890",
		Role:     "admin",
		Website:  "https://example.com",
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Username: "testuser123",
		MinLen:   "hello",
		MaxLen:   "short",
		Len:      "abcde",
		Amount:   100,
	}
}

func TestTranslateError(t *testing.T) {
	v := validator.New()
	_ = v.RegisterValidation("no_hp", func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) >= 10 && len(fl.Field().String()) <= 15
	})
	_ = v.RegisterValidation("nominal", func(fl validator.FieldLevel) bool {
		return fl.Field().Float() > 0
	})

	tests := []struct {
		name    string
		modify  func(*testStruct)
		wantErr map[string]string
	}{
		{
			name: "required field empty",
			modify: func(s *testStruct) {
				s.Name = ""
			},
			wantErr: map[string]string{
				"Name": "Name wajib diisi",
			},
		},
		{
			name: "invalid email",
			modify: func(s *testStruct) {
				s.Email = "not-an-email"
			},
			wantErr: map[string]string{
				"Email": "Email harus berupa alamat email yang valid",
			},
		},
		{
			name: "age below minimum",
			modify: func(s *testStruct) {
				s.Age = 15
			},
			wantErr: map[string]string{
				"Age": "Age harus lebih dari atau sama dengan 18",
			},
		},
		{
			name: "oneof invalid",
			modify: func(s *testStruct) {
				s.Role = "superadmin"
			},
			wantErr: map[string]string{
				"Role": "Role harus salah satu dari: admin, manager, viewer",
			},
		},
		{
			name: "invalid url",
			modify: func(s *testStruct) {
				s.Website = "not-a-url"
			},
			wantErr: map[string]string{
				"Website": "Website harus berupa URL yang valid",
			},
		},
		{
			name: "invalid uuid",
			modify: func(s *testStruct) {
				s.UUID = "not-uuid"
			},
			wantErr: map[string]string{
				"UUID": "UUID harus berupa UUID yang valid",
			},
		},
		{
			name: "alphanumeric",
			modify: func(s *testStruct) {
				s.Username = "hello world!"
			},
			wantErr: map[string]string{
				"Username": "Username hanya boleh berisi huruf dan angka",
			},
		},
		{
			name: "min length",
			modify: func(s *testStruct) {
				s.MinLen = "ab"
			},
			wantErr: map[string]string{
				"MinLen": "MinLen minimal 3 karakter",
			},
		},
		{
			name: "max length exceeded",
			modify: func(s *testStruct) {
				s.MaxLen = "this-is-too-long"
			},
			wantErr: map[string]string{
				"MaxLen": "MaxLen maksimal 10 karakter",
			},
		},
		{
			name: "exact length",
			modify: func(s *testStruct) {
				s.Len = "too-long"
			},
			wantErr: map[string]string{
				"Len": "Len harus tepat 5 karakter",
			},
		},
		{
			name: "gt zero",
			modify: func(s *testStruct) {
				s.Amount = 0
			},
			wantErr: map[string]string{
				"Amount": "Amount harus lebih dari 0",
			},
		},
		{
			name:    "all valid",
			modify:  func(s *testStruct) {},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStruct()
			tt.modify(&s)

			errs := ValidateStruct(v, s)
			if tt.wantErr == nil {
				if errs != nil {
					t.Errorf("expected no errors, got %v", errs)
				}
				return
			}

			if errs == nil {
				t.Fatal("expected errors, got nil")
			}

			for field, expectedMsg := range tt.wantErr {
				gotMsg, ok := errs[field]
				if !ok {
					t.Errorf("missing error for field %q", field)
					continue
				}
				if gotMsg != expectedMsg {
					t.Errorf("field %q: got %q, want %q", field, gotMsg, expectedMsg)
				}
			}
		})
	}
}
