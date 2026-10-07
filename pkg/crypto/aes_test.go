package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// exactly 32 bytes
	key := "this-is-32-byte-key-for-aes-256x"

	tests := []struct {
		name      string
		plaintext string
		key       string
		wantErr   bool
	}{
		{"simple text", "hello world", key, false},
		{"empty string", "", key, false},
		{"unicode", "こんにちは世界", key, false},
		{"json-like", `{"order_id":"ORD-123","amount":50000}`, key, false},
		{"long text", "Lorem ipsum dolor sit amet, consectetur adipiscing elit.", key, false},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?", key, false},
		{"numeric string", "1234567890", key, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.plaintext, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}
			if encrypted == "" {
				t.Fatal("Encrypt() returned empty string")
			}
			if encrypted == tt.plaintext {
				t.Error("encrypted output should not equal plaintext")
			}

			decrypted, err := Decrypt(encrypted, tt.key)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}
			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptDecrypt_HexKey(t *testing.T) {
	// 64 hex chars = 32 bytes when decoded
	hexKey := "a1b2c3d4e5f6081728394a5b6c7d8e9f0a1b2c3d4e5f6081728394a5b6c7d8e9"

	plaintext := "hello with hex key"
	encrypted, err := Encrypt(plaintext, hexKey)
	if err != nil {
		t.Fatalf("Encrypt() with hex key error = %v", err)
	}

	decrypted, err := Decrypt(encrypted, hexKey)
	if err != nil {
		t.Fatalf("Decrypt() with hex key error = %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_InvalidInputs(t *testing.T) {
	key := "this-is-32-byte-key-for-aes-256x"

	tests := []struct {
		name        string
		ciphertext  string
		wantErrCont string
	}{
		{"invalid base64", "!!!not-base64!!!", "illegal base64 data"},
		{"short ciphertext", base64.StdEncoding.EncodeToString([]byte{1, 2}), "ciphertext too short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext, key)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErrCont != "" && !strings.Contains(err.Error(), tt.wantErrCont) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrCont)
			}
		})
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key := "this-is-32-byte-key-for-aes-256x"
	wrongKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	encrypted, err := Encrypt("secret data", key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = Decrypt(encrypted, wrongKey)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

func TestEncrypt_InvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"too short key", "short-key-16-bytes"},                     // 19 bytes
		{"too long key", "this-key-is-way-too-long-at-forty-bytes"}, // 41 bytes
		{"empty key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt("test", tt.key)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
