package dht

import (
	"container/list"
	"sync"
	"time"
)

const (
	// K is the replication parameter (bucket size)
	K = 20

	// Alpha is the concurrency parameter for lookups
	Alpha = 3

	// BucketCount is the number of buckets (160 for 160-bit IDs)
	BucketCount = 160
)

// Contact represents a node in the network
type Contact struct {
	ID          NodeID
	Address     string    // IP:Port
	LastSeen    time.Time
	FailedPings int
}

// NewContact creates a new contact
func NewContact(id NodeID, address string) *Contact {
	return &Contact{
		ID:       id,
		Address:  address,
		LastSeen: time.Now(),
	}
}

// Bucket represents a k-bucket in the routing table
type Bucket struct {
	contacts *list.List
	mu       sync.RWMutex
}

// NewBucket creates a new k-bucket
func NewBucket() *Bucket {
	return &Bucket{
		contacts: list.New(),
	}
}

// AddContact adds a contact to the bucket (LRU policy)
func (b *Bucket) AddContact(contact *Contact) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if contact already exists
	for e := b.contacts.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Contact)
		if c.ID.Equals(contact.ID) {
			// Move to back (most recently seen)
			b.contacts.MoveToBack(e)
			c.LastSeen = time.Now()
			c.FailedPings = 0
			return true
		}
	}

	// If bucket is not full, add to back
	if b.contacts.Len() < K {
		b.contacts.PushBack(contact)
		return true
	}

	// Bucket is full - don't add (could implement ping/replace logic here)
	return false
}

// RemoveContact removes a contact from the bucket
func (b *Bucket) RemoveContact(id NodeID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for e := b.contacts.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Contact)
		if c.ID.Equals(id) {
			b.contacts.Remove(e)
			return
		}
	}
}

// GetContacts returns all contacts in the bucket
func (b *Bucket) GetContacts() []*Contact {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*Contact, 0, b.contacts.Len())
	for e := b.contacts.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(*Contact))
	}
	return result
}

// Len returns the number of contacts in the bucket
func (b *Bucket) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.contacts.Len()
}

// RoutingTable represents the Kademlia routing table
type RoutingTable struct {
	self    NodeID
	buckets [BucketCount]*Bucket
	mu      sync.RWMutex
}

// NewRoutingTable creates a new routing table
func NewRoutingTable(self NodeID) *RoutingTable {
	rt := &RoutingTable{
		self: self,
	}

	// Initialize all buckets
	for i := 0; i < BucketCount; i++ {
		rt.buckets[i] = NewBucket()
	}

	return rt
}

// getBucketIndex returns the bucket index for a given node ID
// The index is the length of the common prefix (number of leading matching bits)
func (rt *RoutingTable) getBucketIndex(id NodeID) int {
	// XOR distance
	xor := rt.self.Xor(id)

	// Find the first non-zero bit (from left)
	for i := 0; i < 20; i++ {
		if xor[i] != 0 {
			// Count leading zeros in this byte
			b := xor[i]
			for j := 7; j >= 0; j-- {
				if (b & (1 << uint(j))) != 0 {
					return i*8 + (7 - j)
				}
			}
		}
	}

	// IDs are identical - shouldn't happen, but return last bucket
	return BucketCount - 1
}

// AddContact adds a contact to the routing table
func (rt *RoutingTable) AddContact(contact *Contact) bool {
	// Don't add ourselves
	if contact.ID.Equals(rt.self) {
		return false
	}

	bucketIndex := rt.getBucketIndex(contact.ID)
	return rt.buckets[bucketIndex].AddContact(contact)
}

// RemoveContact removes a contact from the routing table
func (rt *RoutingTable) RemoveContact(id NodeID) {
	bucketIndex := rt.getBucketIndex(id)
	rt.buckets[bucketIndex].RemoveContact(id)
}

// FindClosest returns the k closest contacts to a target ID
func (rt *RoutingTable) FindClosest(target NodeID, count int) []*Contact {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Collect all contacts
	var allContacts []*Contact
	for _, bucket := range rt.buckets {
		allContacts = append(allContacts, bucket.GetContacts()...)
	}

	// Sort by distance to target using XOR metric
	// Simple bubble sort is fine for small lists
	for i := 0; i < len(allContacts); i++ {
		for j := i + 1; j < len(allContacts); j++ {
			if allContacts[j].ID.CloserTo(target, allContacts[i].ID) {
				allContacts[i], allContacts[j] = allContacts[j], allContacts[i]
			}
		}
	}

	// Return up to 'count' closest
	if len(allContacts) > count {
		return allContacts[:count]
	}
	return allContacts
}

// GetContact retrieves a specific contact by ID
func (rt *RoutingTable) GetContact(id NodeID) *Contact {
	bucketIndex := rt.getBucketIndex(id)
	contacts := rt.buckets[bucketIndex].GetContacts()

	for _, c := range contacts {
		if c.ID.Equals(id) {
			return c
		}
	}
	return nil
}

// Size returns the total number of contacts in the routing table
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	size := 0
	for _, bucket := range rt.buckets {
		size += bucket.Len()
	}
	return size
}

// GetAllContacts returns all contacts from all buckets
func (rt *RoutingTable) GetAllContacts() []*Contact {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var result []*Contact
	for _, bucket := range rt.buckets {
		result = append(result, bucket.GetContacts()...)
	}
	return result
}
