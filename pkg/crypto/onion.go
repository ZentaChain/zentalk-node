package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"

	"github.com/zentalk/protocol/pkg/protocol"
)

var (
	ErrInvalidOnionLayer = errors.New("invalid onion layer")
	ErrInvalidPath       = errors.New("invalid path")
)

// OnionLayer represents a single layer of the onion
type OnionLayer struct {
	NextHop     protocol.Address // Next relay or zero for delivery
	Payload     []byte           // Encrypted inner layer or final message
	PayloadHash protocol.Hash    // BLAKE2b hash of payload
}

// RelayInfo contains relay routing information
type RelayInfo struct {
	Address   protocol.Address
	PublicKey *rsa.PublicKey
}

// BuildOnionLayers builds onion layers for a message using hybrid encryption
// path: ordered list of relay info (first to last)
// recipientAddr: final recipient address
// finalPayload: the actual message to deliver
func BuildOnionLayers(path []*RelayInfo, recipientAddr protocol.Address, finalPayload []byte) ([]byte, error) {
	if len(path) == 0 {
		return nil, ErrInvalidPath
	}

	// Start with the final payload (innermost layer)
	currentPayload := finalPayload

	// Build layers from inside out (reverse order)
	for i := len(path) - 1; i >= 0; i-- {
		// Create layer
		layer := &OnionLayer{
			Payload: currentPayload,
		}

		// Set next hop
		if i == len(path)-1 {
			// Last hop - deliver to recipient
			layer.NextHop = recipientAddr // Use recipient's address
		} else {
			// Not last hop - forward to next relay
			layer.NextHop = path[i+1].Address
		}

		// Calculate payload hash
		hash, err := Hash(currentPayload)
		if err != nil {
			return nil, err
		}
		copy(layer.PayloadHash[:], hash)

		// Encode layer to JSON
		layerJSON, err := json.Marshal(layer)
		if err != nil {
			return nil, err
		}

		// Use hybrid encryption: AES for data, RSA for AES key
		// Generate random AES key
		aesKey, err := GenerateAESKey()
		if err != nil {
			return nil, err
		}

		// Encrypt layer data with AES
		encryptedData, err := AESEncrypt(layerJSON, aesKey)
		if err != nil {
			return nil, err
		}

		// Encrypt AES key with RSA
		encryptedKey, err := RSAEncrypt(aesKey, path[i].PublicKey)
		if err != nil {
			return nil, err
		}

		// Combine: [key length (2 bytes)] + [encrypted AES key] + [encrypted data]
		keyLen := uint16(len(encryptedKey))
		combined := make([]byte, 2+len(encryptedKey)+len(encryptedData))
		combined[0] = byte(keyLen >> 8)
		combined[1] = byte(keyLen)
		copy(combined[2:], encryptedKey)
		copy(combined[2+len(encryptedKey):], encryptedData)

		// This becomes the payload for the next outer layer
		currentPayload = combined
	}

	return currentPayload, nil
}

// DecryptOnionLayer decrypts one layer of the onion using hybrid encryption
func DecryptOnionLayer(encryptedLayer []byte, privateKey *rsa.PrivateKey) (*OnionLayer, error) {
	// Extract key length
	if len(encryptedLayer) < 2 {
		return nil, errors.New("encrypted layer too short")
	}

	keyLen := uint16(encryptedLayer[0])<<8 | uint16(encryptedLayer[1])
	if len(encryptedLayer) < int(2+keyLen) {
		return nil, errors.New("encrypted layer incomplete")
	}

	// Extract encrypted AES key and encrypted data
	encryptedKey := encryptedLayer[2 : 2+keyLen]
	encryptedData := encryptedLayer[2+keyLen:]

	// Decrypt AES key with RSA
	aesKey, err := RSADecrypt(encryptedKey, privateKey)
	if err != nil {
		return nil, err
	}

	// Decrypt data with AES
	decrypted, err := AESDecrypt(encryptedData, aesKey)
	if err != nil {
		return nil, err
	}

	// Parse layer
	var layer OnionLayer
	if err := json.Unmarshal(decrypted, &layer); err != nil {
		return nil, ErrInvalidOnionLayer
	}

	// Verify payload hash
	valid, err := VerifyHash(layer.Payload, layer.PayloadHash[:])
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, errors.New("payload hash mismatch")
	}

	return &layer, nil
}

// AESEncrypt encrypts data with AES-256-GCM
func AESEncrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESDecrypt decrypts data with AES-256-GCM
func AESDecrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GenerateAESKey generates a random AES-256 key
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	_, err := rand.Read(key)
	return key, err
}

// IsDeliveryAddress checks if address is zero (final delivery)
func IsDeliveryAddress(addr protocol.Address) bool {
	zero := protocol.Address{}
	return addr == zero
}
