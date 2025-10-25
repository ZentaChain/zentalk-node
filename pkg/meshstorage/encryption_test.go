package meshstorage

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Test data
	plaintext := []byte("Hello, ZenTalk! This is a secret message that must be encrypted.")

	// Generate random key
	var key EncryptionKey
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Encrypt
	encrypted, err := Encrypt(plaintext, &key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Verify nonce size
	if len(encrypted.Nonce) != NonceSize {
		t.Errorf("Invalid nonce size: expected %d, got %d", NonceSize, len(encrypted.Nonce))
	}

	// Verify ciphertext is different from plaintext
	if bytes.Equal(encrypted.Ciphertext, plaintext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := Decrypt(encrypted, &key)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Verify decrypted matches original
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data doesn't match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	plaintext := []byte("Secret message")

	// Generate correct key
	var correctKey EncryptionKey
	rand.Read(correctKey[:])

	// Encrypt
	encrypted, err := Encrypt(plaintext, &correctKey)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Generate wrong key
	var wrongKey EncryptionKey
	rand.Read(wrongKey[:])

	// Try to decrypt with wrong key
	_, err = Decrypt(encrypted, &wrongKey)
	if err == nil {
		t.Error("Decryption should fail with wrong key")
	}
}

func TestDeriveKeyFromWalletAddress(t *testing.T) {
	walletAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// Derive key
	key1, err := DeriveKeyFromWalletAddress(walletAddr)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Derive same key again
	key2, err := DeriveKeyFromWalletAddress(walletAddr)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Keys should be identical (deterministic)
	if !bytes.Equal(key1[:], key2[:]) {
		t.Error("Derived keys should be identical for same wallet address")
	}

	// Different wallet should produce different key
	differentAddr := "0xabcdef1234567890abcdef1234567890abcdef12"
	key3, err := DeriveKeyFromWalletAddress(differentAddr)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	if bytes.Equal(key1[:], key3[:]) {
		t.Error("Different wallets should produce different keys")
	}
}

func TestDeriveKeyFromSignature(t *testing.T) {
	signature := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"

	// Derive key
	key, err := DeriveKeyFromSignature(signature)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Verify key is 32 bytes
	if len(key) != EncryptionKeySize {
		t.Errorf("Key size should be %d bytes, got %d", EncryptionKeySize, len(key))
	}

	// Test invalid signature
	_, err = DeriveKeyFromSignature("short")
	if err == nil {
		t.Error("Should fail with short signature")
	}
}

func TestEncryptDecryptWithWalletAddress(t *testing.T) {
	walletAddr := "0x1234567890abcdef1234567890abcdef12345678"
	plaintext := []byte("User's encrypted chat history")

	// Derive key from wallet
	key, err := DeriveKeyFromWalletAddress(walletAddr)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Encrypt
	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Simulate user on different device with same wallet
	key2, err := DeriveKeyFromWalletAddress(walletAddr)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Decrypt
	decrypted, err := Decrypt(encrypted, key2)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Verify
	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted data doesn't match original")
	}
}

func TestEncryptDecryptWithPassword(t *testing.T) {
	plaintext := []byte("Password-protected data")
	password := "MySecurePassword123!"

	// Encrypt
	encrypted, err := EncryptWithPassword(plaintext, password)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt with correct password
	decrypted, err := DecryptWithPassword(encrypted, password)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted data doesn't match original")
	}

	// Try with wrong password
	_, err = DecryptWithPassword(encrypted, "WrongPassword")
	if err == nil {
		t.Error("Should fail with wrong password")
	}
}

func TestHashData(t *testing.T) {
	data := []byte("Test data for hashing")

	// Hash data
	hash1 := HashData(data)

	// Verify hash is hex string
	if len(hash1) != 64 { // SHA-256 produces 64 hex characters
		t.Errorf("Hash should be 64 characters, got %d", len(hash1))
	}

	// Same data should produce same hash
	hash2 := HashData(data)
	if hash1 != hash2 {
		t.Error("Same data should produce same hash")
	}

	// Different data should produce different hash
	differentData := []byte("Different test data")
	hash3 := HashData(differentData)
	if hash1 == hash3 {
		t.Error("Different data should produce different hash")
	}
}

func TestVerifyDataHash(t *testing.T) {
	data := []byte("Test data")
	correctHash := HashData(data)

	// Verify correct hash
	if !VerifyDataHash(data, correctHash) {
		t.Error("Should verify correct hash")
	}

	// Verify wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if VerifyDataHash(data, wrongHash) {
		t.Error("Should not verify wrong hash")
	}

	// Modified data
	modifiedData := []byte("Modified data")
	if VerifyDataHash(modifiedData, correctHash) {
		t.Error("Should not verify hash of modified data")
	}
}

func TestEncryptionWithLargeData(t *testing.T) {
	// Test with 1MB of data
	largeData := make([]byte, 1024*1024)
	rand.Read(largeData)

	var key EncryptionKey
	rand.Read(key[:])

	// Encrypt
	encrypted, err := Encrypt(largeData, &key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt
	decrypted, err := Decrypt(encrypted, &key)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Verify
	if !bytes.Equal(decrypted, largeData) {
		t.Error("Decrypted data doesn't match original large data")
	}
}

func TestNonceUniqueness(t *testing.T) {
	plaintext := []byte("Test message")
	var key EncryptionKey
	rand.Read(key[:])

	// Encrypt same message multiple times
	encrypted1, _ := Encrypt(plaintext, &key)
	encrypted2, _ := Encrypt(plaintext, &key)

	// Nonces should be different
	if bytes.Equal(encrypted1.Nonce, encrypted2.Nonce) {
		t.Error("Nonces should be unique for each encryption")
	}

	// Ciphertexts should be different (due to different nonces)
	if bytes.Equal(encrypted1.Ciphertext, encrypted2.Ciphertext) {
		t.Error("Ciphertexts should be different (semantic security)")
	}

	// But both should decrypt to same plaintext
	decrypted1, _ := Decrypt(encrypted1, &key)
	decrypted2, _ := Decrypt(encrypted2, &key)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Both encryptions should decrypt to original plaintext")
	}
}

// Benchmark encryption performance
func BenchmarkEncrypt(b *testing.B) {
	data := make([]byte, 1024) // 1KB
	rand.Read(data)

	var key EncryptionKey
	rand.Read(key[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Encrypt(data, &key)
	}
}

// Benchmark decryption performance
func BenchmarkDecrypt(b *testing.B) {
	data := make([]byte, 1024) // 1KB
	rand.Read(data)

	var key EncryptionKey
	rand.Read(key[:])

	encrypted, _ := Encrypt(data, &key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decrypt(encrypted, &key)
	}
}
