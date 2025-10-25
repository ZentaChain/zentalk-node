package network

import (
	"crypto/rsa"
	"errors"
	"sync"
	"time"

	"github.com/zentalk/protocol/pkg/protocol"
)

var (
	ErrPoolClosed = errors.New("connection pool closed")
	ErrNoRelays   = errors.New("no relays available")
)

// RelayInfo represents relay metadata
type RelayInfo struct {
	Address    protocol.Address `json:"address"`
	Endpoint   string           `json:"endpoint"`    // e.g., "relay1.zentalk.io:8080"
	PublicKey  *rsa.PublicKey   `json:"-"`           // Public key (not serialized)
	PublicKeyPEM string         `json:"public_key"` // PEM format for serialization
	IsActive   bool             `json:"online"`      // Online/active status
	Region     string           `json:"region"`      // e.g., "us-east", "eu-west"
	Reputation int              `json:"reputation"`  // 0-100 score
}

// ConnectionPool manages connections to multiple relays
type ConnectionPool struct {
	relays map[string]*RelayInfo
	clients map[string]*Client
	mu      sync.RWMutex

	privateKey *rsa.PrivateKey
	maxConns   int
	closed     bool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(privateKey *rsa.PrivateKey, maxConns int) *ConnectionPool {
	return &ConnectionPool{
		relays:     make(map[string]*RelayInfo),
		clients:    make(map[string]*Client),
		privateKey: privateKey,
		maxConns:   maxConns,
	}
}

// AddRelay adds a relay to the pool
func (p *ConnectionPool) AddRelay(info *RelayInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.relays[info.Endpoint] = info
}

// RemoveRelay removes a relay from the pool
func (p *ConnectionPool) RemoveRelay(endpoint string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.relays, endpoint)

	// Close existing connection
	if client, exists := p.clients[endpoint]; exists {
		client.Disconnect()
		delete(p.clients, endpoint)
	}
}

// GetClient gets or creates a client connection to a relay
func (p *ConnectionPool) GetClient(endpoint string) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrPoolClosed
	}

	// Check if we already have a connection
	if client, exists := p.clients[endpoint]; exists && client.IsConnected() {
		return client, nil
	}

	// Check connection limit
	if len(p.clients) >= p.maxConns {
		// Remove oldest connection
		p.evictOldest()
	}

	// Create new client
	client := NewClient(p.privateKey)

	// Connect to relay
	if err := client.ConnectToRelay(endpoint); err != nil {
		return nil, err
	}

	p.clients[endpoint] = client
	return client, nil
}

// GetRandomRelay returns a random active relay
func (p *ConnectionPool) GetRandomRelay() (*RelayInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Filter active relays
	var active []*RelayInfo
	for _, relay := range p.relays {
		if relay.IsActive {
			active = append(active, relay)
		}
	}

	if len(active) == 0 {
		return nil, ErrNoRelays
	}

	// Return random (simple version - just return first)
	// In production, use crypto/rand for randomness
	return active[0], nil
}

// GetRandomPath selects random relays for onion routing
func (p *ConnectionPool) GetRandomPath(hops int) ([]*RelayInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Filter active relays
	var active []*RelayInfo
	for _, relay := range p.relays {
		if relay.IsActive {
			active = append(active, relay)
		}
	}

	if len(active) < hops {
		return nil, errors.New("not enough active relays")
	}

	// Select random path (simple version - just take first N)
	// In production, implement proper random selection with Fisher-Yates
	path := make([]*RelayInfo, hops)
	copy(path, active[:hops])

	return path, nil
}

// UpdateRelayStatus updates relay active status
func (p *ConnectionPool) UpdateRelayStatus(endpoint string, isActive bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if relay, exists := p.relays[endpoint]; exists {
		relay.IsActive = isActive

		// If inactive, close connection
		if !isActive {
			if client, exists := p.clients[endpoint]; exists {
				client.Disconnect()
				delete(p.clients, endpoint)
			}
		}
	}
}

// GetStats returns pool statistics
func (p *ConnectionPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	activeRelays := 0
	for _, relay := range p.relays {
		if relay.IsActive {
			activeRelays++
		}
	}

	return map[string]interface{}{
		"total_relays":       len(p.relays),
		"active_relays":      activeRelays,
		"active_connections": len(p.clients),
		"max_connections":    p.maxConns,
	}
}

// Close closes all connections and shuts down the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPoolClosed
	}

	p.closed = true

	// Close all client connections
	for _, client := range p.clients {
		client.Disconnect()
	}

	p.clients = make(map[string]*Client)
	return nil
}

// evictOldest evicts the oldest connection (must be called with lock held)
func (p *ConnectionPool) evictOldest() {
	// Simple eviction - just remove first one
	// In production, track last used time and evict LRU
	for endpoint, client := range p.clients {
		client.Disconnect()
		delete(p.clients, endpoint)
		break
	}
}

// PingAll sends ping to all connected relays
func (p *ConnectionPool) PingAll() {
	p.mu.RLock()
	clients := make([]*Client, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client)
	}
	p.mu.RUnlock()

	for _, client := range clients {
		go client.SendPing()
	}
}

// StartHealthCheck starts periodic health checks
func (p *ConnectionPool) StartHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if p.closed {
				ticker.Stop()
				return
			}
			p.PingAll()
		}
	}()
}

// Example usage:
//
// // Create pool
// privateKey, _ := crypto.GenerateRSAKeyPair()
// pool := NewConnectionPool(privateKey, 10)
//
// // Add relays from blockchain
// for _, relay := range activeRelays {
//     pool.AddRelay(&RelayInfo{
//         Endpoint: relay.Endpoint,
//         PublicKey: relay.PublicKey,
//         IsActive: true,
//     })
// }
//
// // Send message via pool
// path, _ := pool.GetRandomPath(3)
// client, _ := pool.GetClient(path[0].Endpoint)
// client.SendMessage(recipient, "Hello!", publicKeysFromPath(path))
//
// // Start health checks
// pool.StartHealthCheck(30 * time.Second)
