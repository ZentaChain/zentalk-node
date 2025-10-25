package network

import (
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/dht"
	"github.com/zentalk/protocol/pkg/protocol"
	"github.com/zentalk/protocol/pkg/storage"
)

// RelayServer represents a relay server
type RelayServer struct {
	Address    protocol.Address
	Port       int
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey

	listener net.Listener
	peers    map[string]*Peer
	mu       sync.RWMutex

	// Message queue for offline users
	messageQueue *storage.RelayMessageQueue

	// DHT for relay discovery
	dhtNode        *dht.Node
	relayDiscovery *RelayDiscovery
	metadata       *RelayMetadata
	startTime      time.Time

	// Statistics
	messagesRelayed uint64
	lastHeartbeat   time.Time

	// Callbacks
	OnMessageRelayed func()
}

// Peer represents a connected peer (relay or client)
type Peer struct {
	Conn       net.Conn
	Address    protocol.Address
	PublicKey  *rsa.PublicKey
	ClientType uint8
	LastSeen   time.Time
}

// NewRelayServer creates a new relay server
func NewRelayServer(port int, privateKey *rsa.PrivateKey) *RelayServer {
	return &RelayServer{
		Port:       port,
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		peers:      make(map[string]*Peer),
		startTime:  time.Now(),
	}
}

// AttachMessageQueue attaches a message queue for offline message storage
func (rs *RelayServer) AttachMessageQueue(queue *storage.RelayMessageQueue) {
	rs.messageQueue = queue
	log.Println("📬 Message queue attached to relay server")
}

// GetMessageQueue returns the message queue (for cleanup operations)
func (rs *RelayServer) GetMessageQueue() *storage.RelayMessageQueue {
	return rs.messageQueue
}

// Start starts the relay server
func (rs *RelayServer) Start() error {
	addr := fmt.Sprintf(":%d", rs.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	rs.listener = listener
	log.Printf("Relay server listening on %s", addr)

	go rs.acceptLoop()

	return nil
}

// Stop stops the relay server
func (rs *RelayServer) Stop() error {
	if rs.listener != nil {
		return rs.listener.Close()
	}
	return nil
}

// ConnectToRelay connects this relay to another relay
func (rs *RelayServer) ConnectToRelay(relayAddress string, relayAddr protocol.Address) error {
	// Check if already connected
	rs.mu.RLock()
	_, exists := rs.peers[string(relayAddr[:])]
	rs.mu.RUnlock()

	if exists {
		log.Printf("Already connected to relay %x", relayAddr)
		return nil
	}

	log.Printf("Connecting to relay %s (%x)", relayAddress, relayAddr)

	// Dial the relay
	conn, err := net.Dial("tcp", relayAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %v", err)
	}

	// Export our public key
	pubKeyPEM, err := crypto.ExportPublicKeyPEM(rs.PublicKey)
	if err != nil {
		conn.Close()
		return err
	}

	// Send handshake
	hs := &protocol.HandshakeMessage{
		ProtocolVersion: protocol.ProtocolVersion,
		Address:         rs.Address,
		PublicKey:       pubKeyPEM,
		ClientType:      protocol.ClientTypeRelay,
		Timestamp:       uint64(time.Now().Unix()),
	}

	payload := hs.Encode()

	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeHandshake,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	if err := protocol.WriteHeader(conn, header); err != nil {
		conn.Close()
		return err
	}

	if _, err := conn.Write(payload); err != nil {
		conn.Close()
		return err
	}

	// Wait for handshake ACK
	ackHeader, err := protocol.ReadHeader(conn)
	if err != nil {
		conn.Close()
		return err
	}

	if ackHeader.Type != protocol.MsgTypeHandshakeAck {
		conn.Close()
		return fmt.Errorf("expected handshake ACK, got %x", ackHeader.Type)
	}

	// Read and discard ACK payload
	if ackHeader.Length > 0 {
		ackPayload := make([]byte, ackHeader.Length)
		if _, err := io.ReadFull(conn, ackPayload); err != nil {
			conn.Close()
			return err
		}
	}

	// Store peer
	peer := &Peer{
		Conn:       conn,
		Address:    relayAddr,
		PublicKey:  nil,                      // Could decode from ACK if needed
		ClientType: protocol.ClientTypeRelay, // Connecting to another relay
		LastSeen:   time.Now(),
	}

	rs.mu.Lock()
	rs.peers[string(relayAddr[:])] = peer
	rs.mu.Unlock()

	log.Printf("✅ Connected to relay %x", relayAddr)

	// Start handling messages from this relay
	go rs.handleConnection(conn)

	return nil
}

// GetStats returns relay statistics
func (rs *RelayServer) GetStats() map[string]interface{} {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	stats := map[string]interface{}{
		"messages_relayed": rs.messagesRelayed,
		"connected_peers":  len(rs.peers),
		"last_heartbeat":   rs.lastHeartbeat,
	}

	// Add queue stats if available
	if rs.messageQueue != nil {
		queueSize, _ := rs.messageQueue.GetTotalQueueSize()
		stats["queued_messages"] = queueSize
	}

	return stats
}

// AttachDHT attaches a DHT node for relay discovery
func (rs *RelayServer) AttachDHT(node *dht.Node) {
	rs.dhtNode = node
	rs.relayDiscovery = NewRelayDiscovery(node)
	log.Printf("✅ DHT attached to relay server")
}

// SetRelayMetadata sets the relay's metadata for DHT publishing
func (rs *RelayServer) SetRelayMetadata(region, operator, version string, maxConnections int) error {
	if rs.dhtNode == nil {
		return fmt.Errorf("DHT not attached - call AttachDHT() first")
	}

	// Export public key to PEM
	pubKeyPEM, err := crypto.ExportPublicKeyPEM(rs.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to export public key: %w", err)
	}

	// Create metadata
	rs.metadata = &RelayMetadata{
		Address:        rs.Address,
		NetworkAddress: fmt.Sprintf("localhost:%d", rs.Port),
		PublicKeyPEM:   string(pubKeyPEM),
		Region:         region,
		Operator:       operator,
		Version:        version,
		MaxConnections: maxConnections,
		Uptime:         uint64(time.Since(rs.startTime).Seconds()),
		LastSeen:       time.Now().Unix(),
		Reliability:    0.95, // Default high reliability
	}

	log.Printf("✅ Relay metadata set: region=%s, operator=%s", region, operator)
	return nil
}

// PublishToDHT publishes the relay's metadata to the DHT
func (rs *RelayServer) PublishToDHT() error {
	if rs.dhtNode == nil {
		return fmt.Errorf("DHT not attached - call AttachDHT() first")
	}

	if rs.metadata == nil {
		return fmt.Errorf("metadata not set - call SetRelayMetadata() first")
	}

	// Update dynamic fields
	rs.metadata.Uptime = uint64(time.Since(rs.startTime).Seconds())
	rs.metadata.LastSeen = time.Now().Unix()

	// Publish to DHT
	if err := rs.relayDiscovery.PublishRelay(rs.metadata); err != nil {
		return fmt.Errorf("failed to publish relay: %w", err)
	}

	log.Printf("✅ Relay published to DHT (uptime: %ds)", rs.metadata.Uptime)
	return nil
}

// AutoPublishToDHT automatically republishes relay metadata periodically
// Should be run in a goroutine
func (rs *RelayServer) AutoPublishToDHT(interval time.Duration) {
	if rs.dhtNode == nil {
		log.Printf("⚠️  DHT not attached, skipping auto-publish")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Publish immediately
	if err := rs.PublishToDHT(); err != nil {
		log.Printf("⚠️  Initial relay publish failed: %v", err)
	}

	for {
		<-ticker.C

		if err := rs.PublishToDHT(); err != nil {
			log.Printf("⚠️  Failed to auto-publish relay: %v", err)
		} else {
			log.Printf("🔄 Relay metadata republished to DHT")
		}
	}
}

// GetMetadata returns the relay's metadata
func (rs *RelayServer) GetMetadata() *RelayMetadata {
	if rs.metadata != nil {
		// Update dynamic fields before returning
		rs.metadata.Uptime = uint64(time.Since(rs.startTime).Seconds())
		rs.metadata.LastSeen = time.Now().Unix()
	}
	return rs.metadata
}

// Helper to write uint64
func writeUint64(w io.Writer, v uint64) error {
	return binary.Write(w, binary.BigEndian, v)
}

// Helper to read uint64
func readUint64(r io.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}
