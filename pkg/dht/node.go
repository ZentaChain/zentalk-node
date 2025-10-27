package dht

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Node represents a DHT node
type Node struct {
	ID           NodeID
	Address      string // IP:Port
	routingTable *RoutingTable
	storage      *Storage
	listener     net.Listener
	running      bool
	mu           sync.RWMutex

	// Ed25519 keys for signing DHT entries (security enhancement)
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey

	// Pending RPC requests
	pendingRequests map[string]chan *RPCMessage
	pendingMu       sync.RWMutex
}

// NewNode creates a new DHT node
func NewNode(id NodeID, address string) *Node {
	// Generate Ed25519 key pair for signing DHT entries
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Printf("⚠️  Failed to generate Ed25519 keys: %v (signing disabled)", err)
	}

	return &Node{
		ID:              id,
		Address:         address,
		routingTable:    NewRoutingTable(id),
		storage:         NewStorage(),
		pendingRequests: make(map[string]chan *RPCMessage),
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
	}
}

// Start starts the DHT node (listening for incoming connections)
func (n *Node) Start() error {
	listener, err := net.Listen("tcp", n.Address)
	if err != nil {
		return fmt.Errorf("failed to start DHT node: %w", err)
	}

	n.listener = listener
	n.running = true

	// Update address to the actual listening address
	// This is important when n.Address was "localhost:0" (random port)
	n.Address = listener.Addr().String()

	log.Printf("DHT node %s listening on %s", n.ID.String()[:8], n.Address)

	// Start background tasks
	go n.handleConnections()
	go n.expireRoutine()

	return nil
}

// Stop stops the DHT node
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.running = false
	if n.listener != nil {
		return n.listener.Close()
	}
	return nil
}

// handleConnections handles incoming connections
func (n *Node) handleConnections() {
	for n.running {
		conn, err := n.listener.Accept()
		if err != nil {
			if !n.running {
				return
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		go n.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read message
	decoder := json.NewDecoder(conn)
	var msg RPCMessage
	if err := decoder.Decode(&msg); err != nil {
		log.Printf("Failed to decode message: %v", err)
		return
	}

	// Update routing table with sender
	if msg.Sender != nil {
		n.routingTable.AddContact(msg.Sender)
	}

	// Handle message
	response := n.handleRPC(&msg)

	// Send response
	if response != nil {
		encoder := json.NewEncoder(conn)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Failed to send response: %v", err)
		}
	}
}

// handleRPC handles an RPC message and returns a response
func (n *Node) handleRPC(msg *RPCMessage) *RPCMessage {
	sender := NewContact(n.ID, n.Address)

	switch msg.Type {
	case RPCPing:
		return n.handlePing(msg, sender)

	case RPCStore:
		return n.handleStore(msg, sender)

	case RPCFindNode:
		return n.handleFindNode(msg, sender)

	case RPCFindValue:
		return n.handleFindValue(msg, sender)

	default:
		log.Printf("Unknown RPC type: %v", msg.Type)
		return nil
	}
}

// handlePing handles a PING request
func (n *Node) handlePing(msg *RPCMessage, sender *Contact) *RPCMessage {
	response := &PongResponse{}
	rpcMsg, _ := NewRPCMessage(RPCPong, sender, response)
	rpcMsg.RequestID = msg.RequestID
	return rpcMsg
}

// handleStore handles a STORE request with signature verification
func (n *Node) handleStore(msg *RPCMessage, sender *Contact) *RPCMessage {
	var req StoreRequest
	if err := ParsePayload(msg, &req); err != nil {
		log.Printf("Failed to parse STORE request: %v", err)
		return nil
	}

	// Verify signature before storing (prevents DHT poisoning attacks)
	_, err := VerifyAndExtract(req.Value)
	if err != nil {
		log.Printf("⚠️  Rejected unsigned/invalid STORE from %s: %v", sender.ID.String()[:8], err)
		response := &StoreAckResponse{
			Success: false,
			Message: "signature verification failed",
		}
		rpcMsg, _ := NewRPCMessage(RPCStoreAck, sender, response)
		rpcMsg.RequestID = msg.RequestID
		return rpcMsg
	}

	// Store the signed value
	ttl := time.Duration(req.TTL) * time.Second
	n.storage.Store(req.Key, req.Value, ttl, msg.Sender.ID)

	log.Printf("✅ Stored verified key %s (TTL: %v)", req.Key.String()[:8], ttl)

	// Send acknowledgment
	response := &StoreAckResponse{Success: true}
	rpcMsg, _ := NewRPCMessage(RPCStoreAck, sender, response)
	rpcMsg.RequestID = msg.RequestID
	return rpcMsg
}

// handleFindNode handles a FIND_NODE request
func (n *Node) handleFindNode(msg *RPCMessage, sender *Contact) *RPCMessage {
	var req FindNodeRequest
	if err := ParsePayload(msg, &req); err != nil {
		log.Printf("Failed to parse FIND_NODE request: %v", err)
		return nil
	}

	// Find k closest nodes
	contacts := n.routingTable.FindClosest(req.Target, K)

	response := &FindNodeResponse{Contacts: contacts}
	rpcMsg, _ := NewRPCMessage(RPCResponse, sender, response)
	rpcMsg.RequestID = msg.RequestID
	return rpcMsg
}

// handleFindValue handles a FIND_VALUE request
func (n *Node) handleFindValue(msg *RPCMessage, sender *Contact) *RPCMessage {
	var req FindValueRequest
	if err := ParsePayload(msg, &req); err != nil {
		log.Printf("Failed to parse FIND_VALUE request: %v", err)
		return nil
	}

	// Check if we have the value
	value, found := n.storage.Get(req.Key)

	var response interface{}
	if found {
		// Return the value
		response = &FindValueResponse{
			Found: true,
			Value: value,
		}
	} else {
		// Return k closest nodes
		contacts := n.routingTable.FindClosest(req.Key, K)
		response = &FindValueResponse{
			Found:    false,
			Contacts: contacts,
		}
	}

	rpcMsg, _ := NewRPCMessage(RPCResponse, sender, response)
	rpcMsg.RequestID = msg.RequestID
	return rpcMsg
}

// expireRoutine periodically removes expired values
func (n *Node) expireRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for n.running {
		<-ticker.C
		n.storage.ExpireOldValues()
	}
}

// sendRPC sends an RPC message to a target node and waits for response
func (n *Node) sendRPC(target *Contact, msg *RPCMessage) (*RPCMessage, error) {
	conn, err := net.DialTimeout("tcp", target.Address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", target.Address, err)
	}
	defer conn.Close()

	// Set read/write deadlines
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for response
	decoder := json.NewDecoder(conn)
	var response RPCMessage
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return &response, nil
}

// GetRoutingTable returns the routing table (for debugging)
func (n *Node) GetRoutingTable() *RoutingTable {
	return n.routingTable
}

// GetStorage returns the storage (for debugging)
func (n *Node) GetStorage() *Storage {
	return n.storage
}
