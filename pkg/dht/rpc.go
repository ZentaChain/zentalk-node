package dht

import (
	"encoding/json"
	"fmt"
	"time"
)

// RPCType represents the type of RPC message
type RPCType uint8

const (
	RPCPing      RPCType = 0x01
	RPCPong      RPCType = 0x02
	RPCStore     RPCType = 0x03
	RPCStoreAck  RPCType = 0x04
	RPCFindNode  RPCType = 0x05
	RPCFindValue RPCType = 0x06
	RPCResponse  RPCType = 0x07
)

// RPCMessage represents a generic DHT RPC message
type RPCMessage struct {
	Type      RPCType       `json:"type"`
	RequestID string        `json:"request_id"`
	Sender    *Contact      `json:"sender"`
	Payload   []byte        `json:"payload"`
	Timestamp int64         `json:"timestamp"`
}

// NewRPCMessage creates a new RPC message
func NewRPCMessage(msgType RPCType, sender *Contact, payload interface{}) (*RPCMessage, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &RPCMessage{
		Type:      msgType,
		RequestID: generateRequestID(),
		Sender:    sender,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Encode encodes the RPC message to JSON
func (m *RPCMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeRPCMessage decodes an RPC message from JSON
func DecodeRPCMessage(data []byte) (*RPCMessage, error) {
	var msg RPCMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// --- PING/PONG ---

// PingRequest represents a PING request (keep-alive)
type PingRequest struct {
	// Empty - just checking if node is alive
}

// PongResponse represents a PONG response
type PongResponse struct {
	// Empty - acknowledging we're alive
}

// --- STORE ---

// StoreRequest represents a request to store a key-value pair
type StoreRequest struct {
	Key   NodeID `json:"key"`
	Value []byte `json:"value"`
	TTL   int64  `json:"ttl"` // Time-to-live in seconds
}

// StoreAckResponse represents acknowledgment of a store
type StoreAckResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// --- FIND_NODE ---

// FindNodeRequest represents a request to find k closest nodes
type FindNodeRequest struct {
	Target NodeID `json:"target"`
}

// FindNodeResponse represents a response with k closest nodes
type FindNodeResponse struct {
	Contacts []*Contact `json:"contacts"`
}

// --- FIND_VALUE ---

// FindValueRequest represents a request to find a value by key
type FindValueRequest struct {
	Key NodeID `json:"key"`
}

// FindValueResponse represents a response with either value or contacts
type FindValueResponse struct {
	Found    bool       `json:"found"`
	Value    []byte     `json:"value,omitempty"`
	Contacts []*Contact `json:"contacts,omitempty"`
}

// --- Helper Functions ---

// ParsePayload parses the payload of an RPC message into a specific type
func ParsePayload(msg *RPCMessage, target interface{}) error {
	return json.Unmarshal(msg.Payload, target)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), randInt())
}

func randInt() int {
	// Simple random for request IDs (not cryptographic)
	return int(time.Now().UnixNano() % 1000000)
}
