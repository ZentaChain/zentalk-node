package network

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ZentaChain/zentalk-node/pkg/protocol"
)

const (
	// NumGuardRelays is the number of guard relays to use (like Tor)
	NumGuardRelays = 3

	// GuardRotationPeriod is how often to rotate guard relays (default: 2-3 months)
	GuardRotationPeriod = 60 * 24 * time.Hour // 60 days

	// GuardMinUptime is minimum uptime required to become a guard (7 days)
	GuardMinUptime = 7 * 24 * time.Hour
)

// GuardRelay represents a long-lived entry relay
type GuardRelay struct {
	Metadata     *RelayMetadata
	FirstUsed    time.Time // When we first started using this guard
	LastUsed     time.Time // Last time we used this guard
	SuccessCount int       // Successful connections
	FailCount    int       // Failed connections
}

// GuardRelayManager manages persistent guard relays (entry nodes)
// This prevents adversaries from running entry nodes to correlate users
type GuardRelayManager struct {
	guards         []*GuardRelay
	mu             sync.RWMutex
	rotationTime   time.Time
	discovery      *RelayDiscovery
	persistPath    string // Optional: path to persist guards
}

// NewGuardRelayManager creates a new guard relay manager
func NewGuardRelayManager(discovery *RelayDiscovery) *GuardRelayManager {
	return &GuardRelayManager{
		guards:       make([]*GuardRelay, 0, NumGuardRelays),
		discovery:    discovery,
		rotationTime: time.Now().Add(GuardRotationPeriod),
	}
}

// GetGuardRelay returns a random guard relay for use as entry node
// If no guards are set, selects new ones automatically
func (grm *GuardRelayManager) GetGuardRelay() (*RelayMetadata, error) {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	// Initialize guards if empty
	if len(grm.guards) == 0 {
		if err := grm.selectNewGuardsLocked(); err != nil {
			return nil, fmt.Errorf("failed to select guard relays: %w", err)
		}
	}

	// Check if rotation is needed (every 60 days)
	if time.Now().After(grm.rotationTime) {
		log.Printf("🔄 Guard relay rotation period reached, selecting new guards...")
		if err := grm.selectNewGuardsLocked(); err != nil {
			log.Printf("⚠️  Failed to rotate guards: %v", err)
		} else {
			grm.rotationTime = time.Now().Add(GuardRotationPeriod)
		}
	}

	// Filter healthy guards
	healthyGuards := make([]*GuardRelay, 0)
	for _, guard := range grm.guards {
		// Require at least 80% success rate
		totalAttempts := guard.SuccessCount + guard.FailCount
		if totalAttempts == 0 || float64(guard.SuccessCount)/float64(totalAttempts) >= 0.8 {
			healthyGuards = append(healthyGuards, guard)
		}
	}

	if len(healthyGuards) == 0 {
		// All guards failed, select new ones
		log.Printf("⚠️  All guard relays unhealthy, selecting new guards...")
		if err := grm.selectNewGuardsLocked(); err != nil {
			return nil, fmt.Errorf("failed to select replacement guards: %w", err)
		}
		healthyGuards = grm.guards
	}

	// Return random healthy guard
	guard := healthyGuards[rand.Intn(len(healthyGuards))]
	guard.LastUsed = time.Now()

	return guard.Metadata, nil
}

// RecordSuccess records a successful connection to a guard
func (grm *GuardRelayManager) RecordSuccess(guardAddr protocol.Address) {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	for _, guard := range grm.guards {
		if guard.Metadata.Address == guardAddr {
			guard.SuccessCount++
			guard.LastUsed = time.Now()
			return
		}
	}
}

// RecordFailure records a failed connection to a guard
func (grm *GuardRelayManager) RecordFailure(guardAddr protocol.Address) {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	for _, guard := range grm.guards {
		if guard.Metadata.Address == guardAddr {
			guard.FailCount++

			// If guard has too many failures, remove it
			if guard.FailCount > 10 {
				log.Printf("🚫 Removing unreliable guard %s (too many failures)", guardAddr)
				grm.removeGuardLocked(guardAddr)

				// Select replacement
				if err := grm.selectNewGuardsLocked(); err != nil {
					log.Printf("⚠️  Failed to select replacement guard: %v", err)
				}
			}
			return
		}
	}
}

// selectNewGuardsLocked selects new guard relays (must hold lock)
func (grm *GuardRelayManager) selectNewGuardsLocked() error {
	if grm.discovery == nil {
		return fmt.Errorf("relay discovery not initialized")
	}

	// Discover candidate relays (fetch more than needed)
	candidates, err := grm.discovery.DiscoverRelays(NumGuardRelays * 3)
	if err != nil {
		return fmt.Errorf("failed to discover relays: %w", err)
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no relays available")
	}

	// Filter candidates: require minimum uptime and good health
	goodCandidates := make([]*RelayMetadata, 0)
	for _, relay := range candidates {
		// Calculate uptime
		uptime := time.Since(time.Unix(relay.LastSeen-3600*24*7, 0)) // Approximate

		// Require at least 7 days uptime and good reliability
		if uptime >= GuardMinUptime && relay.Reliability >= 0.8 {
			goodCandidates = append(goodCandidates, relay)
		}
	}

	if len(goodCandidates) == 0 {
		// No candidates meet requirements, use any available
		log.Printf("⚠️  No relays meet guard requirements, using any available")
		goodCandidates = candidates
	}

	// Select random subset
	rand.Shuffle(len(goodCandidates), func(i, j int) {
		goodCandidates[i], goodCandidates[j] = goodCandidates[j], goodCandidates[i]
	})

	numToSelect := NumGuardRelays
	if len(goodCandidates) < numToSelect {
		numToSelect = len(goodCandidates)
	}

	// Clear old guards and set new ones
	grm.guards = make([]*GuardRelay, 0, numToSelect)
	for i := 0; i < numToSelect; i++ {
		guard := &GuardRelay{
			Metadata:  goodCandidates[i],
			FirstUsed: time.Now(),
			LastUsed:  time.Now(),
		}
		grm.guards = append(grm.guards, guard)
	}

	log.Printf("🛡️  Selected %d guard relays:", len(grm.guards))
	for _, guard := range grm.guards {
		log.Printf("   - %s (%s)", guard.Metadata.NetworkAddress, guard.Metadata.Region)
	}

	return nil
}

// removeGuardLocked removes a guard relay (must hold lock)
func (grm *GuardRelayManager) removeGuardLocked(addr protocol.Address) {
	for i, guard := range grm.guards {
		if guard.Metadata.Address == addr {
			grm.guards = append(grm.guards[:i], grm.guards[i+1:]...)
			return
		}
	}
}

// GetGuardRelays returns the current list of guard relays (for debugging/UI)
func (grm *GuardRelayManager) GetGuardRelays() []*GuardRelay {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	guards := make([]*GuardRelay, len(grm.guards))
	copy(guards, grm.guards)
	return guards
}

// ForceRotation forces immediate rotation of guard relays
func (grm *GuardRelayManager) ForceRotation() error {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	log.Printf("🔄 Forcing guard relay rotation...")
	if err := grm.selectNewGuardsLocked(); err != nil {
		return err
	}

	grm.rotationTime = time.Now().Add(GuardRotationPeriod)
	return nil
}
