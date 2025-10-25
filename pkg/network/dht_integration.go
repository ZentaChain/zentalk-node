package network

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ZentaChain/zentalk-node/pkg/dht"
	"github.com/ZentaChain/zentalk-node/pkg/protocol"
)

// AttachDHT attaches a DHT node for decentralized key bundle discovery
func (c *Client) AttachDHT(node *dht.Node) {
	c.dhtNode = node
	log.Printf("✅ DHT attached to client %x", c.Address[:8])
}

// PublishKeyBundle publishes the client's key bundle to the DHT
// This makes the client discoverable by others in the network
func (c *Client) PublishKeyBundle() error {
	if c.dhtNode == nil {
		return fmt.Errorf("DHT not attached - call AttachDHT() first")
	}

	if c.x3dhIdentity == nil {
		return fmt.Errorf("X3DH not initialized - call InitializeX3DH() first")
	}

	// Get our key bundle
	bundle, err := c.GetKeyBundle()
	if err != nil {
		return fmt.Errorf("failed to get key bundle: %w", err)
	}

	// Serialize key bundle to JSON
	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("failed to serialize key bundle: %w", err)
	}

	// Use our address as the DHT key
	dhtKey := dht.NewNodeID(c.Address[:])

	// Publish to DHT with 24-hour TTL
	if err := c.dhtNode.Store(dhtKey, bundleJSON, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to publish to DHT: %w", err)
	}

	log.Printf("✅ Published key bundle to DHT (key: %s, size: %d bytes, OPKs: %d)",
		dhtKey.String()[:16], len(bundleJSON), len(bundle.OneTimePreKeys))

	return nil
}

// DiscoverKeyBundle looks up another user's key bundle from the DHT
// Returns the key bundle if found, or error if not found
func (c *Client) DiscoverKeyBundle(peerAddress protocol.Address) (*protocol.KeyBundle, error) {
	if c.dhtNode == nil {
		return nil, fmt.Errorf("DHT not attached - call AttachDHT() first")
	}

	// Use peer's address as the DHT key
	dhtKey := dht.NewNodeID(peerAddress[:])

	log.Printf("🔍 Looking up key bundle for %x in DHT...", peerAddress[:8])

	// Lookup in DHT
	bundleJSON, found := c.dhtNode.Lookup(dhtKey)
	if !found {
		return nil, fmt.Errorf("key bundle not found in DHT for %x", peerAddress[:8])
	}

	// Deserialize key bundle
	var bundle protocol.KeyBundle
	if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
		return nil, fmt.Errorf("failed to deserialize key bundle: %w", err)
	}

	log.Printf("✅ Found key bundle for %x (OPKs: %d)", peerAddress[:8], len(bundle.OneTimePreKeys))

	// Cache the bundle for future use
	c.CacheKeyBundle(peerAddress, &bundle)

	return &bundle, nil
}

// AutoPublishKeyBundle automatically republishes the key bundle periodically
// This ensures the key bundle remains available in the DHT
// Should be run in a goroutine
func (c *Client) AutoPublishKeyBundle(interval time.Duration) {
	if c.dhtNode == nil {
		log.Printf("⚠️  DHT not attached, skipping auto-publish")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C

		if err := c.PublishKeyBundle(); err != nil {
			log.Printf("⚠️  Failed to auto-publish key bundle: %v", err)
		} else {
			log.Printf("🔄 Key bundle republished to DHT")
		}
	}
}

// BootstrapDHT bootstraps the DHT from a known peer
func (c *Client) BootstrapDHT(bootstrapAddress string, bootstrapNodeID dht.NodeID) error {
	if c.dhtNode == nil {
		return fmt.Errorf("DHT not attached - call AttachDHT() first")
	}

	bootstrapContact := dht.NewContact(bootstrapNodeID, bootstrapAddress)

	if err := c.dhtNode.Bootstrap(bootstrapContact); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	log.Printf("✅ DHT bootstrapped from %s", bootstrapAddress)
	return nil
}

// GetDHTNode returns the DHT node (for advanced operations)
func (c *Client) GetDHTNode() *dht.Node {
	return c.dhtNode
}
