package network

import (
	"crypto/rsa"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/protocol"
	"github.com/zentalk/protocol/pkg/storage"
)

// SendTypingIndicator sends a typing status notification
func (c *Client) SendTypingIndicator(to protocol.Address, recipientPubKey *rsa.PublicKey, isTyping bool, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create typing indicator
	indicator := &protocol.TypingIndicator{
		From:      c.Address,
		To:        to,
		Timestamp: uint64(time.Now().UnixMilli()),
		IsTyping:  isTyping,
	}

	// Encode
	payload := indicator.Encode()

	// Encrypt with recipient's public key
	encryptedMsg, err := crypto.RSAEncrypt(payload, recipientPubKey)
	if err != nil {
		return err
	}

	// Build onion layers
	onion, err := crypto.BuildOnionLayers(relayPath, to, encryptedMsg)
	if err != nil {
		return err
	}

	// Create header (use RelayForward since it goes through onion routing)
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

	if isTyping {
		log.Printf("⌨️  Typing indicator sent to %x (typing)", to[:8])
	} else {
		log.Printf("⌨️  Typing indicator sent to %x (stopped)", to[:8])
	}

	return nil
}

// SendReadReceipt sends a read receipt for a message
func (c *Client) SendReadReceipt(to protocol.Address, recipientPubKey *rsa.PublicKey, messageID protocol.MessageID, readStatus uint8, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create read receipt
	receipt := &protocol.ReadReceipt{
		From:       c.Address,
		To:         to,
		MessageID:  messageID,
		Timestamp:  uint64(time.Now().UnixMilli()),
		ReadStatus: readStatus,
	}

	// Encode
	payload := receipt.Encode()

	// Encrypt with recipient's public key
	encryptedMsg, err := crypto.RSAEncrypt(payload, recipientPubKey)
	if err != nil {
		return err
	}

	// Build onion layers
	onion, err := crypto.BuildOnionLayers(relayPath, to, encryptedMsg)
	if err != nil {
		return err
	}

	// Create header (use RelayForward since it goes through onion routing)
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

	statusName := "delivered"
	if readStatus == protocol.ReadStatusRead {
		statusName = "read"
	} else if readStatus == protocol.ReadStatusSeen {
		statusName = "seen"
	}

	log.Printf("✓✓ Read receipt sent to %x (status: %s)", to[:8], statusName)

	return nil
}

// MarkMessageAsRead marks a message as read and sends a read receipt
func (c *Client) MarkMessageAsRead(from protocol.Address, senderPubKey *rsa.PublicKey, messageID protocol.MessageID, relayPath []*crypto.RelayInfo) error {
	// Update database if attached
	if c.messageDB != nil {
		msgIDStr := fmt.Sprintf("%x", messageID)
		if err := c.messageDB.UpdateMessageStatus(msgIDStr, storage.MessageStatusRead); err != nil {
			log.Printf("Failed to update message status: %v", err)
		}
	}

	// Send read receipt
	return c.SendReadReceipt(from, senderPubKey, messageID, protocol.ReadStatusRead, relayPath)
}

// handleTypingIndicator handles incoming typing indicators
func (c *Client) handleTypingIndicator(header *protocol.Header) {
	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(c.relayConn, payload); err != nil {
		log.Printf("Read payload error: %v", err)
		return
	}

	// Decrypt with our private key
	decrypted, err := crypto.RSADecrypt(payload, c.PrivateKey)
	if err != nil {
		log.Printf("Decrypt typing indicator error: %v", err)
		return
	}

	// Decode typing indicator
	var indicator protocol.TypingIndicator
	if err := indicator.Decode(decrypted); err != nil {
		log.Printf("Decode typing indicator error: %v", err)
		return
	}

	// Check if it's for us
	if indicator.To != c.Address {
		return
	}

	if indicator.IsTyping {
		log.Printf("⌨️  %x is typing...", indicator.From[:8])
	} else {
		log.Printf("⌨️  %x stopped typing", indicator.From[:8])
	}

	// Call callback
	if c.OnTypingIndicator != nil {
		c.OnTypingIndicator(&indicator)
	}
}

// handleReadReceipt handles incoming read receipts
func (c *Client) handleReadReceipt(header *protocol.Header) {
	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(c.relayConn, payload); err != nil {
		log.Printf("Read payload error: %v", err)
		return
	}

	// Decrypt with our private key
	decrypted, err := crypto.RSADecrypt(payload, c.PrivateKey)
	if err != nil {
		log.Printf("Decrypt read receipt error: %v", err)
		return
	}

	// Decode read receipt
	var receipt protocol.ReadReceipt
	if err := receipt.Decode(decrypted); err != nil {
		log.Printf("Decode read receipt error: %v", err)
		return
	}

	// Check if it's for us
	if receipt.To != c.Address {
		return
	}

	statusName := "delivered"
	if receipt.ReadStatus == protocol.ReadStatusRead {
		statusName = "read"
	} else if receipt.ReadStatus == protocol.ReadStatusSeen {
		statusName = "seen"
	}

	log.Printf("✓✓ Read receipt from %x: message %x is %s", receipt.From[:8], receipt.MessageID[:8], statusName)

	// Update message status in database
	if c.messageDB != nil {
		msgIDStr := fmt.Sprintf("%x", receipt.MessageID)
		var status storage.MessageStatus
		switch receipt.ReadStatus {
		case protocol.ReadStatusDelivered:
			status = storage.MessageStatusDelivered
		case protocol.ReadStatusRead:
			status = storage.MessageStatusRead
		case protocol.ReadStatusSeen:
			status = storage.MessageStatusRead // Treat seen as read
		}

		if err := c.messageDB.UpdateMessageStatus(msgIDStr, status); err != nil {
			log.Printf("Failed to update message status in DB: %v", err)
		}
	}

	// Call callback
	if c.OnReadReceipt != nil {
		c.OnReadReceipt(&receipt)
	}
}
