package dht

import (
	"testing"
	"time"
)

// TestNodeCreation tests creating a DHT node
func TestNodeCreation(t *testing.T) {
	id := RandomNodeID()
	node := NewNode(id, "localhost:0")

	if !node.ID.Equals(id) {
		t.Errorf("Node ID mismatch: expected %s, got %s", id.String(), node.ID.String())
	}

	if node.routingTable == nil {
		t.Error("Routing table should be initialized")
	}

	if node.storage == nil {
		t.Error("Storage should be initialized")
	}
}

// TestNodeStartStop tests starting and stopping a DHT node
func TestNodeStartStop(t *testing.T) {
	id := RandomNodeID()
	node := NewNode(id, "localhost:0")

	// Start the node
	err := node.Start()
	if err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	// Check node is running
	if !node.running {
		t.Error("Node should be running after Start()")
	}

	// Address should be updated
	if node.Address == "" {
		t.Error("Node address should be set after Start()")
	}

	// Stop the node
	err = node.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}

	// Check node is stopped
	if node.running {
		t.Error("Node should not be running after Stop()")
	}
}

// TestStorageBasicOperations tests basic storage operations
func TestStorageBasicOperations(t *testing.T) {
	storage := NewStorage()

	// Test Store and Get
	key := RandomNodeID()
	value := []byte("test value")
	publisher := RandomNodeID()
	ttl := 10 * time.Second

	storage.Store(key, value, ttl, publisher)

	retrievedValue, found := storage.Get(key)
	if !found {
		t.Error("Value should be found after storing")
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Retrieved value mismatch: expected %s, got %s", value, retrievedValue)
	}

	// Test Has
	if !storage.Has(key) {
		t.Error("Has should return true for stored key")
	}

	// Test Size
	if storage.Size() != 1 {
		t.Errorf("Storage size should be 1, got %d", storage.Size())
	}

	// Test Delete
	storage.Delete(key)
	if storage.Has(key) {
		t.Error("Key should not exist after deletion")
	}
}

// TestStorageExpiration tests value expiration
func TestStorageExpiration(t *testing.T) {
	storage := NewStorage()

	key := RandomNodeID()
	value := []byte("expiring value")
	publisher := RandomNodeID()
	ttl := 100 * time.Millisecond // Very short TTL

	storage.Store(key, value, ttl, publisher)

	// Should be available immediately
	_, found := storage.Get(key)
	if !found {
		t.Error("Value should be found immediately after storing")
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Should be expired now
	_, found = storage.Get(key)
	if found {
		t.Error("Value should be expired after TTL")
	}
}

// TestRoutingTableAddContact tests adding contacts to routing table
func TestRoutingTableAddContact(t *testing.T) {
	selfID := RandomNodeID()
	rt := NewRoutingTable(selfID)

	// Add a contact
	contact := NewContact(RandomNodeID(), "127.0.0.1:1234")
	added := rt.AddContact(contact)

	if !added {
		t.Error("Contact should be added successfully")
	}

	if rt.Size() != 1 {
		t.Errorf("Routing table size should be 1, got %d", rt.Size())
	}

	// Try to add self (should fail)
	selfContact := NewContact(selfID, "127.0.0.1:5678")
	added = rt.AddContact(selfContact)

	if added {
		t.Error("Should not be able to add self to routing table")
	}

	if rt.Size() != 1 {
		t.Errorf("Routing table size should still be 1, got %d", rt.Size())
	}
}

// TestRoutingTableFindClosest tests finding closest contacts
func TestRoutingTableFindClosest(t *testing.T) {
	selfID := RandomNodeID()
	rt := NewRoutingTable(selfID)

	// Add multiple contacts
	contacts := make([]*Contact, 10)
	for i := 0; i < 10; i++ {
		contacts[i] = NewContact(RandomNodeID(), "127.0.0.1:"+string(rune(1000+i)))
		rt.AddContact(contacts[i])
	}

	// Find 3 closest to a random target
	target := RandomNodeID()
	closest := rt.FindClosest(target, 3)

	if len(closest) != 3 {
		t.Errorf("Should find 3 closest contacts, got %d", len(closest))
	}

	// Verify they're sorted by distance
	if len(closest) >= 2 {
		if !closest[0].ID.CloserTo(target, closest[1].ID) {
			t.Error("Closest contacts should be sorted by distance")
		}
	}
}

// TestPingBetweenNodes tests pinging between two DHT nodes
func TestPingBetweenNodes(t *testing.T) {
	// Create two nodes
	node1 := NewNode(RandomNodeID(), "localhost:0")
	node2 := NewNode(RandomNodeID(), "localhost:0")

	// Start both nodes
	if err := node1.Start(); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	if err := node2.Start(); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	// Node1 pings Node2
	contact2 := NewContact(node2.ID, node2.Address)
	err := node1.Ping(contact2)

	if err != nil {
		t.Errorf("Ping should succeed: %v", err)
	}

	// Check that node2 was added to node1's routing table
	if node1.routingTable.GetContact(node2.ID) == nil {
		t.Error("Node2 should be in Node1's routing table after successful ping")
	}
}

// TestStoreAndRetrieve tests storing and retrieving values through DHT
func TestStoreAndRetrieve(t *testing.T) {
	// Create three nodes for a minimal network
	node1 := NewNode(RandomNodeID(), "localhost:0")
	node2 := NewNode(RandomNodeID(), "localhost:0")
	node3 := NewNode(RandomNodeID(), "localhost:0")

	// Start all nodes
	if err := node1.Start(); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	if err := node2.Start(); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	if err := node3.Start(); err != nil {
		t.Fatalf("Failed to start node3: %v", err)
	}
	defer node3.Stop()

	// Connect nodes: node1 <-> node2 <-> node3
	contact1 := NewContact(node1.ID, node1.Address)
	contact2 := NewContact(node2.ID, node2.Address)
	contact3 := NewContact(node3.ID, node3.Address)

	node1.AddPeer(contact2)
	node2.AddPeer(contact1)
	node2.AddPeer(contact3)
	node3.AddPeer(contact2)

	// Store a value from node1
	key := RandomNodeID()
	value := []byte("Hello DHT!")
	ttl := 1 * time.Hour

	err := node1.Store(key, value, ttl)
	if err != nil {
		t.Fatalf("Failed to store value: %v", err)
	}

	// Give some time for propagation
	time.Sleep(500 * time.Millisecond)

	// Try to retrieve from node3
	retrievedValue, found := node3.Lookup(key)
	if !found {
		t.Error("Value should be retrievable from node3")
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Retrieved value mismatch: expected %s, got %s", value, retrievedValue)
	}
}

// TestBootstrap tests bootstrapping into a DHT network
func TestBootstrap(t *testing.T) {
	// Create bootstrap node
	bootstrapNode := NewNode(RandomNodeID(), "localhost:0")
	if err := bootstrapNode.Start(); err != nil {
		t.Fatalf("Failed to start bootstrap node: %v", err)
	}
	defer bootstrapNode.Stop()

	// Create new node
	newNode := NewNode(RandomNodeID(), "localhost:0")
	if err := newNode.Start(); err != nil {
		t.Fatalf("Failed to start new node: %v", err)
	}
	defer newNode.Stop()

	// Bootstrap from bootstrapNode
	contact := NewContact(bootstrapNode.ID, bootstrapNode.Address)
	err := newNode.Bootstrap(contact)

	if err != nil {
		t.Errorf("Bootstrap should succeed: %v", err)
	}

	// Check that bootstrap node is in routing table
	if newNode.routingTable.GetContact(bootstrapNode.ID) == nil {
		t.Error("Bootstrap node should be in new node's routing table")
	}
}

// TestBucketLRUPolicy tests the Least Recently Used policy in k-buckets
func TestBucketLRUPolicy(t *testing.T) {
	bucket := NewBucket()

	// Add contacts
	contacts := make([]*Contact, 5)
	for i := 0; i < 5; i++ {
		contacts[i] = NewContact(RandomNodeID(), "127.0.0.1:"+string(rune(1000+i)))
		bucket.AddContact(contacts[i])
	}

	// Re-add the first contact (should move to back)
	bucket.AddContact(contacts[0])

	// Get all contacts
	allContacts := bucket.GetContacts()

	// First contact should now be at the end (most recently seen)
	if !allContacts[len(allContacts)-1].ID.Equals(contacts[0].ID) {
		t.Error("Re-added contact should be moved to back (most recently seen)")
	}
}

// TestConcurrentAccess tests concurrent access to DHT
func TestConcurrentAccess(t *testing.T) {
	node := NewNode(RandomNodeID(), "localhost:0")
	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}
	defer node.Stop()

	// Concurrently add contacts
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			contact := NewContact(RandomNodeID(), "127.0.0.1:1234")
			node.routingTable.AddContact(contact)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check routing table size
	if node.routingTable.Size() < 1 {
		t.Error("Routing table should have contacts after concurrent adds")
	}
}

// Benchmark tests

func BenchmarkNodeIDXOR(b *testing.B) {
	id1 := RandomNodeID()
	id2 := RandomNodeID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id1.Xor(id2)
	}
}

func BenchmarkRoutingTableAddContact(b *testing.B) {
	rt := NewRoutingTable(RandomNodeID())
	contacts := make([]*Contact, b.N)
	for i := 0; i < b.N; i++ {
		contacts[i] = NewContact(RandomNodeID(), "127.0.0.1:1234")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.AddContact(contacts[i])
	}
}

func BenchmarkRoutingTableFindClosest(b *testing.B) {
	rt := NewRoutingTable(RandomNodeID())

	// Pre-populate with contacts
	for i := 0; i < 100; i++ {
		contact := NewContact(RandomNodeID(), "127.0.0.1:1234")
		rt.AddContact(contact)
	}

	target := RandomNodeID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rt.FindClosest(target, K)
	}
}

func BenchmarkStorageStore(b *testing.B) {
	storage := NewStorage()
	publisher := RandomNodeID()
	ttl := 1 * time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := RandomNodeID()
		value := []byte("benchmark value")
		storage.Store(key, value, ttl, publisher)
	}
}

func BenchmarkStorageGet(b *testing.B) {
	storage := NewStorage()
	publisher := RandomNodeID()
	ttl := 1 * time.Hour

	// Pre-populate
	keys := make([]NodeID, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = RandomNodeID()
		storage.Store(keys[i], []byte("value"), ttl, publisher)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.Get(keys[i%1000])
	}
}
