// Package meshstorage provides client-side encryption for distributed storage
package meshstorage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// AES-256 requires 32-byte keys
	EncryptionKeySize = 32

	// AES-GCM nonce size (96 bits / 12 bytes is standard)
	NonceSize = 12

	// PBKDF2 iterations (100,000 is recommended minimum)
	PBKDF2Iterations = 100000

	// Salt for key derivation (should be unique per user)
	DerivationSalt = "ZenTalk-Mesh-Storage-v1"
)

// EncryptionKey represents a 256-bit encryption key
type EncryptionKey [EncryptionKeySize]byte

// EncryptedData represents encrypted data with its nonce
type EncryptedData struct {
	Nonce      []byte `json:"nonce"`      // Random nonce (12 bytes)
	Ciphertext []byte `json:"ciphertext"` // Encrypted data with auth tag
}

// DeriveKeyFromSignature derives an encryption key from a user's wallet signature
// This allows users to encrypt/decrypt their data using their wallet
func DeriveKeyFromSignature(signature string) (*EncryptionKey, error) {
	// Validate signature format
	if len(signature) < 10 {
		return nil, fmt.Errorf("invalid signature: too short")
	}

	// Derive key using PBKDF2 with SHA-256
	// This makes brute-force attacks computationally expensive
	derivedKey := pbkdf2.Key(
		[]byte(signature),           // Password (user's signature)
		[]byte(DerivationSalt),      // Salt (constant per application)
		PBKDF2Iterations,            // Iterations
		EncryptionKeySize,           // Key length (32 bytes for AES-256)
		sha256.New,                  // Hash function
	)

	var key EncryptionKey
	copy(key[:], derivedKey)

	return &key, nil
}

// DeriveKeyFromWalletAddress derives a deterministic key from user's wallet address
// Alternative to signature-based derivation (less secure but doesn't require signing)
func DeriveKeyFromWalletAddress(walletAddress string) (*EncryptionKey, error) {
	// Validate Ethereum address format
	if len(walletAddress) != 42 || walletAddress[:2] != "0x" {
		return nil, fmt.Errorf("invalid wallet address format")
	}

	// Normalize to lowercase
	normalizedAddr := walletAddress[2:] // Remove 0x prefix

	// Derive key using PBKDF2
	derivedKey := pbkdf2.Key(
		[]byte(normalizedAddr),
		[]byte(DerivationSalt),
		PBKDF2Iterations,
		EncryptionKeySize,
		sha256.New,
	)

	var key EncryptionKey
	copy(key[:], derivedKey)

	return &key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
// GCM (Galois/Counter Mode) provides authenticated encryption
func Encrypt(plaintext []byte, key *EncryptionKey) (*EncryptedData, error) {
	// Create AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	// GCM.Seal appends the auth tag to the ciphertext
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func Decrypt(encrypted *EncryptedData, key *EncryptionKey) ([]byte, error) {
	// Validate input
	if len(encrypted.Nonce) != NonceSize {
		return nil, fmt.Errorf("invalid nonce size: expected %d, got %d", NonceSize, len(encrypted.Nonce))
	}

	// Create AES cipher
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, encrypted.Nonce, encrypted.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong key or corrupted data): %w", err)
	}

	return plaintext, nil
}

// HashData creates a SHA-256 hash of data for integrity checking
func HashData(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyDataHash verifies that data matches the expected hash
func VerifyDataHash(data []byte, expectedHash string) bool {
	actualHash := HashData(data)
	return actualHash == expectedHash
}

// EncryptWithPassword encrypts data using a password instead of a key
// Useful for additional user-provided encryption
func EncryptWithPassword(plaintext []byte, password string) (*EncryptedData, error) {
	// Derive key from password
	derivedKey := pbkdf2.Key(
		[]byte(password),
		[]byte(DerivationSalt),
		PBKDF2Iterations,
		EncryptionKeySize,
		sha256.New,
	)

	var key EncryptionKey
	copy(key[:], derivedKey)

	return Encrypt(plaintext, &key)
}

// DecryptWithPassword decrypts data using a password
func DecryptWithPassword(encrypted *EncryptedData, password string) ([]byte, error) {
	// Derive key from password
	derivedKey := pbkdf2.Key(
		[]byte(password),
		[]byte(DerivationSalt),
		PBKDF2Iterations,
		EncryptionKeySize,
		sha256.New,
	)

	var key EncryptionKey
	copy(key[:], derivedKey)

	return Decrypt(encrypted, &key)
}
