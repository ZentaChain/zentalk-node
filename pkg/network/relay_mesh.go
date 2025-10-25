package network

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zentalk/protocol/pkg/protocol"
)

// BootstrapRelay represents a hardcoded bootstrap relay for initial mesh connectivity
type BootstrapRelay struct {
	Address        protocol.Address
	NetworkAddress string // host:port
	Region         string
}

// Default bootstrap relays - these should be updated with real relay addresses
var DefaultBootstrapRelays = []BootstrapRelay{
	// These are placeholder addresses - in production, these would be actual relay servers
	// Operators should run these bootstrap relays to help new relays join the mesh
}

// MeshManager manages automatic relay mesh formation
type MeshManager struct {
	relay              *RelayServer
	bootstrapRelays    []BootstrapRelay
	targetPeerCount    int
	discoveryInterval  time.Duration
	connectionInterval time.Duration

	running            bool
	stopChan           chan struct{}
	mu                 sync.RWMutex
}

// NewMeshManager creates a new mesh manager for a relay
func NewMeshManager(relay *RelayServer, targetPeerCount int) *MeshManager {
	return &MeshManager{
		relay:              relay,
		bootstrapRelays:    DefaultBootstrapRelays,
		targetPeerCount:    targetPeerCount,
		discoveryInterval:  5 * time.Minute,  // Discover new relays every 5 minutes
		connectionInterval: 30 * time.Second,  // Check connections every 30 seconds
		stopChan:           make(chan struct{}),
	}
}

// SetBootstrapRelays sets custom bootstrap relays
func (mm *MeshManager) SetBootstrapRelays(relays []BootstrapRelay) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.bootstrapRelays = relays
}

// AddBootstrapRelay adds a single bootstrap relay
func (mm *MeshManager) AddBootstrapRelay(relay BootstrapRelay) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.bootstrapRelays = append(mm.bootstrapRelays, relay)
}

// Start starts the auto-mesh formation process
func (mm *MeshManager) Start() error {
	mm.mu.Lock()
	if mm.running {
		mm.mu.Unlock()
		return fmt.Errorf("mesh manager already running")
	}
	mm.running = true
	mm.mu.Unlock()

	log.Printf("🌐 Starting auto-mesh formation (target: %d peers)", mm.targetPeerCount)

	// Connect to bootstrap relays immediately
	go mm.connectToBootstrapRelays()

	// Start discovery loop
	go mm.discoveryLoop()

	// Start connection maintenance loop
	go mm.connectionMaintenanceLoop()

	return nil
}

// Stop stops the auto-mesh formation process
func (mm *MeshManager) Stop() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if !mm.running {
		return
	}

	mm.running = false
	close(mm.stopChan)
	log.Println("🛑 Stopped auto-mesh formation")
}

// connectToBootstrapRelays attempts to connect to all bootstrap relays
func (mm *MeshManager) connectToBootstrapRelays() {
	mm.mu.RLock()
	bootstraps := make([]BootstrapRelay, len(mm.bootstrapRelays))
	copy(bootstraps, mm.bootstrapRelays)
	mm.mu.RUnlock()

	if len(bootstraps) == 0 {
		log.Println("⚠️  No bootstrap relays configured")
		return
	}

	log.Printf("🔗 Connecting to %d bootstrap relays...", len(bootstraps))

	for _, bootstrap := range bootstraps {
		// Check if already connected
		mm.relay.mu.RLock()
		_, exists := mm.relay.peers[string(bootstrap.Address[:])]
		mm.relay.mu.RUnlock()

		if exists {
			log.Printf("✓ Already connected to bootstrap relay: %s", bootstrap.NetworkAddress)
			continue
		}

		// Attempt connection
		if err := mm.relay.ConnectToRelay(bootstrap.NetworkAddress, bootstrap.Address); err != nil {
			log.Printf("⚠️  Failed to connect to bootstrap relay %s: %v", bootstrap.NetworkAddress, err)
		} else {
			log.Printf("✅ Connected to bootstrap relay: %s", bootstrap.NetworkAddress)

			// Add to relay discovery cache
			if mm.relay.relayDiscovery != nil {
				metadata := &RelayMetadata{
					Address:        bootstrap.Address,
					NetworkAddress: bootstrap.NetworkAddress,
					Region:         bootstrap.Region,
					LastSeen:       time.Now().Unix(),
				}
				mm.relay.relayDiscovery.AddKnownRelay(metadata)
			}
		}
	}
}

// discoveryLoop periodically discovers new relays from DHT
func (mm *MeshManager) discoveryLoop() {
	ticker := time.NewTicker(mm.discoveryInterval)
	defer ticker.Stop()

	// Initial discovery
	mm.discoverAndConnect()

	for {
		select {
		case <-ticker.C:
			mm.discoverAndConnect()
		case <-mm.stopChan:
			return
		}
	}
}

// connectionMaintenanceLoop maintains target peer count
func (mm *MeshManager) connectionMaintenanceLoop() {
	ticker := time.NewTicker(mm.connectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.maintainConnections()
		case <-mm.stopChan:
			return
		}
	}
}

// discoverAndConnect discovers new relays and connects to them
func (mm *MeshManager) discoverAndConnect() {
	if mm.relay.relayDiscovery == nil {
		log.Println("⚠️  DHT discovery not available, skipping relay discovery")
		return
	}

	// Get current peer count
	mm.relay.mu.RLock()
	currentPeerCount := len(mm.relay.peers)
	mm.relay.mu.RUnlock()

	if currentPeerCount >= mm.targetPeerCount {
		log.Printf("✓ Mesh fully connected: %d/%d peers", currentPeerCount, mm.targetPeerCount)
		return
	}

	needed := mm.targetPeerCount - currentPeerCount
	log.Printf("🔍 Discovering %d new relays...", needed)

	// Discover relays
	relays, err := mm.relay.relayDiscovery.DiscoverRelays(needed * 2) // Get 2x for redundancy
	if err != nil {
		log.Printf("⚠️  Relay discovery failed: %v", err)
		return
	}

	if len(relays) == 0 {
		log.Println("⚠️  No relays discovered")
		return
	}

	log.Printf("📡 Discovered %d relays from DHT", len(relays))

	// Attempt to connect to discovered relays
	connected := 0
	for _, relay := range relays {
		// Check if we've reached target
		mm.relay.mu.RLock()
		currentCount := len(mm.relay.peers)
		mm.relay.mu.RUnlock()

		if currentCount >= mm.targetPeerCount {
			break
		}

		// Check if already connected
		mm.relay.mu.RLock()
		_, exists := mm.relay.peers[string(relay.Address[:])]
		mm.relay.mu.RUnlock()

		if exists {
			continue
		}

		// Don't connect to ourselves
		if relay.Address == mm.relay.Address {
			continue
		}

		// Attempt connection
		log.Printf("🔗 Connecting to relay: %s (%s)", relay.NetworkAddress, relay.Region)
		if err := mm.relay.ConnectToRelay(relay.NetworkAddress, relay.Address); err != nil {
			log.Printf("⚠️  Connection failed: %v", err)
		} else {
			connected++
			log.Printf("✅ Connected to relay: %s", relay.NetworkAddress)
		}

		// Small delay between connections
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("✓ Connected to %d new relays", connected)
}

// maintainConnections ensures we maintain target peer count
func (mm *MeshManager) maintainConnections() {
	mm.relay.mu.RLock()
	currentPeerCount := len(mm.relay.peers)
	relayPeers := make([]protocol.Address, 0)

	// Collect relay-type peers
	for addr, peer := range mm.relay.peers {
		if peer.ClientType == protocol.ClientTypeRelay {
			var peerAddr protocol.Address
			copy(peerAddr[:], addr)
			relayPeers = append(relayPeers, peerAddr)
		}
	}
	mm.relay.mu.RUnlock()

	relayPeerCount := len(relayPeers)

	if relayPeerCount < mm.targetPeerCount {
		log.Printf("⚠️  Mesh connectivity low: %d/%d relay peers", relayPeerCount, mm.targetPeerCount)
		// Trigger immediate discovery
		go mm.discoverAndConnect()
	} else {
		log.Printf("✓ Mesh healthy: %d relay peers, %d total peers", relayPeerCount, currentPeerCount)
	}

	// TODO: Ping relay peers to check if they're still alive
	// If any are dead, remove them and trigger discovery
}

// GetMeshStatus returns current mesh status
func (mm *MeshManager) GetMeshStatus() map[string]interface{} {
	mm.relay.mu.RLock()
	defer mm.relay.mu.RUnlock()

	relayPeers := 0
	clientPeers := 0

	for _, peer := range mm.relay.peers {
		if peer.ClientType == protocol.ClientTypeRelay {
			relayPeers++
		} else {
			clientPeers++
		}
	}

	return map[string]interface{}{
		"relay_peers":   relayPeers,
		"client_peers":  clientPeers,
		"total_peers":   len(mm.relay.peers),
		"target_peers":  mm.targetPeerCount,
		"mesh_healthy":  relayPeers >= mm.targetPeerCount,
	}
}
