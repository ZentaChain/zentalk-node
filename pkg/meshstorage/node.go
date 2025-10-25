// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
)

// DHTNode represents a node in the distributed hash table
type DHTNode struct {
	host      host.Host
	// Note: IpfsDHT is libp2p's Kademlia DHT implementation (NOT IPFS storage)
	// The name is historical - it was created for IPFS but is a generic DHT for peer discovery
	dht       *dht.IpfsDHT
	ctx       context.Context
	cancel    context.CancelFunc
	storage   *LocalStorage
	mu        sync.RWMutex
	peers     map[peer.ID]*PeerInfo
	bootstrapped bool
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID        peer.ID
	Addresses []multiaddr.Multiaddr
	LastSeen  time.Time
	Active    bool
}

// NodeConfig contains configuration for creating a DHT node
type NodeConfig struct {
	Port          int
	DataDir       string
	BootstrapPeers []string
	PrivateKey    crypto.PrivKey // Optional: provide your own key
}

// NewDHTNode creates a new DHT node
func NewDHTNode(ctx context.Context, config *NodeConfig) (*DHTNode, error) {
	// Generate or use provided private key
	var priv crypto.PrivKey
	var err error

	if config.PrivateKey != nil {
		priv = config.PrivateKey
	} else {
		// Generate new Ed25519 key pair
		priv, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}
	}

	// Create libp2p host
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.Port)

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Create DHT
	dhtInst, err := dht.New(ctx, h,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
	)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	// Create local storage
	storage, err := NewLocalStorage(config.DataDir)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	nodeCtx, cancel := context.WithCancel(ctx)

	node := &DHTNode{
		host:      h,
		dht:       dhtInst,
		ctx:       nodeCtx,
		cancel:    cancel,
		storage:   storage,
		peers:     make(map[peer.ID]*PeerInfo),
		bootstrapped: false,
	}

	// Bootstrap DHT if peers provided
	if len(config.BootstrapPeers) > 0 {
		if err := node.Bootstrap(config.BootstrapPeers); err != nil {
			node.Close()
			return nil, fmt.Errorf("failed to bootstrap: %w", err)
		}
	}

	// Start peer monitoring
	go node.monitorPeers()

	return node, nil
}

// Bootstrap connects to bootstrap peers and joins the DHT network
func (n *DHTNode) Bootstrap(bootstrapPeers []string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.bootstrapped {
		return fmt.Errorf("already bootstrapped")
	}

	// Parse and connect to bootstrap peers
	var connectedCount int
	for _, peerStr := range bootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(peerStr)
		if err != nil {
			fmt.Printf("Invalid bootstrap peer address %s: %v\n", peerStr, err)
			continue
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			fmt.Printf("Failed to parse peer info from %s: %v\n", peerStr, err)
			continue
		}

		// Connect to the peer
		if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", peerInfo.ID, err)
			continue
		}

		fmt.Printf("Connected to bootstrap peer: %s\n", peerInfo.ID)

		// Track peer
		n.peers[peerInfo.ID] = &PeerInfo{
			ID:        peerInfo.ID,
			Addresses: peerInfo.Addrs,
			LastSeen:  time.Now(),
			Active:    true,
		}

		connectedCount++
	}

	if connectedCount == 0 {
		return fmt.Errorf("failed to connect to any bootstrap peers")
	}

	// Bootstrap the DHT
	if err := n.dht.Bootstrap(n.ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	n.bootstrapped = true
	fmt.Printf("Successfully bootstrapped with %d peers\n", connectedCount)

	return nil
}

// FindClosestNodes finds the N closest nodes to a given key using Kademlia XOR distance
func (n *DHTNode) FindClosestNodes(ctx context.Context, key string, count int) ([]peer.AddrInfo, error) {
	// Hash the key to get a consistent byte representation for distance calculation
	keyHash := hashKey(key)

	// Get connected peers from the host
	connectedPeers := n.host.Network().Peers()

	// Calculate distances for all connected peers
	var peerDistances []peerDistance
	for _, pID := range connectedPeers {
		// Get peer addresses
		addrs := n.host.Peerstore().Addrs(pID)
		if len(addrs) == 0 {
			continue
		}

		// Calculate XOR distance between key hash and peer ID
		dist := xorDistanceBytes(keyHash, []byte(pID))

		peerDistances = append(peerDistances, peerDistance{
			info: peer.AddrInfo{
				ID:    pID,
				Addrs: addrs,
			},
			distance: dist,
		})
	}

	// Sort by XOR distance (closest first)
	sortPeersByDistance(peerDistances)

	// Return the N closest peers
	var result []peer.AddrInfo
	for i := 0; i < len(peerDistances) && i < count; i++ {
		result = append(result, peerDistances[i].info)
	}

	return result, nil
}

// GetRoutingTable returns the DHT routing table for debugging
func (n *DHTNode) GetRoutingTable() routing.Routing {
	return n.dht
}

// ID returns the node's peer ID
func (n *DHTNode) ID() peer.ID {
	return n.host.ID()
}

// Addresses returns the node's listen addresses
func (n *DHTNode) Addresses() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

// GetPeers returns information about connected peers
func (n *DHTNode) GetPeers() map[peer.ID]*PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Create a copy to avoid race conditions
	peers := make(map[peer.ID]*PeerInfo)
	for id, info := range n.peers {
		peerCopy := *info
		peers[id] = &peerCopy
	}

	return peers
}

// PeerCount returns the number of connected peers
func (n *DHTNode) PeerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.peers)
}

// monitorPeers periodically checks peer connectivity
func (n *DHTNode) monitorPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.checkPeerHealth()
		}
	}
}

// checkPeerHealth verifies peer connectivity and updates peer info
func (n *DHTNode) checkPeerHealth() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Get currently connected peers from host
	connectedPeers := n.host.Network().Peers()
	activePeers := make(map[peer.ID]bool)

	for _, pID := range connectedPeers {
		activePeers[pID] = true

		// Update or add peer info
		if peerInfo, exists := n.peers[pID]; exists {
			peerInfo.LastSeen = time.Now()
			peerInfo.Active = true
		} else {
			addrs := n.host.Peerstore().Addrs(pID)
			n.peers[pID] = &PeerInfo{
				ID:        pID,
				Addresses: addrs,
				LastSeen:  time.Now(),
				Active:    true,
			}
		}
	}

	// Mark disconnected peers as inactive
	for pID, peerInfo := range n.peers {
		if !activePeers[pID] {
			peerInfo.Active = false
		}
	}
}

// Storage returns the local storage instance
func (n *DHTNode) Storage() *LocalStorage {
	return n.storage
}

// Host returns the libp2p host
func (n *DHTNode) Host() host.Host {
	return n.host
}

// DHT returns the DHT instance (libp2p Kademlia DHT, not IPFS storage)
func (n *DHTNode) DHT() *dht.IpfsDHT {
	return n.dht
}

// IsBootstrapped returns whether the node has successfully bootstrapped
func (n *DHTNode) IsBootstrapped() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.bootstrapped
}

// Connect connects to a peer given its multiaddr
func (n *DHTNode) Connect(ctx context.Context, peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return fmt.Errorf("invalid peer address: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to parse peer info: %w", err)
	}

	if err := n.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	n.mu.Lock()
	n.peers[peerInfo.ID] = &PeerInfo{
		ID:        peerInfo.ID,
		Addresses: peerInfo.Addrs,
		LastSeen:  time.Now(),
		Active:    true,
	}
	n.mu.Unlock()

	return nil
}

// Disconnect disconnects from a peer
func (n *DHTNode) Disconnect(peerID peer.ID) error {
	if err := n.host.Network().ClosePeer(peerID); err != nil {
		return fmt.Errorf("failed to disconnect from peer: %w", err)
	}

	n.mu.Lock()
	if peerInfo, exists := n.peers[peerID]; exists {
		peerInfo.Active = false
	}
	n.mu.Unlock()

	return nil
}

// GetNodeInfo returns information about this node
type NodeInfo struct {
	ID            string
	Addresses     []string
	PeerCount     int
	Bootstrapped  bool
	StorageStats  *StorageStats
}

func (n *DHTNode) GetNodeInfo() (*NodeInfo, error) {
	stats, err := n.storage.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	addrs := make([]string, len(n.host.Addrs()))
	for i, addr := range n.host.Addrs() {
		addrs[i] = addr.String()
	}

	return &NodeInfo{
		ID:           n.host.ID().String(),
		Addresses:    addrs,
		PeerCount:    n.PeerCount(),
		Bootstrapped: n.IsBootstrapped(),
		StorageStats: stats,
	}, nil
}

// Close gracefully shuts down the node
func (n *DHTNode) Close() error {
	n.cancel()

	// Close DHT
	if err := n.dht.Close(); err != nil {
		fmt.Printf("Error closing DHT: %v\n", err)
	}

	// Close host
	if err := n.host.Close(); err != nil {
		fmt.Printf("Error closing host: %v\n", err)
	}

	// Close storage
	if err := n.storage.Close(); err != nil {
		fmt.Printf("Error closing storage: %v\n", err)
	}

	return nil
}

// Kademlia XOR distance helper functions

// peerDistance represents a peer with its XOR distance from a key
type peerDistance struct {
	info     peer.AddrInfo
	distance []byte
}

// hashKey hashes a string key to get a consistent byte representation for distance calculations
func hashKey(key string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hasher.Sum(nil)
}

// xorDistanceBytes calculates the XOR distance between two byte arrays
// In Kademlia, distance is defined as XOR of the ID hashes
func xorDistanceBytes(bytes1, bytes2 []byte) []byte {
	// Determine the length (use the longer one, pad the shorter)
	maxLen := len(bytes1)
	if len(bytes2) > maxLen {
		maxLen = len(bytes2)
	}

	// Calculate XOR distance
	distance := make([]byte, maxLen)
	for i := 0; i < maxLen; i++ {
		var b1, b2 byte
		if i < len(bytes1) {
			b1 = bytes1[i]
		}
		if i < len(bytes2) {
			b2 = bytes2[i]
		}
		distance[i] = b1 ^ b2
	}

	return distance
}

// compareDistances compares two XOR distances
// Returns -1 if dist1 < dist2, 0 if equal, 1 if dist1 > dist2
func compareDistances(dist1, dist2 []byte) int {
	// Compare byte by byte (big-endian)
	minLen := len(dist1)
	if len(dist2) < minLen {
		minLen = len(dist2)
	}

	for i := 0; i < minLen; i++ {
		if dist1[i] < dist2[i] {
			return -1
		}
		if dist1[i] > dist2[i] {
			return 1
		}
	}

	// If all compared bytes are equal, compare lengths
	if len(dist1) < len(dist2) {
		return -1
	}
	if len(dist1) > len(dist2) {
		return 1
	}

	return 0
}

// sortPeersByDistance sorts peers by their XOR distance (ascending - closest first)
func sortPeersByDistance(peers []peerDistance) {
	// Simple bubble sort (efficient for small peer lists in Phase 2)
	// Can be optimized with quicksort for larger networks in Phase 3+
	n := len(peers)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if compareDistances(peers[j].distance, peers[j+1].distance) > 0 {
				// Swap
				peers[j], peers[j+1] = peers[j+1], peers[j]
			}
		}
	}
}
