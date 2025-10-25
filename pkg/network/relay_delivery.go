package network

import (
	"fmt"
	"log"
	"time"

	"github.com/zentalk/protocol/pkg/protocol"
)

// forwardToNextHop forwards message to next relay
func (rs *RelayServer) forwardToNextHop(nextHop protocol.Address, payload []byte) error {
	// Find peer connection
	rs.mu.RLock()
	peer, exists := rs.peers[string(nextHop[:])]
	rs.mu.RUnlock()

	if !exists {
		log.Printf("Next hop relay not connected: %x", nextHop)
		return fmt.Errorf("peer not connected: %x", nextHop)
	}

	log.Printf("Forwarding to next hop relay %x", nextHop)

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to peer
	if err := protocol.WriteHeader(peer.Conn, header); err != nil {
		return err
	}

	_, err := peer.Conn.Write(payload)
	if err == nil {
		log.Printf("✅ Forwarded to relay %x", nextHop)
	}
	return err
}

// deliverMessage delivers final message to recipient
func (rs *RelayServer) deliverMessage(recipientAddr protocol.Address, encryptedPayload []byte) error {
	log.Printf("Delivering message to %x", recipientAddr)

	// Find recipient peer
	rs.mu.RLock()
	peer, exists := rs.peers[string(recipientAddr[:])]
	rs.mu.RUnlock()

	if !exists {
		log.Printf("Recipient not connected: %x", recipientAddr)

		// Queue message if message queue is available
		if rs.messageQueue != nil {
			messageID := protocol.GenerateMessageID()
			if err := rs.messageQueue.QueueMessage(recipientAddr, messageID, encryptedPayload); err != nil {
				log.Printf("Failed to queue message: %v", err)
				return fmt.Errorf("recipient offline and queue failed: %v", err)
			}
			log.Printf("✅ Message queued for offline user %x", recipientAddr[:8])
			return nil
		}

		return fmt.Errorf("recipient not connected: %x", recipientAddr)
	}

	// Create header for direct message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeDirectMessage,
		Length:    uint32(len(encryptedPayload)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to recipient
	if err := protocol.WriteHeader(peer.Conn, header); err != nil {
		log.Printf("Write header error: %v", err)
		return err
	}

	if _, err := peer.Conn.Write(encryptedPayload); err != nil {
		log.Printf("Write payload error: %v", err)
		return err
	}

	log.Printf("✅ Message delivered to %x", recipientAddr)
	return nil
}

// deliverQueuedMessages delivers all queued messages to a reconnected user
func (rs *RelayServer) deliverQueuedMessages(recipientAddr protocol.Address) {
	// Get queued messages
	messages, err := rs.messageQueue.GetQueuedMessages(recipientAddr)
	if err != nil {
		log.Printf("Failed to get queued messages: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	log.Printf("📬 Delivering %d queued messages to %x", len(messages), recipientAddr[:8])

	// Find recipient peer
	rs.mu.RLock()
	peer, exists := rs.peers[string(recipientAddr[:])]
	rs.mu.RUnlock()

	if !exists {
		log.Printf("Recipient disconnected before queue delivery: %x", recipientAddr[:8])
		return
	}

	// Deliver each message
	successCount := 0
	for _, msg := range messages {
		// Create header for direct message
		header := &protocol.Header{
			Magic:     protocol.ProtocolMagic,
			Version:   protocol.ProtocolVersion,
			Type:      protocol.MsgTypeDirectMessage,
			Length:    uint32(len(msg.EncryptedPayload)),
			Flags:     protocol.FlagEncrypted,
			MessageID: protocol.GenerateMessageID(),
		}

		// Send to recipient
		if err := protocol.WriteHeader(peer.Conn, header); err != nil {
			log.Printf("Failed to deliver queued message: %v", err)
			continue
		}

		if _, err := peer.Conn.Write(msg.EncryptedPayload); err != nil {
			log.Printf("Failed to write queued message: %v", err)
			continue
		}

		// Delete message from queue after successful delivery
		if err := rs.messageQueue.DeleteMessage(msg.MessageID); err != nil {
			log.Printf("Failed to delete delivered message: %v", err)
		}

		successCount++
		time.Sleep(50 * time.Millisecond) // Small delay between messages
	}

	log.Printf("✅ Delivered %d/%d queued messages to %x", successCount, len(messages), recipientAddr[:8])
}
