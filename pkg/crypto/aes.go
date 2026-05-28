package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	encoding_hex "encoding/hex"
	"errors"
	"io"
)

// Encrypt encrypts plain text string into base64 encoded ciphertext using AES-256 (GCM).
// The key must be exactly 32 bytes long.
func Encrypt(plaintext string, key string) (string, error) {
	keyBytes := []byte(key)
	if len(keyBytes) == 64 {
		// Attempt to decode hex string
		if decoded, err := encoding_hex.DecodeString(key); err == nil && len(decoded) == 32 {
			keyBytes = decoded
		}
	}
	if len(keyBytes) != 32 {
		return "", errors.New("crypto: invalid key size, must be exactly 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64 encoded ciphertext back to plain text string using AES-256 (GCM).
// The key must be exactly 32 bytes long.
func Decrypt(ciphertextBase64 string, key string) (string, error) {
	keyBytes := []byte(key)
	if len(keyBytes) == 64 {
		// Attempt to decode hex string
		if decoded, err := encoding_hex.DecodeString(key); err == nil && len(decoded) == 32 {
			keyBytes = decoded
		}
	}
	if len(keyBytes) != 32 {
		return "", errors.New("crypto: invalid key size, must be exactly 32 bytes for AES-256")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("crypto: ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
