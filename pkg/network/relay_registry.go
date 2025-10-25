package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
)

// RelayRegistry manages available relays
type RelayRegistry struct {
	relays map[string]*RelayInfo // key: endpoint
	mu     sync.RWMutex
}

// NewRelayRegistry creates a new relay registry
func NewRelayRegistry() *RelayRegistry {
	return &RelayRegistry{
		relays: make(map[string]*RelayInfo),
	}
}

// AddRelay adds a relay to the registry
func (r *RelayRegistry) AddRelay(info *RelayInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.relays[info.Endpoint] = info
}

// RemoveRelay removes a relay from the registry
func (r *RelayRegistry) RemoveRelay(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.relays, endpoint)
}

// GetRelay gets relay info by endpoint
func (r *RelayRegistry) GetRelay(endpoint string) (*RelayInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.relays[endpoint]
	return info, ok
}

// GetAllRelays returns all registered relays
func (r *RelayRegistry) GetAllRelays() []*RelayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	relays := make([]*RelayInfo, 0, len(r.relays))
	for _, info := range r.relays {
		relays = append(relays, info)
	}
	return relays
}

// GetOnlineRelays returns only online relays
func (r *RelayRegistry) GetOnlineRelays() []*RelayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	relays := make([]*RelayInfo, 0, len(r.relays))
	for _, info := range r.relays {
		if info.IsActive {
			relays = append(relays, info)
		}
	}
	return relays
}

// GetRelaysByRegion returns relays in a specific region
func (r *RelayRegistry) GetRelaysByRegion(region string) []*RelayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	relays := make([]*RelayInfo, 0)
	for _, info := range r.relays {
		if info.Region == region && info.IsActive {
			relays = append(relays, info)
		}
	}
	return relays
}

// GetBestRelays returns the top N relays by reputation
func (r *RelayRegistry) GetBestRelays(n int) []*RelayInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get all online relays
	relays := make([]*RelayInfo, 0, len(r.relays))
	for _, info := range r.relays {
		if info.IsActive {
			relays = append(relays, info)
		}
	}

	// Sort by reputation (simple bubble sort for small n)
	for i := 0; i < len(relays)-1; i++ {
		for j := i + 1; j < len(relays); j++ {
			if relays[j].Reputation > relays[i].Reputation {
				relays[i], relays[j] = relays[j], relays[i]
			}
		}
	}

	// Return top N
	if len(relays) > n {
		return relays[:n]
	}
	return relays
}

// BuildRelayPath builds an optimal relay path for onion routing
func (r *RelayRegistry) BuildRelayPath(numRelays int) ([]*crypto.RelayInfo, error) {
	bestRelays := r.GetBestRelays(numRelays)

	if len(bestRelays) < numRelays {
		return nil, fmt.Errorf("not enough online relays (need %d, found %d)", numRelays, len(bestRelays))
	}

	// Convert to crypto.RelayInfo format
	path := make([]*crypto.RelayInfo, numRelays)
	for i, relay := range bestRelays {
		// Parse public key from PEM if needed
		if relay.PublicKey == nil && relay.PublicKeyPEM != "" {
			pubKey, err := crypto.ImportPublicKeyPEM([]byte(relay.PublicKeyPEM))
			if err != nil {
				return nil, fmt.Errorf("failed to parse relay public key: %v", err)
			}
			relay.PublicKey = pubKey
		}

		path[i] = &crypto.RelayInfo{
			Address:   relay.Address,
			PublicKey: relay.PublicKey,
		}
	}

	return path, nil
}

// LoadFromFile loads relay registry from JSON file
func (r *RelayRegistry) LoadFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %v", err)
	}

	var relays []*RelayInfo
	if err := json.Unmarshal(data, &relays); err != nil {
		return fmt.Errorf("failed to parse registry JSON: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.relays = make(map[string]*RelayInfo)
	for _, relay := range relays {
		r.relays[relay.Endpoint] = relay
	}

	return nil
}

// SaveToFile saves relay registry to JSON file
func (r *RelayRegistry) SaveToFile(filePath string) error {
	r.mu.RLock()
	relays := make([]*RelayInfo, 0, len(r.relays))
	for _, info := range r.relays {
		relays = append(relays, info)
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(relays, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %v", err)
	}

	return nil
}

// FetchFromURL fetches relay registry from remote URL
func (r *RelayRegistry) FetchFromURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch registry: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry fetch failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read registry response: %v", err)
	}

	var relays []*RelayInfo
	if err := json.Unmarshal(data, &relays); err != nil {
		return fmt.Errorf("failed to parse registry JSON: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.relays = make(map[string]*RelayInfo)
	for _, relay := range relays {
		r.relays[relay.Endpoint] = relay
	}

	return nil
}

// HealthCheck checks if a relay is online
func (r *RelayRegistry) HealthCheck(endpoint string) bool {
	// Simple TCP connection test
	// In production, this would ping the relay and check response
	// For now, we'll just return the stored status
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.relays[endpoint]; ok {
		return info.IsActive
	}
	return false
}

// UpdateRelayStatus updates the online status of a relay
func (r *RelayRegistry) UpdateRelayStatus(endpoint string, isActive bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info, ok := r.relays[endpoint]; ok {
		info.IsActive = isActive
	}
}

// ===== HELPER FUNCTIONS =====

// CreateSampleRegistry creates a sample registry for testing
func CreateSampleRegistry() *RelayRegistry {
	registry := NewRelayRegistry()

	// Generate sample relay keys
	relay1Key, _ := crypto.GenerateRSAKeyPair()
	relay2Key, _ := crypto.GenerateRSAKeyPair()
	relay3Key, _ := crypto.GenerateRSAKeyPair()

	relay1PEM, _ := crypto.ExportPublicKeyPEM(&relay1Key.PublicKey)
	relay2PEM, _ := crypto.ExportPublicKeyPEM(&relay2Key.PublicKey)
	relay3PEM, _ := crypto.ExportPublicKeyPEM(&relay3Key.PublicKey)

	var relay1Addr, relay2Addr, relay3Addr [20]byte
	copy(relay1Addr[:], []byte("relay1______________"))
	copy(relay2Addr[:], []byte("relay2______________"))
	copy(relay3Addr[:], []byte("relay3______________"))

	registry.AddRelay(&RelayInfo{
		Address:      relay1Addr,
		Endpoint:     "localhost:8081",
		PublicKey:    &relay1Key.PublicKey,
		PublicKeyPEM: string(relay1PEM),
		Region:       "us-east",
		Reputation:   95,
		IsActive:     true,
	})

	registry.AddRelay(&RelayInfo{
		Address:      relay2Addr,
		Endpoint:     "localhost:8082",
		PublicKey:    &relay2Key.PublicKey,
		PublicKeyPEM: string(relay2PEM),
		Region:       "eu-west",
		Reputation:   88,
		IsActive:     true,
	})

	registry.AddRelay(&RelayInfo{
		Address:      relay3Addr,
		Endpoint:     "localhost:8083",
		PublicKey:    &relay3Key.PublicKey,
		PublicKeyPEM: string(relay3PEM),
		Region:       "ap-south",
		Reputation:   92,
		IsActive:     true,
	})

	return registry
}
