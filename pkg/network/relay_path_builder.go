package network

import (
	"fmt"
	"log"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
)

// BuildSecureRelayPath builds a relay path using a guard relay as the entry node
// This provides stronger privacy guarantees against entry node correlation attacks
func (c *Client) BuildSecureRelayPath(numRelays int) ([]*crypto.RelayInfo, error) {
	if numRelays < 3 {
		return nil, fmt.Errorf("relay path must have at least 3 hops")
	}

	// Initialize guard relay manager if not already done
	if c.guardRelayManager == nil && c.relayDiscovery != nil {
		c.guardRelayManager = NewGuardRelayManager(c.relayDiscovery)
		log.Printf("🛡️  Guard relay manager initialized")
	}

	var path []*crypto.RelayInfo

	// Use guard relay as entry node if available
	if c.guardRelayManager != nil {
		guardRelay, err := c.guardRelayManager.GetGuardRelay()
		if err != nil {
			log.Printf("⚠️  Failed to get guard relay: %v, using random entry", err)
		} else {
			// Parse guard's public key
			guardPubKey, err := crypto.ImportPublicKeyPEM([]byte(guardRelay.PublicKeyPEM))
			if err != nil {
				log.Printf("⚠️  Failed to parse guard public key: %v", err)
			} else {
				// Convert guard to RelayInfo
				guardInfo := &crypto.RelayInfo{
					Address:   guardRelay.Address,
					PublicKey: guardPubKey,
				}
				path = append(path, guardInfo)
				log.Printf("🛡️  Using guard relay as entry: %s", guardRelay.NetworkAddress)
			}

			// Record that we're using this guard
			defer func() {
				if err == nil {
					c.guardRelayManager.RecordSuccess(guardRelay.Address)
				} else {
					c.guardRelayManager.RecordFailure(guardRelay.Address)
				}
			}()
		}
	}

	// Discover remaining relays (non-guards)
	remainingHops := numRelays - len(path)
	if remainingHops > 0 && c.relayDiscovery != nil {
		additionalRelays, err := c.relayDiscovery.DiscoverRelays(remainingHops)
		if err != nil {
			return nil, fmt.Errorf("failed to discover relays: %w", err)
		}

		// Convert to RelayInfo
		for i := 0; i < remainingHops && i < len(additionalRelays); i++ {
			pubKey, err := crypto.ImportPublicKeyPEM([]byte(additionalRelays[i].PublicKeyPEM))
			if err != nil {
				log.Printf("⚠️  Failed to parse relay public key: %v", err)
				continue
			}

			relayInfo := &crypto.RelayInfo{
				Address:   additionalRelays[i].Address,
				PublicKey: pubKey,
			}
			path = append(path, relayInfo)
		}
	}

	if len(path) < numRelays {
		return nil, fmt.Errorf("not enough relays available (need %d, found %d)", numRelays, len(path))
	}

	log.Printf("🔐 Built secure relay path: %d hops (guard + %d middle relays)", len(path), len(path)-1)
	return path, nil
}

// BuildRelayPath builds a relay path (legacy - doesn't use guard relays)
// Use BuildSecureRelayPath instead for better privacy
func (c *Client) BuildRelayPath(numRelays int) ([]*crypto.RelayInfo, error) {
	if c.relayDiscovery == nil {
		return nil, fmt.Errorf("relay discovery not initialized")
	}

	relays, err := c.relayDiscovery.DiscoverRelays(numRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to discover relays: %w", err)
	}

	if len(relays) < numRelays {
		return nil, fmt.Errorf("not enough relays available (need %d, found %d)", numRelays, len(relays))
	}

	// Convert to RelayInfo
	path := make([]*crypto.RelayInfo, 0, numRelays)
	for i := 0; i < numRelays; i++ {
		pubKey, err := crypto.ImportPublicKeyPEM([]byte(relays[i].PublicKeyPEM))
		if err != nil {
			log.Printf("⚠️  Failed to parse relay public key: %v", err)
			continue
		}

		relayInfo := &crypto.RelayInfo{
			Address:   relays[i].Address,
			PublicKey: pubKey,
		}
		path = append(path, relayInfo)
	}

	if len(path) < numRelays {
		return nil, fmt.Errorf("not enough valid relays (need %d, found %d)", numRelays, len(path))
	}

	return path, nil
}
