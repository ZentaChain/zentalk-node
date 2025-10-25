package dht

import (
	"sync"
	"time"
)

// StoredValue represents a value stored in the DHT
type StoredValue struct {
	Key       NodeID
	Value     []byte
	ExpiresAt time.Time
	Publisher NodeID // Original publisher
}

// Storage represents the local key-value storage for a DHT node
type Storage struct {
	data map[NodeID]*StoredValue
	mu   sync.RWMutex
}

// NewStorage creates a new storage instance
func NewStorage() *Storage {
	return &Storage{
		data: make(map[NodeID]*StoredValue),
	}
}

// Store stores a key-value pair with TTL
func (s *Storage) Store(key NodeID, value []byte, ttl time.Duration, publisher NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = &StoredValue{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		Publisher: publisher,
	}
}

// Get retrieves a value by key
func (s *Storage) Get(key NodeID) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, exists := s.data[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(stored.ExpiresAt) {
		return nil, false
	}

	return stored.Value, true
}

// Delete removes a key-value pair
func (s *Storage) Delete(key NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
}

// GetAll returns all stored values (for debugging/inspection)
func (s *Storage) GetAll() []*StoredValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*StoredValue, 0, len(s.data))
	for _, v := range s.data {
		// Only include non-expired values
		if time.Now().Before(v.ExpiresAt) {
			result = append(result, v)
		}
	}
	return result
}

// ExpireOldValues removes expired values
func (s *Storage) ExpireOldValues() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, value := range s.data {
		if now.After(value.ExpiresAt) {
			delete(s.data, key)
		}
	}
}

// Size returns the number of stored values
func (s *Storage) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Has checks if a key exists and is not expired
func (s *Storage) Has(key NodeID) bool {
	_, exists := s.Get(key)
	return exists
}
