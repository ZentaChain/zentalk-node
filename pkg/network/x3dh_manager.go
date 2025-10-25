package network

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/zentalk/protocol/pkg/protocol"
)

// InitializeX3DH initializes X3DH identity and prekeys for forward secrecy
// Returns error if key generation fails
func (c *Client) InitializeX3DH() error {
	// Generate X3DH identity keypair
	identity, err := protocol.GenerateIdentityKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate identity keypair: %w", err)
	}
	c.x3dhIdentity = identity

	// Generate signed prekey
	signedPreKey, err := protocol.GenerateSignedPreKey(1, identity)
	if err != nil {
		return fmt.Errorf("failed to generate signed prekey: %w", err)
	}
	c.signedPreKey = signedPreKey

	// Generate initial pool of one-time prekeys (start at ID 100, generate 50 keys)
	oneTimePreKeys, err := protocol.GenerateOneTimePreKeys(100, 50)
	if err != nil {
		return fmt.Errorf("failed to generate one-time prekeys: %w", err)
	}

	// Store one-time prekeys in map
	for _, opk := range oneTimePreKeys {
		c.oneTimePreKeys[opk.KeyID] = opk
	}

	// Generate random registration ID (use timestamp + random component)
	c.registrationID = uint32(time.Now().Unix())

	log.Printf("✅ X3DH initialized: Identity=%x..., SignedPreKey=#%d, OneTimePreKeys=%d, RegID=%d",
		identity.DHPublic[:8], signedPreKey.KeyID, len(oneTimePreKeys), c.registrationID)

	// Persist X3DH state if storage is attached
	if err := c.saveX3DHState(); err != nil {
		log.Printf("⚠️  Failed to persist X3DH state: %v", err)
	}

	return nil
}

// GetKeyBundle returns the client's key bundle for X3DH key agreement
// This should be published to a key server or shared directly with contacts
func (c *Client) GetKeyBundle() (*protocol.KeyBundle, error) {
	if c.x3dhIdentity == nil || c.signedPreKey == nil {
		return nil, errors.New("X3DH not initialized - call InitializeX3DH() first")
	}

	// Convert one-time prekeys to slice
	opks := make([]*protocol.OneTimePreKeyPrivate, 0, len(c.oneTimePreKeys))
	for _, opk := range c.oneTimePreKeys {
		opks = append(opks, opk)
	}

	// Create key bundle
	bundle := protocol.CreateKeyBundle(
		c.Address,
		c.x3dhIdentity,
		c.signedPreKey,
		opks,
		c.registrationID,
	)

	return bundle, nil
}

// RefillOneTimePreKeys generates additional one-time prekeys if the pool is low
func (c *Client) RefillOneTimePreKeys(threshold int) error {
	if len(c.oneTimePreKeys) >= threshold {
		return nil // Pool is sufficient
	}

	// Find highest existing key ID
	maxID := uint32(0)
	for id := range c.oneTimePreKeys {
		if id > maxID {
			maxID = id
		}
	}

	// Generate 50 new keys starting after the highest ID
	newKeys, err := protocol.GenerateOneTimePreKeys(maxID+1, 50)
	if err != nil {
		return fmt.Errorf("failed to generate one-time prekeys: %w", err)
	}

	// Add to map
	for _, opk := range newKeys {
		c.oneTimePreKeys[opk.KeyID] = opk
	}

	log.Printf("✅ Refilled one-time prekeys: now have %d keys", len(c.oneTimePreKeys))

	// Persist X3DH state if storage is attached
	if err := c.saveX3DHState(); err != nil {
		log.Printf("⚠️  Failed to persist X3DH state after refill: %v", err)
	}

	return nil
}

// GetX3DHIdentity returns the client's X3DH identity
func (c *Client) GetX3DHIdentity() *protocol.IdentityKeyPair {
	return c.x3dhIdentity
}

// GetSignedPreKey returns the client's signed prekey
func (c *Client) GetSignedPreKey() *protocol.SignedPreKeyPrivate {
	return c.signedPreKey
}

// GetOneTimePreKeys returns the client's one-time prekeys map
func (c *Client) GetOneTimePreKeys() map[uint32]*protocol.OneTimePreKeyPrivate {
	return c.oneTimePreKeys
}

// CacheKeyBundle stores a key bundle for a user
func (c *Client) CacheKeyBundle(addr protocol.Address, bundle *protocol.KeyBundle) {
	c.keyBundleCache[addr] = bundle
	log.Printf("✅ Key bundle cached for %x (OPKs: %d)", addr[:8], len(bundle.OneTimePreKeys))

	// Persist key bundle cache if storage is attached
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveKeyBundleCache(c.keyBundleCache); err != nil {
			log.Printf("⚠️  Failed to persist key bundle cache: %v", err)
		}
	}
}

// GetCachedKeyBundle retrieves a cached key bundle
func (c *Client) GetCachedKeyBundle(addr protocol.Address) (*protocol.KeyBundle, bool) {
	bundle, exists := c.keyBundleCache[addr]
	return bundle, exists
}

// ClearKeyBundleCache clears all cached key bundles
func (c *Client) ClearKeyBundleCache() {
	c.keyBundleCache = make(map[protocol.Address]*protocol.KeyBundle)
	log.Printf("Key bundle cache cleared")
}

// RemoveCachedKeyBundle removes a specific key bundle from cache
func (c *Client) RemoveCachedKeyBundle(addr protocol.Address) {
	delete(c.keyBundleCache, addr)
	log.Printf("Key bundle removed from cache: %x", addr[:8])

	// Persist updated cache
	if c.sessionStorage != nil {
		if err := c.sessionStorage.SaveKeyBundleCache(c.keyBundleCache); err != nil {
			log.Printf("⚠️  Failed to persist key bundle cache: %v", err)
		}
	}
}

// saveX3DHState saves the current X3DH state to disk
func (c *Client) saveX3DHState() error {
	if c.sessionStorage == nil {
		return nil // No storage attached
	}

	// Convert oneTimePreKeys map to string-keyed map for JSON
	opkMap := make(map[string]*protocol.OneTimePreKeyPrivate)
	for keyID, opk := range c.oneTimePreKeys {
		opkMap[fmt.Sprintf("%d", keyID)] = opk
	}

	state := &X3DHState{
		IdentityKeyPair: c.x3dhIdentity,
		SignedPreKey:    c.signedPreKey,
		OneTimePreKeys:  opkMap,
		RegistrationID:  c.registrationID,
	}

	return c.sessionStorage.SaveX3DHState(state)
}

// loadX3DHState loads X3DH state from disk
func (c *Client) loadX3DHState() error {
	if c.sessionStorage == nil {
		return nil // No storage attached
	}

	state, err := c.sessionStorage.LoadX3DHState()
	if err != nil {
		return err
	}

	if state == nil {
		return nil // No persisted state exists
	}

	// Restore X3DH state
	c.x3dhIdentity = state.IdentityKeyPair
	c.signedPreKey = state.SignedPreKey
	c.registrationID = state.RegistrationID

	// Convert string-keyed map back to uint32-keyed map
	c.oneTimePreKeys = make(map[uint32]*protocol.OneTimePreKeyPrivate)
	for keyIDStr, opk := range state.OneTimePreKeys {
		var keyID uint32
		fmt.Sscanf(keyIDStr, "%d", &keyID)
		c.oneTimePreKeys[keyID] = opk
	}

	log.Printf("✅ Loaded X3DH state: Identity=%x..., SignedPreKey=#%d, OneTimePreKeys=%d, RegID=%d",
		c.x3dhIdentity.DHPublic[:8], c.signedPreKey.KeyID, len(c.oneTimePreKeys), c.registrationID)

	return nil
}
