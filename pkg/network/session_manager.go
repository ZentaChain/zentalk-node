package network

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"log"

	"github.com/zentalk/protocol/pkg/protocol"
)

// AESEncryptGCM encrypts plaintext using AES-256-GCM
func AESEncryptGCM(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESDecryptGCM decrypts ciphertext using AES-256-GCM
func AESDecryptGCM(ciphertext []byte, key []byte) ([]byte, error) {
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
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// InitializeRatchetSession initializes a ratchet session as responder (Bob's side)
// This is called when receiving an initial X3DH message from a sender
func (c *Client) InitializeRatchetSession(from protocol.Address, initialMsg *protocol.InitialMessage) error {
	if c.x3dhIdentity == nil || c.signedPreKey == nil {
		return errors.New("X3DH not initialized")
	}

	// Check if session already exists
	if _, exists := c.ratchetSessions[from]; exists {
		log.Printf("⚠️  Ratchet session with %x already exists", from[:8])
		return nil
	}

	// Perform X3DH as responder
	sharedSecret, err := protocol.X3DHResponder(
		c.x3dhIdentity,
		c.signedPreKey,
		c.oneTimePreKeys,
		initialMsg,
	)
	if err != nil {
		return fmt.Errorf("X3DH responder failed: %w", err)
	}

	log.Printf("✅ X3DH completed as responder: SharedSecret=%x...", sharedSecret[:8])

	// Initialize ratchet session as receiver with signed prekey
	// Bob uses his signed prekey because Alice used Bob's signed prekey public as the remote DH key
	session := protocol.NewRatchetStateReceiver(
		sharedSecret,
		c.signedPreKey.PrivateKey,
		c.signedPreKey.PublicKey,
		c.Address,
		from,
	)

	// Set the remote DH public key to Alice's ephemeral key from the initial message
	session.DHReceivingPublic = initialMsg.EphemeralKey

	// Perform initial DH to derive receiving chain key
	// This matches the initial DH that Alice performed on her side
	// Bob computes: DH(Bob's SPK private, Alice's ephemeral public)
	// Alice computed: DH(Alice's ephemeral private, Bob's SPK public)
	// These produce the same shared secret
	dhOutput, err := protocol.DH(session.DHSendingPrivate, session.DHReceivingPublic)
	if err != nil {
		return fmt.Errorf("initial DH failed: %w", err)
	}

	// Derive receiving chain key from the DH output
	newRootKey, receivingChainKey, err := protocol.KDF_RK(session.RootKey, dhOutput)
	if err != nil {
		return fmt.Errorf("initial KDF failed: %w", err)
	}

	session.RootKey = newRootKey
	session.ReceivingChainKey = receivingChainKey

	// Store session
	c.ratchetSessions[from] = session

	// Persist session if storage is attached
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveRatchetSession(from, session); err != nil {
			log.Printf("⚠️  Failed to persist ratchet session: %v", err)
		}
	}

	log.Printf("✅ Ratchet session initialized with %x (responder)", from[:8])
	return nil
}

// GetRatchetSession retrieves an existing ratchet session
func (c *Client) GetRatchetSession(addr protocol.Address) (*protocol.RatchetState, bool) {
	session, exists := c.ratchetSessions[addr]
	return session, exists
}

// SetRatchetSession stores a ratchet session
func (c *Client) SetRatchetSession(addr protocol.Address, session *protocol.RatchetState) {
	c.ratchetSessions[addr] = session

	// Persist session if storage is attached
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveRatchetSession(addr, session); err != nil {
			log.Printf("⚠️  Failed to persist ratchet session: %v", err)
		}
	}
}

// tryDecryptRatchetMessage attempts to decrypt a ratchet message
// Returns (plaintext, true) if successful, (nil, false) otherwise
func (c *Client) tryDecryptRatchetMessage(payload []byte, from protocol.Address) ([]byte, bool) {
	// Check if payload looks like a ratchet message: [header length (2 bytes)] + [header] + [ciphertext]
	if len(payload) < 2 {
		return nil, false
	}

	// Extract header length
	headerLen := uint16(payload[0])<<8 | uint16(payload[1])

	// Sanity check: header length should be reasonable (MessageHeader is ~80 bytes)
	if headerLen < 40 || headerLen > 200 || len(payload) < int(2+headerLen) {
		return nil, false
	}

	// Extract header and ciphertext
	ratchetHeader := payload[2 : 2+headerLen]
	ciphertext := payload[2+headerLen:]

	// Check if we have a ratchet session with this sender
	session, exists := c.ratchetSessions[from]
	if !exists {
		log.Printf("⚠️  Received ratchet message from %x but no session exists", from[:8])
		return nil, false
	}

	// Decrypt using ratchet
	plaintext, err := session.RatchetDecrypt(ratchetHeader, ciphertext, AESDecryptGCM)
	if err != nil {
		log.Printf("Failed to decrypt ratchet message from %x: %v", from[:8], err)
		return nil, false
	}

	// Persist updated session state (ratchet advances keys after each message)
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveRatchetSession(from, session); err != nil {
			log.Printf("⚠️  Failed to persist ratchet session after decrypt: %v", err)
		}
	}

	log.Printf("🔓 Ratchet message decrypted from %x: %d bytes", from[:8], len(plaintext))
	return plaintext, true
}
