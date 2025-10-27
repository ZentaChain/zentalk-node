package dht

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpiredEntry     = errors.New("entry expired")
	ErrMissingPublicKey = errors.New("missing public key")
)

// SignedEntry represents a DHT entry with Ed25519 signature
// This prevents unauthorized nodes from storing/poisoning DHT data
type SignedEntry struct {
	Key       NodeID    `json:"key"`        // DHT key
	Value     []byte    `json:"value"`      // Actual data (e.g., relay metadata)
	PublicKey []byte    `json:"public_key"` // Ed25519 public key (32 bytes)
	Signature []byte    `json:"signature"`  // Ed25519 signature (64 bytes)
	Timestamp int64     `json:"timestamp"`  // Unix timestamp
	TTL       int64     `json:"ttl"`        // Time-to-live in seconds
	Nonce     []byte    `json:"nonce"`      // Random nonce for replay protection
}

// SignEntry creates a signed DHT entry
func SignEntry(key NodeID, value []byte, privateKey ed25519.PrivateKey, ttl time.Duration) (*SignedEntry, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(privateKey))
	}

	// Generate random nonce for replay protection
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create entry
	entry := &SignedEntry{
		Key:       key,
		Value:     value,
		PublicKey: privateKey.Public().(ed25519.PublicKey),
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
		Nonce:     nonce,
	}

	// Sign the entry
	if err := entry.Sign(privateKey); err != nil {
		return nil, err
	}

	return entry, nil
}

// Sign signs the entry with the given private key
func (e *SignedEntry) Sign(privateKey ed25519.PrivateKey) error {
	// Create message to sign (all fields except signature)
	message := e.signatureMessage()

	// Sign with Ed25519
	signature := ed25519.Sign(privateKey, message)
	e.Signature = signature

	return nil
}

// Verify verifies the entry's signature
func (e *SignedEntry) Verify() error {
	if len(e.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: %d", len(e.PublicKey))
	}

	if len(e.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size: %d", len(e.Signature))
	}

	// Check expiration
	if e.IsExpired() {
		return ErrExpiredEntry
	}

	// Recreate signed message
	message := e.signatureMessage()

	// Verify signature
	if !ed25519.Verify(e.PublicKey, message, e.Signature) {
		return ErrInvalidSignature
	}

	return nil
}

// signatureMessage creates the message that is signed
// Format: key || value || publicKey || timestamp || ttl || nonce
func (e *SignedEntry) signatureMessage() []byte {
	message := make([]byte, 0, len(e.Key)+len(e.Value)+len(e.PublicKey)+8+8+len(e.Nonce))

	// Append key
	message = append(message, e.Key[:]...)

	// Append value
	message = append(message, e.Value...)

	// Append public key
	message = append(message, e.PublicKey...)

	// Append timestamp (8 bytes)
	timestampBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		timestampBytes[i] = byte(e.Timestamp >> (56 - i*8))
	}
	message = append(message, timestampBytes...)

	// Append TTL (8 bytes)
	ttlBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		ttlBytes[i] = byte(e.TTL >> (56 - i*8))
	}
	message = append(message, ttlBytes...)

	// Append nonce
	message = append(message, e.Nonce...)

	return message
}

// IsExpired checks if the entry has expired
func (e *SignedEntry) IsExpired() bool {
	expiryTime := e.Timestamp + e.TTL
	return time.Now().Unix() > expiryTime
}

// Encode encodes the signed entry to JSON
func (e *SignedEntry) Encode() ([]byte, error) {
	return json.Marshal(e)
}

// DecodeSignedEntry decodes a signed entry from JSON
func DecodeSignedEntry(data []byte) (*SignedEntry, error) {
	var entry SignedEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// VerifyAndExtract verifies the entry and returns the value if valid
func VerifyAndExtract(data []byte) ([]byte, error) {
	entry, err := DecodeSignedEntry(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode entry: %w", err)
	}

	if err := entry.Verify(); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	return entry.Value, nil
}

