package network

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/protocol"
	"github.com/zentalk/protocol/pkg/storage"
)

var (
	messageOrderingMu sync.Mutex // Protects message ordering operations
)

// receiveLoop receives messages from relay
func (c *Client) receiveLoop() {
	for c.connected {
		header, err := protocol.ReadHeader(c.relayConn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read header error: %v", err)
			}
			break
		}

		// Handle message based on type
		switch header.Type {
		case protocol.MsgTypeDirectMessage:
			c.handleDirectMessage(header)

		case protocol.MsgTypeTyping:
			c.handleTypingIndicator(header)

		case protocol.MsgTypeReadReceipt:
			c.handleReadReceipt(header)

		case protocol.MsgTypePong:
			// Pong received
			log.Println("Pong received")

		case protocol.MsgTypeAck:
			// Acknowledgment received
			c.handleAckMessage(header)

		case protocol.MsgTypeNack:
			// Negative acknowledgment received
			c.handleNackMessage(header)

		default:
			log.Printf("Unknown message type: 0x%04x", header.Type)
		}
	}

	log.Println("Receive loop stopped")
}

// handleDirectMessage handles incoming direct message or group message
func (c *Client) handleDirectMessage(header *protocol.Header) {
	// Add panic recovery for decode errors
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in message decode: %v", r)
		}
	}()

	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(c.relayConn, payload); err != nil {
		log.Printf("Read payload error: %v", err)
		return
	}

	// Try different decryption methods in order:
	// 1. RSA decryption to unwrap onion routing
	// 2. Check for X3DH initial message (to set up ratchet session)
	// 3. Try ratchet decryption (forward secrecy messages)
	// 4. Try hybrid decryption (large messages like profiles)

	// FIRST: Try RSA decryption to unwrap end-to-end encryption
	// If RSA decryption fails, the payload might be a non-RSA message (like X3DH init)
	decrypted, err := crypto.RSADecrypt(payload, c.PrivateKey)
	if err != nil {
		// RSA decryption failed - payload might be non-encrypted (e.g., X3DH init message)
		// Use the raw payload as-is
		decrypted = payload
		log.Printf("RSA decryption failed, using raw payload: %v", err)
	}

	// SECOND: Check if this is an X3DH initial message
	if len(decrypted) > 4 && string(decrypted[0:4]) == "X3DH" {
		// This is an X3DH initial message - decode and initialize session
		var initialMsg protocol.InitialMessage
		if err := initialMsg.Decode(decrypted[4:]); err != nil {
			log.Printf("Failed to decode X3DH initial message: %v", err)
			return
		}

		log.Printf("📨 Received X3DH initial message from %x (sender: %x)", initialMsg.IdentityKey[:8], initialMsg.SenderAddress[:8])

		// Initialize ratchet session as responder using the sender address from the message
		if err := c.InitializeRatchetSession(initialMsg.SenderAddress, &initialMsg); err != nil {
			log.Printf("Failed to initialize ratchet session: %v", err)
			return
		}

		log.Printf("✅ Ratchet session initialized with %x from X3DH init message", initialMsg.SenderAddress[:8])
		return
	}

	// THIRD: Try ratchet decryption (detect by checking header length range)
	var finalPlaintext []byte
	if len(decrypted) > 2 {
		headerLen := uint16(decrypted[0])<<8 | uint16(decrypted[1])
		// Ratchet message headers are typically 40-200 bytes
		if headerLen >= 40 && headerLen <= 200 && len(decrypted) >= int(2+headerLen) {
			// This might be a ratchet message - try with all known sessions
			for addr, session := range c.ratchetSessions {
				plaintext, err := session.RatchetDecrypt(
					decrypted[2:2+headerLen],
					decrypted[2+headerLen:],
					AESDecryptGCM,
				)
				if err == nil {
					finalPlaintext = plaintext
					log.Printf("🔓 Ratchet message decrypted from %x: %d bytes", addr[:8], len(plaintext))
					break
				}
			}
		}
	}

	// FOURTH: If not ratchet, try hybrid decryption (for profiles)
	if finalPlaintext == nil && len(decrypted) > 2 {
		keyLen := uint16(decrypted[0])<<8 | uint16(decrypted[1])

		// If keyLen looks reasonable for an RSA-encrypted AES key (typically 512 bytes for RSA-4096)
		// and the payload is large enough, try hybrid decryption
		if keyLen > 0 && keyLen < 1024 && int(2+keyLen) < len(decrypted) {
			encryptedKey := decrypted[2 : 2+keyLen]
			encryptedData := decrypted[2+keyLen:]

			// Try to decrypt AES key with RSA
			aesKey, err := crypto.RSADecrypt(encryptedKey, c.PrivateKey)
			if err == nil {
				// Try to decrypt data with AES
				profileData, err := crypto.AESDecrypt(encryptedData, aesKey)
				if err == nil {
					finalPlaintext = profileData
				}
			}
		}
	}

	// If still no plaintext, use the RSA-decrypted data as-is (might be a regular message)
	if finalPlaintext == nil {
		finalPlaintext = decrypted
	}

	// Try to decode as DirectMessage first
	// Use a function to catch panics
	isDirectMessage := func() bool {
		defer func() {
			if r := recover(); r != nil {
				// Decode failed, not a direct message
			}
		}()

		var directMsg protocol.DirectMessage
		if err := directMsg.Decode(finalPlaintext); err == nil {
			// Check if this is actually for us (To field matches our address)
			if directMsg.To == c.Address {
				// Handle message with ordering and deduplication
				c.handleOrderedMessage(&directMsg)
				return true
			}
		}
		return false
	}()

	if isDirectMessage {
		return
	}

	// If not a direct message, try group message
	isGroupMessage := func() bool {
		defer func() {
			if r := recover(); r != nil {
				// Decode failed, not a group message
			}
		}()

		var groupMsg protocol.GroupMessage
		if err := groupMsg.Decode(finalPlaintext); err == nil {
			log.Printf("Group message received from %x in group %x: %s", groupMsg.From, groupMsg.GroupID, string(groupMsg.Content))
			if c.OnGroupMessageReceived != nil {
				c.OnGroupMessageReceived(&groupMsg)
			}
			return true
		}
		return false
	}()

	if isGroupMessage {
		return
	}

	// If not group message, try profile update
	isProfileUpdate := func() bool {
		defer func() {
			if r := recover(); r != nil {
				// Decode failed, not a profile update
			}
		}()

		// Try to decode as profile (hybrid decryption already done above)
		var profile protocol.ProfileUpdate
		if err := profile.Decode(finalPlaintext); err == nil {
			username := string(bytes.Trim(profile.Username[:], "\x00"))
			log.Printf("Profile update received from %x: %s", profile.Address, username)
			if c.OnProfileUpdate != nil {
				c.OnProfileUpdate(&profile)
			}
			return true
		}
		return false
	}()

	if !isProfileUpdate {
		// Try typing indicator
		isTypingIndicator := func() bool {
			defer func() {
				if r := recover(); r != nil {
					// Decode failed
				}
			}()

			var indicator protocol.TypingIndicator
			if err := indicator.Decode(finalPlaintext); err == nil {
				if indicator.To == c.Address {
					if indicator.IsTyping {
						log.Printf("⌨️  %x is typing...", indicator.From[:8])
					} else {
						log.Printf("⌨️  %x stopped typing", indicator.From[:8])
					}
					if c.OnTypingIndicator != nil {
						c.OnTypingIndicator(&indicator)
					}
					return true
				}
			}
			return false
		}()

		if !isTypingIndicator {
			// Try read receipt
			isReadReceipt := func() bool {
				defer func() {
					if r := recover(); r != nil {
						// Decode failed
					}
				}()

				var receipt protocol.ReadReceipt
				if err := receipt.Decode(finalPlaintext); err == nil {
					if receipt.To == c.Address {
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
								status = storage.MessageStatusRead
							}

							if err := c.messageDB.UpdateMessageStatus(msgIDStr, status); err != nil {
								log.Printf("Failed to update message status in DB: %v", err)
							}
						}

						if c.OnReadReceipt != nil {
							c.OnReadReceipt(&receipt)
						}
						return true
					}
				}
				return false
			}()

			if !isReadReceipt {
				log.Printf("Failed to decode message as direct, group, profile, typing, or receipt")
			}
		}
	}
}

// handleOrderedMessage handles message ordering, buffering, and deduplication
func (c *Client) handleOrderedMessage(msg *protocol.DirectMessage) {
	messageOrderingMu.Lock()
	defer messageOrderingMu.Unlock()

	from := msg.From
	seqNum := msg.SequenceNumber

	// Initialize structures for this peer if needed
	if c.messageBuffer[from] == nil {
		c.messageBuffer[from] = make(map[uint64]*protocol.DirectMessage)
	}
	if c.receivedMessageIDs[from] == nil {
		c.receivedMessageIDs[from] = make(map[uint64]bool)
	}

	// Check for duplicate message (deduplication)
	if c.receivedMessageIDs[from][seqNum] {
		log.Printf("⚠️  Duplicate message from %x (seq: %d) - discarding", from[:8], seqNum)
		return
	}

	// Mark message as received
	c.receivedMessageIDs[from][seqNum] = true

	expectedSeq := c.receiveSequenceNumbers[from]

	// Case 1: Message is in order (next expected sequence number)
	if seqNum == expectedSeq {
		log.Printf("📨 Message in order from %x (seq: %d): %s", from[:8], seqNum, string(msg.Content))

		// Deliver this message
		c.deliverMessage(msg)

		// Increment expected sequence number
		c.receiveSequenceNumbers[from] = expectedSeq + 1

		// Check if we can deliver buffered messages
		c.deliverBufferedMessages(from)
		return
	}

	// Case 2: Message is ahead (out of order - buffer it)
	if seqNum > expectedSeq {
		log.Printf("⚠️  Out-of-order message from %x (seq: %d, expected: %d) - buffering",
			from[:8], seqNum, expectedSeq)
		c.messageBuffer[from][seqNum] = msg
		return
	}

	// Case 3: Message is old (seq < expected) - probably a duplicate, discard
	log.Printf("⚠️  Old message from %x (seq: %d, expected: %d) - discarding",
		from[:8], seqNum, expectedSeq)
}

// deliverBufferedMessages delivers any buffered messages that are now in order
func (c *Client) deliverBufferedMessages(from protocol.Address) {
	for {
		expectedSeq := c.receiveSequenceNumbers[from]
		bufferedMsg, exists := c.messageBuffer[from][expectedSeq]

		if !exists {
			// No more consecutive messages in buffer
			break
		}

		log.Printf("📨 Delivering buffered message from %x (seq: %d): %s",
			from[:8], expectedSeq, string(bufferedMsg.Content))

		// Deliver the buffered message
		c.deliverMessage(bufferedMsg)

		// Remove from buffer
		delete(c.messageBuffer[from], expectedSeq)

		// Increment expected sequence number
		c.receiveSequenceNumbers[from] = expectedSeq + 1
	}
}

// deliverMessage delivers a message to the application layer
func (c *Client) deliverMessage(msg *protocol.DirectMessage) {
	log.Printf("✅ Direct message delivered from %x (seq: %d): %s",
		msg.From[:8], msg.SequenceNumber, string(msg.Content))

	// Save incoming message to database
	if c.messageDB != nil {
		conversationID := storage.GetConversationID(
			hex.EncodeToString(c.Address[:]),
			hex.EncodeToString(msg.From[:]),
		)

		storedMsg := &storage.StoredMessage{
			ConversationID: conversationID,
			MessageID:      fmt.Sprintf("%x-%d", msg.From, msg.Timestamp),
			FromAddress:    hex.EncodeToString(msg.From[:]),
			ToAddress:      hex.EncodeToString(msg.To[:]),
			Content:        msg.Content,
			ContentType:    msg.ContentType,
			Timestamp:      int64(msg.Timestamp),
			Status:         storage.MessageStatusDelivered,
			IsOutgoing:     false,
		}

		if err := c.messageDB.SaveMessage(storedMsg); err != nil {
			log.Printf("Failed to save incoming message to DB: %v", err)
		}
	}

	// Send ACK to sender
	c.sendAck(msg.From, msg.ReplyTo, msg.SequenceNumber)

	// Call application callback
	if c.OnMessageReceived != nil {
		c.OnMessageReceived(msg)
	}
}

// sendAck sends an acknowledgment for a received message
func (c *Client) sendAck(to protocol.Address, messageID protocol.MessageID, seqNum uint64) {
	if !c.connected {
		return
	}

	ack := &protocol.AckMessage{
		From:           c.Address,
		To:             to,
		MessageID:      messageID,
		SequenceNumber: seqNum,
		Timestamp:      uint64(time.Now().UnixMilli()),
	}

	payload := ack.Encode()

	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeAck,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		log.Printf("Failed to send ACK header: %v", err)
		return
	}

	if _, err := c.relayConn.Write(payload); err != nil {
		log.Printf("Failed to send ACK payload: %v", err)
		return
	}

	log.Printf("✓ ACK sent to %x (seq: %d)", to[:8], seqNum)
}

// sendNack sends a negative acknowledgment for a failed message
func (c *Client) sendNack(to protocol.Address, messageID protocol.MessageID, seqNum uint64, errorCode uint8, errorMsg string) {
	if !c.connected {
		return
	}

	nack := &protocol.NackMessage{
		From:           c.Address,
		To:             to,
		MessageID:      messageID,
		SequenceNumber: seqNum,
		Timestamp:      uint64(time.Now().UnixMilli()),
		ErrorCode:      errorCode,
		ErrorMessage:   []byte(errorMsg),
	}

	payload := nack.Encode()

	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeNack,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		log.Printf("Failed to send NACK header: %v", err)
		return
	}

	if _, err := c.relayConn.Write(payload); err != nil {
		log.Printf("Failed to send NACK payload: %v", err)
		return
	}

	log.Printf("✗ NACK sent to %x (seq: %d, error: %d)", to[:8], seqNum, errorCode)
}

// handleAckMessage handles incoming ACK messages
func (c *Client) handleAckMessage(header *protocol.Header) {
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(c.relayConn, payload); err != nil {
		log.Printf("Read ACK payload error: %v", err)
		return
	}

	var ack protocol.AckMessage
	if err := ack.Decode(payload); err != nil {
		log.Printf("Failed to decode ACK: %v", err)
		return
	}

	log.Printf("✓ ACK received from %x (seq: %d)", ack.From[:8], ack.SequenceNumber)

	// Call application callback
	if c.OnAckReceived != nil {
		c.OnAckReceived(&ack)
	}
}

// handleNackMessage handles incoming NACK messages
func (c *Client) handleNackMessage(header *protocol.Header) {
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(c.relayConn, payload); err != nil {
		log.Printf("Read NACK payload error: %v", err)
		return
	}

	var nack protocol.NackMessage
	if err := nack.Decode(payload); err != nil {
		log.Printf("Failed to decode NACK: %v", err)
		return
	}

	log.Printf("✗ NACK received from %x (seq: %d, error: %d): %s",
		nack.From[:8], nack.SequenceNumber, nack.ErrorCode, string(nack.ErrorMessage))

	// Call application callback
	if c.OnNackReceived != nil {
		c.OnNackReceived(&nack)
	}
}
