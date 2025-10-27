package dht

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestSignEntry(t *testing.T) {
	// Generate test key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create test data
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("test relay metadata")
	ttl := 1 * time.Hour

	// Sign entry
	entry, err := SignEntry(key, value, privateKey, ttl)
	if err != nil {
		t.Fatalf("SignEntry failed: %v", err)
	}

	// Verify signature was created
	if len(entry.Signature) != ed25519.SignatureSize {
		t.Errorf("Invalid signature size: got %d, want %d", len(entry.Signature), ed25519.SignatureSize)
	}

	// Verify public key matches
	if len(entry.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("Invalid public key size: got %d, want %d", len(entry.PublicKey), ed25519.PublicKeySize)
	}

	// Verify public key is correct
	for i := 0; i < len(publicKey); i++ {
		if entry.PublicKey[i] != publicKey[i] {
			t.Error("Public key mismatch")
			break
		}
	}

	t.Logf("✅ Entry signed successfully: signature=%d bytes, nonce=%d bytes", len(entry.Signature), len(entry.Nonce))
}

func TestVerifyEntry(t *testing.T) {
	// Generate test key pair
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create and sign entry
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("test data")
	ttl := 1 * time.Hour

	entry, err := SignEntry(key, value, privateKey, ttl)
	if err != nil {
		t.Fatalf("SignEntry failed: %v", err)
	}

	// Verify signature
	err = entry.Verify()
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}

	t.Logf("✅ Signature verified successfully")
}

func TestVerifyInvalidSignature(t *testing.T) {
	// Generate two different key pairs
	_, privateKey1, _ := ed25519.GenerateKey(nil)
	publicKey2, _, _ := ed25519.GenerateKey(nil)

	// Sign with privateKey1
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("test data")
	ttl := 1 * time.Hour

	entry, _ := SignEntry(key, value, privateKey1, ttl)

	// Replace public key with different one (tamper)
	entry.PublicKey = publicKey2

	// Verification should fail
	err := entry.Verify()
	if err == nil {
		t.Error("Expected verification to fail with tampered public key")
	} else {
		t.Logf("✅ Correctly rejected tampered signature: %v", err)
	}
}

func TestExpiredEntry(t *testing.T) {
	// Generate key pair
	_, privateKey, _ := ed25519.GenerateKey(nil)

	// Create entry
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("test data")
	ttl := 1 * time.Hour

	entry, _ := SignEntry(key, value, privateKey, ttl)

	// Manually set timestamp to 2 hours ago (expired)
	entry.Timestamp = time.Now().Unix() - 2*3600

	// Check if expired
	if !entry.IsExpired() {
		t.Error("Entry should be expired")
	}

	// Verification should fail due to expiration
	err := entry.Verify()
	if err != ErrExpiredEntry {
		t.Errorf("Expected ErrExpiredEntry, got: %v", err)
	} else {
		t.Logf("✅ Correctly detected expired entry")
	}
}

func TestVerifyAndExtract(t *testing.T) {
	// Generate key pair
	_, privateKey, _ := ed25519.GenerateKey(nil)

	// Create and sign entry
	key := NodeID{1, 2, 3, 4, 5}
	originalValue := []byte("test relay metadata")
	ttl := 1 * time.Hour

	entry, _ := SignEntry(key, originalValue, privateKey, ttl)

	// Encode entry
	encoded, err := entry.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify and extract
	extractedValue, err := VerifyAndExtract(encoded)
	if err != nil {
		t.Errorf("VerifyAndExtract failed: %v", err)
	}

	// Compare values
	if string(extractedValue) != string(originalValue) {
		t.Errorf("Value mismatch: got %s, want %s", extractedValue, originalValue)
	}

	t.Logf("✅ Successfully verified and extracted: %s", extractedValue)
}

func TestTamperedValue(t *testing.T) {
	// Generate key pair
	_, privateKey, _ := ed25519.GenerateKey(nil)

	// Create and sign entry
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("original value")
	ttl := 1 * time.Hour

	entry, _ := SignEntry(key, value, privateKey, ttl)

	// Tamper with value
	entry.Value = []byte("tampered value")

	// Verification should fail
	err := entry.Verify()
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature for tampered value, got: %v", err)
	} else {
		t.Logf("✅ Correctly detected tampered value")
	}
}

func BenchmarkSignEntry(b *testing.B) {
	_, privateKey, _ := ed25519.GenerateKey(nil)
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("benchmark data")
	ttl := 1 * time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SignEntry(key, value, privateKey, ttl)
	}
}

func BenchmarkVerifyEntry(b *testing.B) {
	_, privateKey, _ := ed25519.GenerateKey(nil)
	key := NodeID{1, 2, 3, 4, 5}
	value := []byte("benchmark data")
	ttl := 1 * time.Hour

	entry, _ := SignEntry(key, value, privateKey, ttl)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = entry.Verify()
	}
}
