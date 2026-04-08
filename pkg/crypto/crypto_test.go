package utils

import (
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Errorf("GenerateSalt() error = %v", err)
		return
	}
	if len(salt) != SaltSize {
		t.Errorf("GenerateSalt() length = %d, want %d", len(salt), SaltSize)
	}
	// Check that salt is not all zeros
	allZero := true
	for _, b := range salt {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("GenerateSalt() returned all zeros")
	}
}

func TestGenerateKey(t *testing.T) {
	password := "test-password-123"
	salt := make([]byte, SaltSize)
	for i := range salt {
		salt[i] = byte(i)
	}

	key := GenerateKey(password, salt)
	if len(key) != KeySize {
		t.Errorf("GenerateKey() length = %d, want %d", len(key), KeySize)
	}
	// Same password and salt should produce same key
	key2 := GenerateKey(password, salt)
	if string(key) != string(key2) {
		t.Error("GenerateKey() not deterministic")
	}
	// Different password should produce different key
	key3 := GenerateKey("different-password", salt)
	if string(key) == string(key3) {
		t.Error("GenerateKey() same key for different passwords")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	password := "test-password-123"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}

	key := GenerateKey(password, salt)

	plaintext := []byte("Hello, Kerrigan v2! This is a secret message.")

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Errorf("Encrypt() error = %v", err)
		return
	}
	if len(ciphertext) <= len(plaintext) {
		t.Error("Encrypt() ciphertext not longer than plaintext")
	}

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Errorf("Decrypt() error = %v", err)
		return
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypt() = %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecrypt_DifferentKeys(t *testing.T) {
	password1 := "password-1"
	password2 := "password-2"
	salt, _ := GenerateSalt()

	key1 := GenerateKey(password1, salt)
	key2 := GenerateKey(password2, salt)

	plaintext := []byte("Secret message")

	// Encrypt with key1
	ciphertext, err := Encrypt(plaintext, key1)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Try to decrypt with key2 - should fail
	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Decrypt() should fail with wrong key")
	}
}

func TestHash256(t *testing.T) {
	data := []byte("test data for hashing")

	hash1 := Hash256(data)
	if len(hash1) != 32 {
		t.Errorf("Hash256() length = %d, want 32", len(hash1))
	}

	// Same data should produce same hash
	hash2 := Hash256(data)
	if string(hash1) != string(hash2) {
		t.Error("Hash256() not deterministic")
	}

	// Different data should produce different hash
	differentData := []byte("different data")
	hash3 := Hash256(differentData)
	if string(hash1) == string(hash3) {
		t.Error("Hash256() same hash for different data")
	}
}

func TestHash256Hex(t *testing.T) {
	data := []byte("test data")

	hashHex := Hash256Hex(data)
	if len(hashHex) != 64 {
		t.Errorf("Hash256Hex() length = %d, want 64", len(hashHex))
	}

	// Should be valid hex
	for _, c := range hashHex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash256Hex() contains invalid hex character: %c", c)
		}
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	password := "test-password"
	salt, _ := GenerateSalt()
	key := GenerateKey(password, salt)

	plaintext := []byte{}
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Errorf("Encrypt() error = %v", err)
	}
	if len(ciphertext) == 0 {
		t.Error("Encrypt() returned empty ciphertext for empty plaintext")
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	password := "test-password"
	salt, _ := GenerateSalt()
	key := GenerateKey(password, salt)

	// Too short ciphertext
	shortCiphertext := []byte("too short")
	_, err := Decrypt(shortCiphertext, key)
	if err == nil {
		t.Error("Decrypt() should fail with short ciphertext")
	}
}
