package crypto

import (
	"bytes"
	"crypto/rsa"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRSAKeyPair(t *testing.T) {
	privateKey, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair() error = %v", err)
	}

	if privateKey == nil {
		t.Fatal("GenerateRSAKeyPair() returned nil key")
	}

	// Verify key size is 4096 bits
	keySize := privateKey.N.BitLen()
	if keySize != 4096 {
		t.Errorf("GenerateRSAKeyPair() key size = %d, want 4096", keySize)
	}

	// Verify public key is embedded
	if privateKey.PublicKey.N == nil {
		t.Error("GenerateRSAKeyPair() public key not initialized")
	}
}

func TestExportImportPrivateKeyPEM(t *testing.T) {
	// Generate a key
	originalKey, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair() error = %v", err)
	}

	// Export to PEM
	pemData, err := ExportPrivateKeyPEM(originalKey)
	if err != nil {
		t.Fatalf("ExportPrivateKeyPEM() error = %v", err)
	}

	// Verify PEM format
	pemStr := string(pemData)
	if !strings.HasPrefix(pemStr, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Error("ExportPrivateKeyPEM() does not start with PEM header")
	}
	if !strings.HasSuffix(strings.TrimSpace(pemStr), "-----END RSA PRIVATE KEY-----") {
		t.Error("ExportPrivateKeyPEM() does not end with PEM footer")
	}

	// Import back
	importedKey, err := ImportPrivateKeyPEM(pemData)
	if err != nil {
		t.Fatalf("ImportPrivateKeyPEM() error = %v", err)
	}

	// Verify keys match
	if originalKey.N.Cmp(importedKey.N) != 0 {
		t.Error("ImportPrivateKeyPEM() key mismatch: modulus differs")
	}
	if originalKey.E != importedKey.E {
		t.Error("ImportPrivateKeyPEM() key mismatch: exponent differs")
	}
}

func TestExportImportPublicKeyPEM(t *testing.T) {
	// Generate a key
	privateKey, _ := GenerateRSAKeyPair()
	originalPublicKey := &privateKey.PublicKey

	// Export to PEM
	pemData, err := ExportPublicKeyPEM(originalPublicKey)
	if err != nil {
		t.Fatalf("ExportPublicKeyPEM() error = %v", err)
	}

	// Verify PEM format
	pemStr := string(pemData)
	if !strings.HasPrefix(pemStr, "-----BEGIN PUBLIC KEY-----") {
		t.Error("ExportPublicKeyPEM() does not start with PEM header")
	}
	if !strings.HasSuffix(strings.TrimSpace(pemStr), "-----END PUBLIC KEY-----") {
		t.Error("ExportPublicKeyPEM() does not end with PEM footer")
	}

	// Import back
	importedPublicKey, err := ImportPublicKeyPEM(pemData)
	if err != nil {
		t.Fatalf("ImportPublicKeyPEM() error = %v", err)
	}

	// Verify keys match
	if originalPublicKey.N.Cmp(importedPublicKey.N) != 0 {
		t.Error("ImportPublicKeyPEM() key mismatch: modulus differs")
	}
	if originalPublicKey.E != importedPublicKey.E {
		t.Error("ImportPublicKeyPEM() key mismatch: exponent differs")
	}
}

func TestImportPrivateKeyPEMInvalid(t *testing.T) {
	tests := []struct {
		name    string
		pemData []byte
	}{
		{
			name:    "empty data",
			pemData: []byte{},
		},
		{
			name:    "invalid PEM",
			pemData: []byte("not a PEM file"),
		},
		{
			name:    "malformed PEM",
			pemData: []byte("-----BEGIN RSA PRIVATE KEY-----\ninvalid base64\n-----END RSA PRIVATE KEY-----"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ImportPrivateKeyPEM(tt.pemData)
			if err == nil {
				t.Error("ImportPrivateKeyPEM() expected error, got nil")
			}
		})
	}
}

func TestSaveLoadKeyFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	keyFile := filepath.Join(tempDir, "test_key.pem")

	// Generate and export a key
	privateKey, _ := GenerateRSAKeyPair()
	pemData, _ := ExportPrivateKeyPEM(privateKey)

	// Save to file
	err := SaveKeyToFile(keyFile, pemData)
	if err != nil {
		t.Fatalf("SaveKeyToFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Fatal("SaveKeyToFile() did not create file")
	}

	// Load from file
	loadedData, err := LoadKeyFromFile(keyFile)
	if err != nil {
		t.Fatalf("LoadKeyFromFile() error = %v", err)
	}

	// Verify data matches
	if !bytes.Equal(pemData, loadedData) {
		t.Error("LoadKeyFromFile() data mismatch")
	}
}

func TestLoadKeyFromFileNotFound(t *testing.T) {
	_, err := LoadKeyFromFile("/nonexistent/path/key.pem")
	if err == nil {
		t.Error("LoadKeyFromFile() expected error for nonexistent file")
	}
}

func TestRSAEncryptDecrypt(t *testing.T) {
	// Generate key pair
	privateKey, _ := GenerateRSAKeyPair()
	publicKey := &privateKey.PublicKey

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "short message",
			plaintext: []byte("Hello, ZenTalk!"),
		},
		{
			name:      "empty message",
			plaintext: []byte{},
		},
		{
			name:      "binary data",
			plaintext: []byte{0x00, 0xFF, 0x42, 0xAB, 0xCD},
		},
		{
			name:      "max size message",
			plaintext: make([]byte, 446), // Max for 4096-bit RSA with OAEP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := RSAEncrypt(tt.plaintext, publicKey)
			if err != nil {
				t.Fatalf("RSAEncrypt() error = %v", err)
			}

			// Ciphertext should be different from plaintext
			if len(tt.plaintext) > 0 && bytes.Equal(ciphertext, tt.plaintext) {
				t.Error("RSAEncrypt() ciphertext equals plaintext")
			}

			// Decrypt
			decrypted, err := RSADecrypt(ciphertext, privateKey)
			if err != nil {
				t.Fatalf("RSADecrypt() error = %v", err)
			}

			// Verify roundtrip
			if !bytes.Equal(tt.plaintext, decrypted) {
				t.Errorf("RSADecrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestRSAEncryptTooLarge(t *testing.T) {
	privateKey, _ := GenerateRSAKeyPair()
	publicKey := &privateKey.PublicKey

	// Try to encrypt data that's too large for RSA
	tooLarge := make([]byte, 1000) // Too large for 4096-bit RSA with OAEP

	_, err := RSAEncrypt(tooLarge, publicKey)
	if err == nil {
		t.Error("RSAEncrypt() expected error for oversized data")
	}
}

func TestRSADecryptInvalid(t *testing.T) {
	privateKey, _ := GenerateRSAKeyPair()

	// Try to decrypt invalid ciphertext
	invalidCiphertext := []byte("not valid ciphertext")

	_, err := RSADecrypt(invalidCiphertext, privateKey)
	if err == nil {
		t.Error("RSADecrypt() expected error for invalid ciphertext")
	}
}

func TestSignDataVerifySignature(t *testing.T) {
	// Generate key pair
	privateKey, _ := GenerateRSAKeyPair()
	publicKey := &privateKey.PublicKey

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "simple message",
			data: []byte("Sign this message"),
		},
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "large data",
			data: bytes.Repeat([]byte("A"), 10000),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign the data
			signature, err := SignData(tt.data, privateKey)
			if err != nil {
				t.Fatalf("SignData() error = %v", err)
			}

			if len(signature) == 0 {
				t.Fatal("SignData() returned empty signature")
			}

			// Verify signature
			err = VerifySignature(tt.data, signature, publicKey)
			if err != nil {
				t.Fatalf("VerifySignature() error = %v", err)
			}
		})
	}
}

func TestVerifySignatureInvalid(t *testing.T) {
	privateKey, _ := GenerateRSAKeyPair()
	publicKey := &privateKey.PublicKey
	data := []byte("original data")

	// Sign the data
	signature, _ := SignData(data, privateKey)

	tests := []struct {
		name      string
		data      []byte
		signature []byte
		publicKey *rsa.PublicKey
	}{
		{
			name:      "modified data",
			data:      []byte("modified data"),
			signature: signature,
			publicKey: publicKey,
		},
		{
			name:      "modified signature",
			data:      data,
			signature: append([]byte{}, signature...),
			publicKey: publicKey,
		},
		{
			name:      "empty signature",
			data:      data,
			signature: []byte{},
			publicKey: publicKey,
		},
		{
			name:      "wrong key",
			data:      data,
			signature: signature,
			publicKey: func() *rsa.PublicKey { k, _ := GenerateRSAKeyPair(); return &k.PublicKey }(),
		},
	}

	// Corrupt one test signature
	tests[1].signature[0] ^= 0xFF

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySignature(tt.data, tt.signature, tt.publicKey)
			if err == nil {
				t.Error("VerifySignature() expected error for invalid signature")
			}
		})
	}
}

func TestSignatureUniqueness(t *testing.T) {
	privateKey, _ := GenerateRSAKeyPair()
	data := []byte("test message")

	// Sign the same data twice
	sig1, _ := SignData(data, privateKey)
	sig2, _ := SignData(data, privateKey)

	// Signatures should be identical for deterministic signing
	if !bytes.Equal(sig1, sig2) {
		t.Error("SignData() produced different signatures for same data")
	}
}

func TestKeyPairConsistency(t *testing.T) {
	// Generate multiple key pairs
	key1, _ := GenerateRSAKeyPair()
	key2, _ := GenerateRSAKeyPair()

	// They should be different
	if key1.N.Cmp(key2.N) == 0 {
		t.Error("GenerateRSAKeyPair() generated identical keys")
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	// Generate key
	originalPriv, _ := GenerateRSAKeyPair()
	originalPub := &originalPriv.PublicKey

	// Export both keys
	privPEM, _ := ExportPrivateKeyPEM(originalPriv)
	pubPEM, _ := ExportPublicKeyPEM(originalPub)

	// Import both keys
	importedPriv, _ := ImportPrivateKeyPEM(privPEM)
	importedPub, _ := ImportPublicKeyPEM(pubPEM)

	// Test encryption/decryption with imported keys
	plaintext := []byte("roundtrip test")
	ciphertext, _ := RSAEncrypt(plaintext, importedPub)
	decrypted, _ := RSADecrypt(ciphertext, importedPriv)

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Export/Import roundtrip failed: encryption/decryption mismatch")
	}

	// Test signing/verification with imported keys
	signature, _ := SignData(plaintext, importedPriv)
	err := VerifySignature(plaintext, signature, importedPub)
	if err != nil {
		t.Error("Export/Import roundtrip failed: signature verification failed")
	}
}
