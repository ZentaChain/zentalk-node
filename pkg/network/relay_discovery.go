package network

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/dht"
	"github.com/zentalk/protocol/pkg/protocol"
)

// RelayDiscovery manages relay discovery via DHT
type RelayDiscovery struct {
	dhtNode       *dht.Node
	knownRelays   map[protocol.Address]*RelayMetadata
	relayHealth   map[protocol.Address]*RelayHealthInfo
	blacklist     map[protocol.Address]time.Time // Blacklisted relays with expiry time
	mu            sync.RWMutex
	lastRefresh   time.Time
	refreshPeriod time.Duration
}

// RelayHealthInfo tracks health metrics for a relay
type RelayHealthInfo struct {
	LastPing        time.Time
	PingCount       int
	FailureCount    int
	SuccessCount    int
	AverageLatency  time.Duration
	LastError       error
	ConsecutiveFails int
}

// NewRelayDiscovery creates a new relay discovery manager
func NewRelayDiscovery(dhtNode *dht.Node) *RelayDiscovery {
	return &RelayDiscovery{
		dhtNode:       dhtNode,
		knownRelays:   make(map[protocol.Address]*RelayMetadata),
		relayHealth:   make(map[protocol.Address]*RelayHealthInfo),
		blacklist:     make(map[protocol.Address]time.Time),
		refreshPeriod: 5 * time.Minute,
	}
}

// PublishRelay publishes a relay's metadata to the DHT
func (rd *RelayDiscovery) PublishRelay(metadata *RelayMetadata) error {
	if rd.dhtNode == nil {
		return fmt.Errorf("DHT node not initialized")
	}

	// Update last seen timestamp
	metadata.LastSeen = time.Now().Unix()

	// Serialize metadata
	data, err := metadata.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode relay metadata: %w", err)
	}

	// Publish to DHT using relay's address as the key
	dhtKey := dht.NewNodeID(metadata.Address[:])

	// Use 24-hour TTL for relay metadata
	if err := rd.dhtNode.Store(dhtKey, data, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to publish to DHT: %w", err)
	}

	log.Printf("✅ Relay published to DHT: %s (key: %s...)", metadata.NetworkAddress, dhtKey.String()[:16])

	// Also add to known relays
	rd.mu.Lock()
	rd.knownRelays[metadata.Address] = metadata
	rd.mu.Unlock()

	return nil
}

// DiscoverRelays discovers N relays from the DHT
func (rd *RelayDiscovery) DiscoverRelays(count int) ([]*RelayMetadata, error) {
	if rd.dhtNode == nil {
		return nil, fmt.Errorf("DHT node not initialized")
	}

	// Check if we need to refresh
	if time.Since(rd.lastRefresh) > rd.refreshPeriod {
		if err := rd.refreshRelayCache(); err != nil {
			log.Printf("⚠️  Failed to refresh relay cache: %v", err)
		}
	}

	rd.mu.RLock()
	defer rd.mu.RUnlock()

	// Filter healthy relays not on blacklist
	available := make([]*RelayMetadata, 0)
	for addr, meta := range rd.knownRelays {
		// Check blacklist
		if expiry, blacklisted := rd.blacklist[addr]; blacklisted {
			if time.Now().Before(expiry) {
				continue // Still blacklisted
			} else {
				// Expired, remove from blacklist
				delete(rd.blacklist, addr)
			}
		}

		// Check health
		if meta.IsHealthy(1 * time.Hour) {
			available = append(available, meta)
		}
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no healthy relays available")
	}

	// If requesting more than available, return all
	if count >= len(available) {
		return available, nil
	}

	// Randomly select 'count' relays
	selected := make([]*RelayMetadata, 0, count)
	used := make(map[int]bool)

	for len(selected) < count {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(available))))
		i := int(idx.Int64())
		if !used[i] {
			selected = append(selected, available[i])
			used[i] = true
		}
	}

	return selected, nil
}

// DiscoverRelaysByRegion discovers relays in a specific geographic region
func (rd *RelayDiscovery) DiscoverRelaysByRegion(region string, count int) ([]*RelayMetadata, error) {
	if rd.dhtNode == nil {
		return nil, fmt.Errorf("DHT node not initialized")
	}

	rd.mu.RLock()
	defer rd.mu.RUnlock()

	// Filter relays by region
	regional := make([]*RelayMetadata, 0)
	for addr, meta := range rd.knownRelays {
		// Check blacklist
		if expiry, blacklisted := rd.blacklist[addr]; blacklisted && time.Now().Before(expiry) {
			continue
		}

		// Check region and health
		if meta.Region == region && meta.IsHealthy(1*time.Hour) {
			regional = append(regional, meta)
		}
	}

	if len(regional) == 0 {
		return nil, fmt.Errorf("no healthy relays in region %s", region)
	}

	// If requesting more than available, return all
	if count >= len(regional) {
		return regional, nil
	}

	// Randomly select 'count' relays
	selected := make([]*RelayMetadata, 0, count)
	used := make(map[int]bool)

	for len(selected) < count {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(regional))))
		i := int(idx.Int64())
		if !used[i] {
			selected = append(selected, regional[i])
			used[i] = true
		}
	}

	return selected, nil
}

// SelectOptimalCircuit selects the best circuit of N hops
func (rd *RelayDiscovery) SelectOptimalCircuit(hopCount int) ([]*crypto.RelayInfo, error) {
	if hopCount < 1 {
		return nil, fmt.Errorf("hop count must be at least 1")
	}

	// Get available relays
	relays, err := rd.DiscoverRelays(hopCount * 3) // Get 3x to have options
	if err != nil {
		return nil, err
	}

	if len(relays) < hopCount {
		return nil, fmt.Errorf("not enough relays available: need %d, have %d", hopCount, len(relays))
	}

	// Score each relay
	scores := make([]*RelayScore, len(relays))
	for i, relay := range relays {
		scores[i] = &RelayScore{
			Metadata: relay,
			Score:    relay.CalculateScore(),
		}
	}

	// Sort by score (highest first)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Select top relays with diversity
	selected := make([]*RelayScore, 0, hopCount)
	usedOperators := make(map[string]bool)
	usedRegions := make(map[string]bool)

	for _, relay := range scores {
		if len(selected) >= hopCount {
			break
		}

		// Prefer diversity in operators and regions
		operatorUsed := usedOperators[relay.Metadata.Operator]
		regionUsed := usedRegions[relay.Metadata.Region]

		// If we have choices left, prefer diverse selection
		if len(selected) < hopCount-1 {
			if operatorUsed && regionUsed {
				continue // Skip if both operator and region already used
			}
		}

		selected = append(selected, relay)
		usedOperators[relay.Metadata.Operator] = true
		usedRegions[relay.Metadata.Region] = true
	}

	// Convert to RelayInfo
	circuit := make([]*crypto.RelayInfo, len(selected))
	for i, relay := range selected {
		// Parse public key
		pubKey, err := crypto.ImportPublicKeyPEM([]byte(relay.Metadata.PublicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("failed to parse relay public key: %w", err)
		}

		circuit[i] = &crypto.RelayInfo{
			Address:   relay.Metadata.Address,
			PublicKey: pubKey,
		}
	}

	log.Printf("✅ Selected optimal circuit with %d hops (diversity: %d operators, %d regions)",
		len(circuit), len(usedOperators), len(usedRegions))

	return circuit, nil
}

// BlacklistRelay blacklists a relay for a duration
func (rd *RelayDiscovery) BlacklistRelay(addr protocol.Address, duration time.Duration) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.blacklist[addr] = time.Now().Add(duration)
	log.Printf("⚫ Relay %x blacklisted for %v", addr[:8], duration)
}

// UpdateRelayHealth updates health information for a relay
func (rd *RelayDiscovery) UpdateRelayHealth(addr protocol.Address, success bool, latency time.Duration, err error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	health, exists := rd.relayHealth[addr]
	if !exists {
		health = &RelayHealthInfo{}
		rd.relayHealth[addr] = health
	}

	health.LastPing = time.Now()
	health.PingCount++

	if success {
		health.SuccessCount++
		health.ConsecutiveFails = 0

		// Update average latency (exponential moving average)
		if health.AverageLatency == 0 {
			health.AverageLatency = latency
		} else {
			health.AverageLatency = (health.AverageLatency*9 + latency) / 10
		}
	} else {
		health.FailureCount++
		health.ConsecutiveFails++
		health.LastError = err

		// Blacklist after 3 consecutive failures
		if health.ConsecutiveFails >= 3 {
			rd.blacklist[addr] = time.Now().Add(10 * time.Minute)
			log.Printf("⚫ Relay %x auto-blacklisted (3 consecutive failures)", addr[:8])
		}
	}
}

// refreshRelayCache refreshes the relay cache from DHT
func (rd *RelayDiscovery) refreshRelayCache() error {
	if rd.dhtNode == nil {
		return fmt.Errorf("DHT node not initialized")
	}

	log.Printf("🔄 Refreshing relay cache from DHT...")

	// In a real implementation, we would:
	// 1. Query DHT for a "relay directory" key or use a known relay bootstrap list
	// 2. Or perform iterative random walks through DHT to discover relay keys
	// 3. For now, we rely on relays being published and clients caching them

	rd.lastRefresh = time.Now()
	return nil
}

// GetKnownRelays returns all currently known relays
func (rd *RelayDiscovery) GetKnownRelays() []*RelayMetadata {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	relays := make([]*RelayMetadata, 0, len(rd.knownRelays))
	for _, meta := range rd.knownRelays {
		relays = append(relays, meta)
	}
	return relays
}

// GetRelayCount returns the number of known relays
func (rd *RelayDiscovery) GetRelayCount() int {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	return len(rd.knownRelays)
}

// AddKnownRelay manually adds a relay to the known relays cache
// Useful for bootstrapping or testing
func (rd *RelayDiscovery) AddKnownRelay(metadata *RelayMetadata) {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	rd.knownRelays[metadata.Address] = metadata
}
