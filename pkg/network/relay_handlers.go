package network

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/protocol"
)

// handleHandshake handles connection handshake and returns the peer address
func (rs *RelayServer) handleHandshake(conn net.Conn, header *protocol.Header) protocol.Address {
	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		log.Printf("Read payload error: %v", err)
		return protocol.Address{}
	}

	// Decode handshake
	var hs protocol.HandshakeMessage
	if err := hs.Decode(payload); err != nil {
		log.Printf("Decode handshake error: %v", err)
		return protocol.Address{}
	}

	log.Printf("Handshake from %x, type=%d", hs.Address, hs.ClientType)

	// Import public key
	publicKey, err := crypto.ImportPublicKeyPEM(hs.PublicKey)
	if err != nil {
		log.Printf("Import public key error: %v", err)
		return protocol.Address{}
	}

	// Store peer
	peer := &Peer{
		Conn:       conn,
		Address:    hs.Address,
		PublicKey:  publicKey,
		ClientType: hs.ClientType,
		LastSeen:   time.Now(),
	}

	rs.mu.Lock()
	rs.peers[string(hs.Address[:])] = peer
	rs.mu.Unlock()

	// Send handshake ACK
	rs.sendHandshakeAck(conn)

	log.Printf("Peer registered: %x", hs.Address)

	// Deliver queued messages for this user (if any)
	if rs.messageQueue != nil && hs.ClientType == protocol.ClientTypeUser {
		go rs.deliverQueuedMessages(hs.Address)
	}

	return hs.Address
}

// handleRelayForward handles message forwarding
func (rs *RelayServer) handleRelayForward(conn net.Conn, header *protocol.Header) {
	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		log.Printf("Read payload error: %v", err)
		return
	}

	// Decrypt onion layer
	layer, err := crypto.DecryptOnionLayer(payload, rs.PrivateKey)
	if err != nil {
		log.Printf("Decrypt onion error: %v", err)
		return
	}

	// Check if this is final delivery or forwarding
	// If NextHop is zero, it's an error (should have recipient address)
	// Otherwise, check if it's a relay or a client
	if crypto.IsDeliveryAddress(layer.NextHop) {
		log.Printf("Error: NextHop is zero, cannot deliver")
		return
	}

	// Check if next hop is connected
	rs.mu.RLock()
	peer, exists := rs.peers[string(layer.NextHop[:])]
	rs.mu.RUnlock()

	if !exists {
		log.Printf("Next hop not connected: %x", layer.NextHop)

		// Try to queue the message (deliverMessage will handle queuing if messageQueue is available)
		rs.deliverMessage(layer.NextHop, layer.Payload)
		return
	}

	// Check if it's a relay or client
	if peer.ClientType == protocol.ClientTypeRelay {
		// Forward to next relay
		log.Printf("Forwarding to next hop relay: %x", layer.NextHop)
		rs.forwardToNextHop(layer.NextHop, layer.Payload)
	} else {
		// Deliver to client
		log.Printf("Delivering message to client: %x", layer.NextHop)
		rs.deliverMessage(layer.NextHop, layer.Payload)
	}

	// Increment relay counter
	rs.messagesRelayed++
	if rs.OnMessageRelayed != nil {
		rs.OnMessageRelayed()
	}

	// Send ACK
	rs.sendAck(conn, header.MessageID)
}

// handlePing handles ping messages
func (rs *RelayServer) handlePing(conn net.Conn, header *protocol.Header) {
	log.Println("Ping received, sending pong")

	// Send pong
	pongHeader := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypePong,
		Length:    0,
		Flags:     0,
		MessageID: header.MessageID,
	}

	if err := protocol.WriteHeader(conn, pongHeader); err != nil {
		log.Printf("Write pong error: %v", err)
	} else {
		log.Println("Pong sent successfully")
	}
}

// sendHandshakeAck sends handshake acknowledgment
func (rs *RelayServer) sendHandshakeAck(conn net.Conn) error {
	// Export public key
	pubKeyPEM, err := crypto.ExportPublicKeyPEM(rs.PublicKey)
	if err != nil {
		return err
	}

	// Create handshake ACK
	hs := &protocol.HandshakeMessage{
		ProtocolVersion: protocol.ProtocolVersion,
		Address:         rs.Address,
		PublicKey:       pubKeyPEM,
		ClientType:      protocol.ClientTypeRelay,
		Timestamp:       uint64(time.Now().Unix()),
	}

	payload := hs.Encode()

	// Create header
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeHandshakeAck,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send header + payload
	if err := protocol.WriteHeader(conn, header); err != nil {
		return err
	}

	_, err = conn.Write(payload)
	return err
}

// sendAck sends acknowledgment
func (rs *RelayServer) sendAck(conn net.Conn, messageID protocol.MessageID) error {
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeAck,
		Length:    0,
		Flags:     0,
		MessageID: messageID,
	}

	return protocol.WriteHeader(conn, header)
}
