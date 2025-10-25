package network

import (
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
	"github.com/ZentaChain/zentalk-node/pkg/protocol"
	"github.com/ZentaChain/zentalk-node/pkg/storage"
)

// SendRatchetMessage sends an encrypted message using Double Ratchet
// Provides forward secrecy - each message uses a unique key
// If recipientKeyBundle is nil, it will try to use a cached bundle
func (c *Client) SendRatchetMessage(to protocol.Address, recipientKeyBundle *protocol.KeyBundle, plaintext []byte, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	if c.x3dhIdentity == nil {
		return errors.New("X3DH not initialized - call InitializeX3DH() first")
	}

	// Check if we have an existing ratchet session
	session, exists := c.ratchetSessions[to]

	if !exists {
		// No session exists - perform X3DH key agreement
		log.Printf("🔐 No ratchet session with %x, performing X3DH...", to[:8])

		// If no bundle provided, check cache
		if recipientKeyBundle == nil {
			cachedBundle, found := c.GetCachedKeyBundle(to)
			if !found {
				return fmt.Errorf("no key bundle available for %x - provide bundle or cache it first", to[:8])
			}
			recipientKeyBundle = cachedBundle
			log.Printf("Using cached key bundle for %x", to[:8])
		}

		// Perform X3DH as initiator
		sharedSecret, ephemPriv, ephemPub, initialMsg, err := protocol.X3DHInitiator(c.Address, c.x3dhIdentity, recipientKeyBundle)
		if err != nil {
			return fmt.Errorf("X3DH failed: %w", err)
		}

		log.Printf("✅ X3DH completed: SharedSecret=%x..., UsedOPK=%d", sharedSecret[:8], initialMsg.UsedOneTimePreKeyID)

		// Initialize ratchet session with shared secret
		// Use ephemeral keys from X3DH for the initial ratchet DH
		session, err = protocol.NewRatchetState(
			sharedSecret,
			recipientKeyBundle.SignedPreKey.PublicKey,
			ephemPriv,
			ephemPub,
			c.Address,
			to,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize ratchet state: %w", err)
		}

		// Store session
		c.ratchetSessions[to] = session

		// Persist session if storage is attached
		if c.sessionStorage != nil {
			if err := c.sessionStorage.SaveRatchetSession(to, session); err != nil {
				log.Printf("⚠️  Failed to persist new ratchet session: %v", err)
			}
		}

		// Send X3DH initial message to recipient so they can set up their session
		if err := c.sendX3DHInitialMessage(to, initialMsg, relayPath); err != nil {
			return fmt.Errorf("failed to send X3DH initial message: %w", err)
		}

		log.Printf("✅ X3DH initial message sent to %x", to[:8])
	}

	// Encrypt message using ratchet
	ratchetHeader, ciphertext, err := session.RatchetEncrypt(plaintext, AESEncryptGCM)
	if err != nil {
		return fmt.Errorf("ratchet encryption failed: %w", err)
	}

	// Persist updated session state (ratchet advances keys after each message)
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveRatchetSession(to, session); err != nil {
			log.Printf("⚠️  Failed to persist ratchet session after encrypt: %v", err)
		}
	}

	log.Printf("🔒 Message encrypted with ratchet: header=%d bytes, ciphertext=%d bytes", len(ratchetHeader), len(ciphertext))

	// Create ratchet message payload: [header length (2 bytes)] + [header] + [ciphertext]
	headerLen := uint16(len(ratchetHeader))
	ratchetPayload := make([]byte, 2+len(ratchetHeader)+len(ciphertext))
	ratchetPayload[0] = byte(headerLen >> 8)
	ratchetPayload[1] = byte(headerLen)
	copy(ratchetPayload[2:], ratchetHeader)
	copy(ratchetPayload[2+len(ratchetHeader):], ciphertext)

	// Build onion layers around the ratchet payload
	onion, err := crypto.BuildOnionLayers(relayPath, to, ratchetPayload)
	if err != nil {
		return err
	}

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(onion)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to relay
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(onion); err != nil {
		return err
	}

	log.Printf("📤 Ratchet message sent to %x via %d relays (forward secrecy enabled)", to[:8], len(relayPath))
	return nil
}

// sendX3DHInitialMessage sends the X3DH initial message to recipient
func (c *Client) sendX3DHInitialMessage(to protocol.Address, initialMsg *protocol.InitialMessage, relayPath []*crypto.RelayInfo) error {
	// Encode initial message
	encoded := initialMsg.Encode()

	// Wrap in a simple container with a marker to identify it as X3DH init message
	// Format: [magic bytes "X3DH"] + [encoded initial message]
	payload := make([]byte, 4+len(encoded))
	copy(payload[0:4], []byte("X3DH"))
	copy(payload[4:], encoded)

	// Build onion layers
	onion, err := crypto.BuildOnionLayers(relayPath, to, payload)
	if err != nil {
		return err
	}

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(onion)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to relay
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(onion); err != nil {
		return err
	}

	return nil
}

// GetNextSequenceNumber gets and increments the sequence number for a peer
func (c *Client) GetNextSequenceNumber(to protocol.Address) uint64 {
	seqNum := c.sendSequenceNumbers[to]
	c.sendSequenceNumbers[to] = seqNum + 1
	return seqNum
}

// SendMessage sends a message through the relay network with specified content type
func (c *Client) SendMessage(to protocol.Address, recipientPubKey *rsa.PublicKey, content []byte, contentType uint8, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create direct message with sequence number
	msg := &protocol.DirectMessage{
		From:           c.Address,
		To:             to,
		Timestamp:      uint64(time.Now().UnixMilli()),
		SequenceNumber: c.GetNextSequenceNumber(to),
		ContentType:    contentType,
		Content:        content,
	}

	// Encode message
	msgPayload := msg.Encode()

	// Encrypt message with recipient's public key (end-to-end encryption)
	encryptedMsg, err := crypto.RSAEncrypt(msgPayload, recipientPubKey)
	if err != nil {
		return err
	}

	// Build onion layers around encrypted message
	onion, err := crypto.BuildOnionLayers(relayPath, to, encryptedMsg)
	if err != nil {
		return err
	}

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(onion)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to relay
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(onion); err != nil {
		return err
	}

	// Save outgoing message to database
	if c.messageDB != nil {
		conversationID := storage.GetConversationID(
			hex.EncodeToString(c.Address[:]),
			hex.EncodeToString(to[:]),
		)

		storedMsg := &storage.StoredMessage{
			ConversationID: conversationID,
			MessageID:      fmt.Sprintf("%x", header.MessageID),
			FromAddress:    hex.EncodeToString(c.Address[:]),
			ToAddress:      hex.EncodeToString(to[:]),
			Content:        content,
			ContentType:    contentType,
			Timestamp:      int64(msg.Timestamp),
			Status:         storage.MessageStatusSent,
			IsOutgoing:     true,
		}

		if err := c.messageDB.SaveMessage(storedMsg); err != nil {
			log.Printf("Failed to save outgoing message to DB: %v", err)
		}
	}

	log.Printf("Message sent to %x via %d relays (type: 0x%02x)", to, len(relayPath), contentType)
	return nil
}

// SendTextMessage sends a text message (convenience wrapper)
func (c *Client) SendTextMessage(to protocol.Address, recipientPubKey *rsa.PublicKey, text string, relayPath []*crypto.RelayInfo) error {
	return c.SendMessage(to, recipientPubKey, []byte(text), protocol.ContentTypeText, relayPath)
}

// MediaMessage represents the content structure for media messages
// Format: [ChunkID (8 bytes)] + [32-byte encryption key]
type MediaMessage struct {
	ChunkID       uint64
	EncryptionKey [32]byte
}

// SendMediaMessage uploads encrypted media to MeshStorage and sends ChunkID + key to recipient
// mediaType: Image, Video, Audio, or File
// Returns: (ChunkID, encryption key, error)
func (c *Client) SendMediaMessage(to protocol.Address, recipientPubKey *rsa.PublicKey, mediaData []byte, mediaType uint8, meshStorageClient interface{}, relayPath []*crypto.RelayInfo) (uint64, []byte, error) {
	if !c.connected {
		return 0, nil, ErrNotConnected
	}

	// Type assert meshStorageClient to access UploadEncrypted
	type MeshStorageUploader interface {
		UploadEncrypted(data []byte) (uint64, []byte, error)
	}

	uploader, ok := meshStorageClient.(MeshStorageUploader)
	if !ok {
		return 0, nil, errors.New("invalid MeshStorage client - must implement UploadEncrypted")
	}

	// Upload encrypted media to MeshStorage
	chunkID, encryptionKey, err := uploader.UploadEncrypted(mediaData)
	if err != nil {
		return 0, nil, err
	}

	log.Printf("Media uploaded to MeshStorage: ChunkID=%d, size=%d bytes, encrypted with AES-256", chunkID, len(mediaData))

	// Create media message content: [ChunkID (8 bytes)] + [32-byte key]
	content := make([]byte, 8+32)

	// Write ChunkID (big-endian uint64)
	content[0] = byte(chunkID >> 56)
	content[1] = byte(chunkID >> 48)
	content[2] = byte(chunkID >> 40)
	content[3] = byte(chunkID >> 32)
	content[4] = byte(chunkID >> 24)
	content[5] = byte(chunkID >> 16)
	content[6] = byte(chunkID >> 8)
	content[7] = byte(chunkID)

	// Write encryption key
	copy(content[8:], encryptionKey)

	// Send media message with the ChunkID + key as content
	if err := c.SendMessage(to, recipientPubKey, content, mediaType, relayPath); err != nil {
		return 0, nil, err
	}

	log.Printf("Media message sent to %x (type: 0x%02x, ChunkID: %d)", to, mediaType, chunkID)
	return chunkID, encryptionKey, nil
}

// ParseMediaMessage parses media message content to extract ChunkID and encryption key
func ParseMediaMessage(content []byte) (chunkID uint64, encryptionKey []byte, err error) {
	if len(content) < 40 {
		return 0, nil, errors.New("invalid media message: too short (expected 40 bytes minimum)")
	}

	// Extract ChunkID (8 bytes, big-endian)
	chunkID = uint64(content[0])<<56 |
		uint64(content[1])<<48 |
		uint64(content[2])<<40 |
		uint64(content[3])<<32 |
		uint64(content[4])<<24 |
		uint64(content[5])<<16 |
		uint64(content[6])<<8 |
		uint64(content[7])

	// Extract encryption key (32 bytes)
	encryptionKey = make([]byte, 32)
	copy(encryptionKey, content[8:40])

	return chunkID, encryptionKey, nil
}
