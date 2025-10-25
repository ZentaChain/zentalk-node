package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestHash(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string // BLAKE2b-256 hash in hex
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: "0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
		},
		{
			name:     "simple string",
			input:    []byte("hello world"),
			expected: "256c83b297114d201b30179f3f0ef0cace9783622da5974326b436178aeef610",
		},
		{
			name:  "arbitrary data",
			input: []byte("The quick brown fox jumps over the lazy dog"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := Hash(tt.input)
			if err != nil {
				t.Fatalf("Hash() error = %v", err)
			}

			if len(hash) != 32 {
				t.Errorf("Hash() length = %d, want 32", len(hash))
			}

			// For known test vectors, verify exact hash
			if tt.expected != "" {
				got := hex.EncodeToString(hash)
				if got != tt.expected {
					t.Errorf("Hash() = %s, want %s", got, tt.expected)
				}
			}
		})
	}
}

func TestHashString(t *testing.T) {
	input := []byte("test data")

	hashStr, err := HashString(input)
	if err != nil {
		t.Fatalf("HashString() error = %v", err)
	}

	// Should return hex string
	if len(hashStr) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("HashString() length = %d, want 64", len(hashStr))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(hashStr)
	if err != nil {
		t.Errorf("HashString() returned invalid hex: %v", err)
	}

	// Should match Hash() output
	hashBytes, _ := Hash(input)
	expectedStr := hex.EncodeToString(hashBytes)
	if hashStr != expectedStr {
		t.Errorf("HashString() = %s, want %s", hashStr, expectedStr)
	}
}

func TestGenerateNonce(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small nonce", 8},
		{"medium nonce", 16},
		{"large nonce", 32},
		{"very large nonce", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce, err := GenerateNonce(tt.size)
			if err != nil {
				t.Fatalf("GenerateNonce() error = %v", err)
			}

			if len(nonce) != tt.size {
				t.Errorf("GenerateNonce() length = %d, want %d", len(nonce), tt.size)
			}

			// Generate another nonce - should be different (probabilistically)
			nonce2, err := GenerateNonce(tt.size)
			if err != nil {
				t.Fatalf("GenerateNonce() second call error = %v", err)
			}

			if bytes.Equal(nonce, nonce2) {
				t.Error("GenerateNonce() produced identical nonces (collision)")
			}
		})
	}
}

func TestGenerateNonceZeroSize(t *testing.T) {
	nonce, err := GenerateNonce(0)
	if err != nil {
		t.Fatalf("GenerateNonce(0) error = %v", err)
	}

	if len(nonce) != 0 {
		t.Errorf("GenerateNonce(0) length = %d, want 0", len(nonce))
	}
}

func TestVerifyHash(t *testing.T) {
	input := []byte("verify this data")
	correctHash, _ := Hash(input)
	wrongHash := make([]byte, 32)
	copy(wrongHash, correctHash)
	wrongHash[0] ^= 0xFF // Flip bits to make it wrong

	tests := []struct {
		name     string
		data     []byte
		hash     []byte
		expected bool
	}{
		{
			name:     "correct hash",
			data:     input,
			hash:     correctHash,
			expected: true,
		},
		{
			name:     "wrong hash",
			data:     input,
			hash:     wrongHash,
			expected: false,
		},
		{
			name:     "modified data",
			data:     []byte("modified data"),
			hash:     correctHash,
			expected: false,
		},
		{
			name:     "empty hash",
			data:     input,
			hash:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := VerifyHash(tt.data, tt.hash)
			if err != nil {
				t.Fatalf("VerifyHash() error = %v", err)
			}

			if valid != tt.expected {
				t.Errorf("VerifyHash() = %v, want %v", valid, tt.expected)
			}
		})
	}
}

func TestHashConsistency(t *testing.T) {
	// Test that same input always produces same hash
	input := []byte("consistency test")

	hash1, _ := Hash(input)
	hash2, _ := Hash(input)
	hash3, _ := Hash(input)

	if !bytes.Equal(hash1, hash2) {
		t.Error("Hash() not consistent between calls")
	}

	if !bytes.Equal(hash2, hash3) {
		t.Error("Hash() not consistent between calls")
	}
}
