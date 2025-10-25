package network

import (
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/dht"
	"github.com/zentalk/protocol/pkg/protocol"
	"github.com/zentalk/protocol/pkg/storage"
)

var (
	ErrNotConnected    = errors.New("not connected")
	ErrHandshakeFailed = errors.New("handshake failed")
)

// Client represents a client connection to a relay
type Client struct {
	Address      protocol.Address
	PrivateKey   *rsa.PrivateKey
	PublicKey    *rsa.PublicKey
	relayConn    net.Conn
	relayAddress string
	connected    bool

	// Message persistence
	messageDB *storage.MessageDB

	// Session persistence (X3DH & ratchet state)
	sessionStorage *SessionStorage

	// DHT for decentralized key bundle discovery
	dhtNode *dht.Node

	// Relay discovery
	relayDiscovery *RelayDiscovery

	// X3DH & Double Ratchet (Forward Secrecy)
	x3dhIdentity   *protocol.IdentityKeyPair                   // Our X3DH identity
	signedPreKey   *protocol.SignedPreKeyPrivate               // Our current signed prekey
	oneTimePreKeys map[uint32]*protocol.OneTimePreKeyPrivate   // Pool of one-time prekeys
	ratchetSessions  map[protocol.Address]*protocol.RatchetState // Active ratchet sessions
	keyBundleCache map[protocol.Address]*protocol.KeyBundle    // Cached key bundles
	registrationID uint32                                       // Unique registration ID

	// Message ordering and reliability
	sendSequenceNumbers    map[protocol.Address]uint64                    // Next sequence number to send per peer
	receiveSequenceNumbers map[protocol.Address]uint64                    // Next expected sequence number per peer
	messageBuffer          map[protocol.Address]map[uint64]*protocol.DirectMessage // Out-of-order message buffer
	receivedMessageIDs     map[protocol.Address]map[uint64]bool           // Deduplication tracking

	// Callbacks
	OnMessageReceived      func(*protocol.DirectMessage)
	OnGroupMessageReceived func(*protocol.GroupMessage)
	OnProfileUpdate        func(*protocol.ProfileUpdate)
	OnTypingIndicator      func(*protocol.TypingIndicator)
	OnReadReceipt          func(*protocol.ReadReceipt)
	OnAckReceived          func(*protocol.AckMessage)
	OnNackReceived         func(*protocol.NackMessage)
}

// NewClient creates a new client
func NewClient(privateKey *rsa.PrivateKey) *Client {
	return &Client{
		PrivateKey:             privateKey,
		PublicKey:              &privateKey.PublicKey,
		oneTimePreKeys:         make(map[uint32]*protocol.OneTimePreKeyPrivate),
		ratchetSessions:        make(map[protocol.Address]*protocol.RatchetState),
		keyBundleCache:         make(map[protocol.Address]*protocol.KeyBundle),
		sendSequenceNumbers:    make(map[protocol.Address]uint64),
		receiveSequenceNumbers: make(map[protocol.Address]uint64),
		messageBuffer:          make(map[protocol.Address]map[uint64]*protocol.DirectMessage),
		receivedMessageIDs:     make(map[protocol.Address]map[uint64]bool),
	}
}

// AttachDatabase attaches a message database for persistence
func (c *Client) AttachDatabase(db *storage.MessageDB) {
	c.messageDB = db
}

// GetRelayDiscovery returns the relay discovery manager
// Initializes it if not already created
func (c *Client) GetRelayDiscovery() *RelayDiscovery {
	if c.relayDiscovery == nil && c.dhtNode != nil {
		c.relayDiscovery = NewRelayDiscovery(c.dhtNode)
	}
	return c.relayDiscovery
}

// AttachSessionStorage attaches a session storage for X3DH and ratchet persistence
func (c *Client) AttachSessionStorage(storage *SessionStorage) {
	c.sessionStorage = storage
}

// LoadPersistedState loads X3DH state, ratchet sessions, and key bundles from disk
// This should be called after attaching storage to restore previous session state
func (c *Client) LoadPersistedState() error {
	if c.sessionStorage == nil {
		return nil // No storage attached, nothing to load
	}

	// Load X3DH state
	if err := c.loadX3DHState(); err != nil {
		log.Printf("⚠️  Failed to load X3DH state: %v", err)
		// Don't return error - continue with other loads
	}

	// Load ratchet sessions
	sessions, err := c.sessionStorage.LoadAllRatchetSessions()
	if err != nil {
		log.Printf("⚠️  Failed to load ratchet sessions: %v", err)
	} else if len(sessions) > 0 {
		// Convert string-keyed map to Address-keyed map
		for addrHex, session := range sessions {
			addrBytes, err := hex.DecodeString(addrHex)
			if err != nil {
				log.Printf("⚠️  Invalid address in persisted session: %s", addrHex)
				continue
			}
			var addr protocol.Address
			copy(addr[:], addrBytes)
			c.ratchetSessions[addr] = session
		}
		log.Printf("✅ Loaded %d ratchet sessions from storage", len(sessions))
	}

	// Load key bundle cache
	cache, err := c.sessionStorage.LoadKeyBundleCache()
	if err != nil {
		log.Printf("⚠️  Failed to load key bundle cache: %v", err)
	} else if len(cache) > 0 {
		c.keyBundleCache = cache
		log.Printf("✅ Loaded %d cached key bundles from storage", len(cache))
	}

	return nil
}

// ConnectToRelay connects to a relay server
func (c *Client) ConnectToRelay(relayAddress string) error {
	conn, err := net.Dial("tcp", relayAddress)
	if err != nil {
		return err
	}

	c.relayConn = conn
	c.relayAddress = relayAddress

	// Perform handshake
	if err := c.performHandshake(); err != nil {
		conn.Close()
		return err
	}

	c.connected = true
	log.Printf("Connected to relay %s", relayAddress)

	// Start receive loop with auto-reconnection
	go c.receiveLoopWithReconnect()

	// Start keepalive routine
	go c.keepaliveLoop()

	return nil
}

// Disconnect disconnects from relay
func (c *Client) Disconnect() error {
	if c.relayConn != nil {
		c.connected = false
		return c.relayConn.Close()
	}
	return nil
}

// performHandshake performs connection handshake
func (c *Client) performHandshake() error {
	// Export public key
	pubKeyPEM, err := crypto.ExportPublicKeyPEM(c.PublicKey)
	if err != nil {
		return err
	}

	// Create handshake message
	hs := &protocol.HandshakeMessage{
		ProtocolVersion: protocol.ProtocolVersion,
		Address:         c.Address,
		PublicKey:       pubKeyPEM,
		ClientType:      protocol.ClientTypeUser,
		Timestamp:       uint64(time.Now().Unix()),
	}

	payload := hs.Encode()

	// Create header
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeHandshake,
		Length:    uint32(len(payload)),
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send handshake
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(payload); err != nil {
		return err
	}

	// Wait for handshake ACK
	ackHeader, err := protocol.ReadHeader(c.relayConn)
	if err != nil {
		return err
	}

	if ackHeader.Type != protocol.MsgTypeHandshakeAck {
		return ErrHandshakeFailed
	}

	// Read and discard the ACK payload (relay's public key)
	if ackHeader.Length > 0 {
		payload := make([]byte, ackHeader.Length)
		if _, err := io.ReadFull(c.relayConn, payload); err != nil {
			return err
		}
		// Could decode and store relay's public key here if needed
	}

	log.Println("Handshake successful")
	return nil
}

// SendPing sends a ping to relay
func (c *Client) SendPing() error {
	if !c.connected {
		return ErrNotConnected
	}

	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypePing,
		Length:    0,
		Flags:     0,
		MessageID: protocol.GenerateMessageID(),
	}

	return protocol.WriteHeader(c.relayConn, header)
}

// IsConnected returns connection status
func (c *Client) IsConnected() bool {
	return c.connected
}

// GetRelayAddress returns connected relay address
func (c *Client) GetRelayAddress() string {
	return c.relayAddress
}
