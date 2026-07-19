package crypto

import (
	"testing"
)

func TestECDHFullRoundTrip(t *testing.T) {
	alicePriv, alicePub, err := ECDHGenerateKeyPair()
	if err != nil {
		t.Fatalf("ECDHGenerateKeyPair() error = %v", err)
	}

	bobPriv, bobPub, err := ECDHGenerateKeyPair()
	if err != nil {
		t.Fatalf("ECDHGenerateKeyPair() error = %v", err)
	}

	aliceShared, err := ECDHComputeSharedSecret(alicePriv, bobPub)
	if err != nil {
		t.Fatalf("alice ECDHComputeSharedSecret() error = %v", err)
	}

	bobShared, err := ECDHComputeSharedSecret(bobPriv, alicePub)
	if err != nil {
		t.Fatalf("bob ECDHComputeSharedSecret() error = %v", err)
	}

	if string(aliceShared) != string(bobShared) {
		t.Fatal("shared secrets do not match")
	}

	aliceKey, err := ECDHDeriveKey(aliceShared, []byte("session-123"))
	if err != nil {
		t.Fatalf("ECDHDeriveKey() error = %v", err)
	}
	bobKey, err := ECDHDeriveKey(bobShared, []byte("session-123"))
	if err != nil {
		t.Fatalf("ECDHDeriveKey() error = %v", err)
	}

	tests := []string{
		"Mid-server-DEMO-key123",
		"hello world",
		`{"key":"value","id":42}`,
		"",
	}

	for _, plain := range tests {
		encrypted, err := ECDHEncryptPayload(plain, aliceKey)
		if err != nil {
			t.Errorf("ECDHEncryptPayload(%q) error = %v", plain, err)
			continue
		}
		if encrypted == plain {
			t.Errorf("encrypted payload equals plaintext for %q", plain)
		}

		decrypted, err := ECDHDecryptPayload(encrypted, bobKey)
		if err != nil {
			t.Errorf("ECDHDecryptPayload(%q) error = %v", plain, err)
			continue
		}
		if decrypted != plain {
			t.Errorf("decrypted = %q, want %q", decrypted, plain)
		}
	}
}

func TestECDHDecrypt_InvalidInputs(t *testing.T) {
	_, key, _ := ECDHGenerateKeyPair()
	derived, _ := ECDHDeriveKey(key.Bytes(), []byte("test"))

	if _, err := ECDHDecryptPayload("!!!not-base64!!!", derived); err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
	if _, err := ECDHDecryptPayload("dG9vLXNob3J0", derived); err == nil {
		t.Error("expected error for short ciphertext, got nil")
	}
}

func TestECDHEncodeDecodeKeys(t *testing.T) {
	priv, pub, err := ECDHGenerateKeyPair()
	if err != nil {
		t.Fatalf("ECDHGenerateKeyPair() error = %v", err)
	}

	pubEncoded := ECDHEncodePublicKey(pub)
	pubDecoded, err := ECDHDecodePublicKey(pubEncoded)
	if err != nil {
		t.Fatalf("ECDHDecodePublicKey() error = %v", err)
	}
	if !pub.Equal(pubDecoded) {
		t.Fatal("decoded public key does not match original")
	}

	privEncoded := ECDHEncodePrivateKey(priv)
	privDecoded, err := ECDHDecodePrivateKey(privEncoded)
	if err != nil {
		t.Fatalf("ECDHDecodePrivateKey() error = %v", err)
	}
	if !priv.Equal(privDecoded) {
		t.Fatal("decoded private key does not match original")
	}
}

func TestECDHDifferentInfo_ProducesDifferentKeys(t *testing.T) {
	priv, pub, _ := ECDHGenerateKeyPair()
	shared, _ := ECDHComputeSharedSecret(priv, pub)

	key1, _ := ECDHDeriveKey(shared, []byte("session-1"))
	key2, _ := ECDHDeriveKey(shared, []byte("session-2"))

	if string(key1) == string(key2) {
		t.Error("derived keys with different info should not be equal")
	}
}

func TestECDHDecodePublicKey_Invalid(t *testing.T) {
	if _, err := ECDHDecodePublicKey("!!invalid!!"); err == nil {
		t.Error("expected error for invalid public key encoding, got nil")
	}
}

func TestECDHDecodePrivateKey_Invalid(t *testing.T) {
	if _, err := ECDHDecodePrivateKey("!!invalid!!"); err == nil {
		t.Error("expected error for invalid private key encoding, got nil")
	}
}
