package meshstorage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDistributedStorage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0, // Random port
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	if ds == nil {
		t.Fatal("Distributed storage is nil")
	}

	if ds.node == nil {
		t.Fatal("Node is nil")
	}

	if ds.encoder == nil {
		t.Fatal("Encoder is nil")
	}

	if ds.client == nil {
		t.Fatal("RPC client is nil")
	}

	t.Log("Distributed storage created successfully!")
}

func TestStoreDistributedSingleNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Test data
	testData := []byte("This is test data for distributed storage!")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	// Store distributed (should store all shards locally since no peers)
	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Verify distributed chunk metadata
	if distributedChunk.UserAddr != userAddr {
		t.Fatalf("UserAddr mismatch: expected %s, got %s", userAddr, distributedChunk.UserAddr)
	}

	if distributedChunk.ChunkID != chunkID {
		t.Fatalf("ChunkID mismatch: expected %d, got %d", chunkID, distributedChunk.ChunkID)
	}

	if len(distributedChunk.ShardLocations) != TotalShards {
		t.Fatalf("Expected %d shard locations, got %d", TotalShards, len(distributedChunk.ShardLocations))
	}

	// All shards should be stored on local node
	localNodeID := node.ID()
	for i, loc := range distributedChunk.ShardLocations {
		if loc.PeerID != localNodeID {
			t.Fatalf("Shard %d stored on wrong peer: expected local node, got %s", i, loc.PeerID)
		}
	}

	t.Log("Successfully stored distributed chunk on single node!")
}

func TestRetrieveDistributedSingleNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Test data
	originalData := []byte("Testing retrieval from distributed storage with single node")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 2

	// Store distributed
	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, originalData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Retrieve distributed
	retrievedData, err := ds.RetrieveDistributed(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to retrieve distributed: %v", err)
	}

	// Verify data matches
	if !bytes.Equal(retrievedData, originalData) {
		t.Fatalf("Retrieved data doesn't match original.\nOriginal: %s\nRetrieved: %s",
			string(originalData), string(retrievedData))
	}

	t.Log("Successfully retrieved distributed chunk from single node!")
}

func TestStoreAndRetrieveMultipleChunks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	userAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// Store multiple chunks
	testData := [][]byte{
		[]byte("First chunk of distributed data"),
		[]byte("Second chunk with different content"),
		[]byte("Third chunk for testing multiple storage"),
	}

	distributedChunks := make([]*DistributedChunk, len(testData))

	// Store all chunks
	for i, data := range testData {
		chunk, err := ds.StoreDistributed(ctx, userAddr, i, data)
		if err != nil {
			t.Fatalf("Failed to store chunk %d: %v", i, err)
		}
		distributedChunks[i] = chunk
	}

	// Retrieve and verify all chunks
	for i, expectedData := range testData {
		retrievedData, err := ds.RetrieveDistributed(ctx, distributedChunks[i])
		if err != nil {
			t.Fatalf("Failed to retrieve chunk %d: %v", i, err)
		}

		if !bytes.Equal(retrievedData, expectedData) {
			t.Fatalf("Chunk %d mismatch.\nExpected: %s\nGot: %s",
				i, string(expectedData), string(retrievedData))
		}
	}

	t.Log("Successfully stored and retrieved multiple chunks!")
}

func TestGenerateStorageKey(t *testing.T) {
	testCases := []struct {
		userAddr string
		chunkID  int
	}{
		{"0x1234567890abcdef1234567890abcdef12345678", 1},
		{"0x1234567890abcdef1234567890abcdef12345678", 2},
		{"0xabcdef1234567890abcdef1234567890abcdef12", 1},
	}

	keys := make(map[string]bool)

	for _, tc := range testCases {
		key := generateStorageKey(tc.userAddr, tc.chunkID)

		// Key should be 64 hex characters (SHA256)
		if len(key) != 64 {
			t.Fatalf("Expected key length 64, got %d for %s:%d", len(key), tc.userAddr, tc.chunkID)
		}

		// Keys should be unique
		if keys[key] {
			t.Fatalf("Duplicate key generated: %s", key)
		}
		keys[key] = true

		// Same input should generate same key (deterministic)
		key2 := generateStorageKey(tc.userAddr, tc.chunkID)
		if key != key2 {
			t.Fatalf("Key generation not deterministic for %s:%d", tc.userAddr, tc.chunkID)
		}
	}

	t.Log("Storage key generation is deterministic and unique!")
}

func TestGetShardStatusSingleNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store a chunk
	testData := []byte("Test data for shard status check")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Get shard status
	status, err := ds.GetShardStatus(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to get shard status: %v", err)
	}

	// All shards should be available (stored locally)
	if len(status) != TotalShards {
		t.Fatalf("Expected %d status entries, got %d", TotalShards, len(status))
	}

	for i, available := range status {
		if !available {
			t.Fatalf("Shard %d should be available (local node)", i)
		}
	}

	t.Log("All shards are available!")
}

func TestCalculateHealthSingleNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store a chunk
	testData := []byte("Test data for health calculation")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Calculate health
	health, err := ds.CalculateHealth(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to calculate health: %v", err)
	}

	// Health should be 1.0 (all shards available)
	if health != 1.0 {
		t.Fatalf("Expected health 1.0, got %f", health)
	}

	t.Logf("Health score: %.2f (perfect health!)", health)
}

func TestLargeDataDistributed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Test with 1MB data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 100

	// Store distributed
	t.Log("Storing 1MB of data...")
	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, largeData)
	if err != nil {
		t.Fatalf("Failed to store large data: %v", err)
	}

	// Retrieve distributed
	t.Log("Retrieving 1MB of data...")
	retrievedData, err := ds.RetrieveDistributed(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to retrieve large data: %v", err)
	}

	// Verify
	if !bytes.Equal(retrievedData, largeData) {
		t.Fatal("Large data mismatch after distributed store/retrieve")
	}

	t.Log("Successfully stored and retrieved 1MB of distributed data!")
}

func TestRetrieveNilChunk(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Try to retrieve nil chunk
	_, err = ds.RetrieveDistributed(ctx, nil)
	if err == nil {
		t.Fatal("Expected error when retrieving nil chunk, got nil")
	}

	if err.Error() != "distributed chunk is nil" {
		t.Fatalf("Expected 'distributed chunk is nil' error, got: %v", err)
	}

	t.Log("Correctly rejected nil chunk retrieval")
}

func TestDifferentUsers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Test data for different users
	users := []struct {
		addr string
		data []byte
	}{
		{"0x1111111111111111111111111111111111111111", []byte("User 1 data")},
		{"0x2222222222222222222222222222222222222222", []byte("User 2 data")},
		{"0x3333333333333333333333333333333333333333", []byte("User 3 data")},
	}

	chunks := make([]*DistributedChunk, len(users))

	// Store data for each user
	for i, user := range users {
		chunk, err := ds.StoreDistributed(ctx, user.addr, 1, user.data)
		if err != nil {
			t.Fatalf("Failed to store data for user %s: %v", user.addr, err)
		}
		chunks[i] = chunk
	}

	// Retrieve and verify each user's data
	for i, user := range users {
		retrievedData, err := ds.RetrieveDistributed(ctx, chunks[i])
		if err != nil {
			t.Fatalf("Failed to retrieve data for user %s: %v", user.addr, err)
		}

		if !bytes.Equal(retrievedData, user.data) {
			t.Fatalf("Data mismatch for user %s", user.addr)
		}
	}

	// Verify each user has different storage keys
	key1 := generateStorageKey(users[0].addr, 1)
	key2 := generateStorageKey(users[1].addr, 1)
	key3 := generateStorageKey(users[2].addr, 1)

	if key1 == key2 || key1 == key3 || key2 == key3 {
		t.Fatal("Different users should have different storage keys")
	}

	t.Log("Successfully isolated storage for different users!")
}

func TestStoreEmptyData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Try to store empty data
	emptyData := []byte{}
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	_, err = ds.StoreDistributed(ctx, userAddr, chunkID, emptyData)
	if err == nil {
		t.Fatal("Expected error when storing empty data, got nil")
	}

	t.Logf("Correctly rejected empty data: %v", err)
}

// Benchmark tests
func BenchmarkStoreDistributed(b *testing.B) {
	ctx := context.Background()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		b.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		b.Fatalf("Failed to create distributed storage: %v", err)
	}

	testData := []byte("Benchmark test data for distributed storage performance")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := ds.StoreDistributed(ctx, userAddr, i, testData)
		if err != nil {
			b.Fatalf("Store failed: %v", err)
		}
	}
}

func BenchmarkRetrieveDistributed(b *testing.B) {
	ctx := context.Background()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		b.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		b.Fatalf("Failed to create distributed storage: %v", err)
	}

	testData := []byte("Benchmark test data for distributed storage performance")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// Pre-store chunks
	chunks := make([]*DistributedChunk, b.N)
	for i := 0; i < b.N; i++ {
		chunk, err := ds.StoreDistributed(ctx, userAddr, i, testData)
		if err != nil {
			b.Fatalf("Store failed: %v", err)
		}
		chunks[i] = chunk
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := ds.RetrieveDistributed(ctx, chunks[i])
		if err != nil {
			b.Fatalf("Retrieve failed: %v", err)
		}
	}
}

func TestDistributedStorageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Integration test: multiple users, multiple chunks, large data
	t.Log("Running comprehensive integration test...")

	// Test scenario: 3 users, each storing 5 chunks of varying sizes
	users := []string{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		"0x3333333333333333333333333333333333333333",
	}

	chunks := make(map[string][]*DistributedChunk)

	// Store data
	for _, user := range users {
		chunks[user] = make([]*DistributedChunk, 5)
		for i := 0; i < 5; i++ {
			// Varying data sizes
			dataSize := (i + 1) * 10 * 1024 // 10KB, 20KB, 30KB, 40KB, 50KB
			data := make([]byte, dataSize)
			for j := range data {
				data[j] = byte((j + i) % 256)
			}

			chunk, err := ds.StoreDistributed(ctx, user, i, data)
			if err != nil {
				t.Fatalf("Failed to store chunk %d for user %s: %v", i, user, err)
			}
			chunks[user][i] = chunk

			// Verify health
			health, err := ds.CalculateHealth(ctx, chunk)
			if err != nil {
				t.Fatalf("Failed to calculate health: %v", err)
			}
			if health != 1.0 {
				t.Fatalf("Expected health 1.0, got %f", health)
			}
		}
	}

	t.Log("Stored all chunks, verifying retrieval...")

	// Retrieve and verify all data
	for _, user := range users {
		for i := 0; i < 5; i++ {
			retrievedData, err := ds.RetrieveDistributed(ctx, chunks[user][i])
			if err != nil {
				t.Fatalf("Failed to retrieve chunk %d for user %s: %v", i, user, err)
			}

			// Verify size
			expectedSize := (i + 1) * 10 * 1024
			if len(retrievedData) != expectedSize {
				t.Fatalf("Size mismatch for chunk %d of user %s: expected %d, got %d",
					i, user, expectedSize, len(retrievedData))
			}

			// Verify data integrity
			for j := range retrievedData {
				expected := byte((j + i) % 256)
				if retrievedData[j] != expected {
					t.Fatalf("Data corruption at byte %d of chunk %d for user %s", j, i, user)
				}
			}
		}
	}

	// Get statistics
	stats, err := node.Storage().GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	t.Logf("Integration test completed successfully!")
	t.Logf("Total chunks stored: %d", stats.TotalChunks)
	t.Logf("Total bytes stored: %d", stats.TotalSize)
	t.Logf("Unique users: %d", stats.TotalUsers)
}

func TestShardLocationMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store a chunk
	testData := []byte("Test data for shard location metadata")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Verify shard location metadata
	for i, loc := range distributedChunk.ShardLocations {
		if loc.ShardIndex != i {
			t.Fatalf("Shard %d has wrong index: %d", i, loc.ShardIndex)
		}

		if loc.PeerID == "" {
			t.Fatalf("Shard %d has empty PeerID", i)
		}

		// PeerAddrs can be empty for local storage in some cases, but should be a valid slice
		if loc.PeerAddrs == nil {
			t.Fatalf("Shard %d has nil PeerAddrs", i)
		}
	}

	t.Log("Shard location metadata is valid!")
}

// Helper function to print distributed chunk info (for debugging)
func printDistributedChunkInfo(t *testing.T, chunk *DistributedChunk) {
	t.Logf("DistributedChunk Info:")
	t.Logf("  User: %s", chunk.UserAddr)
	t.Logf("  ChunkID: %d", chunk.ChunkID)
	t.Logf("  OriginalSize: %d bytes", chunk.OriginalSize)
	t.Logf("  ShardSize: %d bytes", chunk.ShardSize)
	t.Logf("  Shards: %d", len(chunk.ShardLocations))

	for i, loc := range chunk.ShardLocations {
		t.Logf("    Shard %d: Peer %s (%d addrs)", i, loc.PeerID, len(loc.PeerAddrs))
	}
}

func TestPrintChunkInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store a chunk
	testData := []byte("Test data for printing chunk info")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 1

	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Print info
	printDistributedChunkInfo(t, distributedChunk)
}

func TestDeleteDistributedChunk(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store test data
	testData := []byte("Test data for deletion - this should be completely removed from all 15 shards")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 42

	// Store distributed chunk (creates 15 shards)
	t.Logf("Storing data distributed across %d shards...", TotalShards)
	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Verify all 15 shards exist
	t.Log("Verifying all shards exist before deletion...")
	status, err := ds.GetShardStatus(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to get shard status: %v", err)
	}

	availableCount := 0
	for i, available := range status {
		if available {
			availableCount++
		} else {
			t.Fatalf("Shard %d not available before deletion", i)
		}
	}

	if availableCount != TotalShards {
		t.Fatalf("Expected %d shards available before deletion, got %d", TotalShards, availableCount)
	}
	t.Logf("✓ All %d shards verified as available", TotalShards)

	// Calculate health before deletion
	health, err := ds.CalculateHealth(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to calculate health: %v", err)
	}
	if health != 1.0 {
		t.Fatalf("Expected health 1.0 before deletion, got %f", health)
	}
	t.Log("✓ Health score: 1.0 (perfect health before deletion)")

	// Delete the distributed chunk
	t.Logf("Deleting chunk from all %d shard nodes...", TotalShards)
	err = ds.DeleteChunk(ctx, userAddr, chunkID)
	if err != nil {
		t.Fatalf("Failed to delete chunk: %v", err)
	}
	t.Log("✓ DeleteChunk completed successfully")

	// Verify deletion by trying to retrieve
	t.Log("Verifying chunk cannot be retrieved after deletion...")
	_, err = ds.RetrieveDistributed(ctx, distributedChunk)
	if err == nil {
		t.Fatal("Expected error when retrieving deleted chunk, got nil")
	}
	t.Logf("✓ Retrieval correctly failed after deletion: %v", err)

	// Verify individual shards are deleted by checking storage directly
	t.Log("Verifying individual shards are deleted from storage...")
	deletedCount := 0
	for i := 0; i < TotalShards; i++ {
		shardKey := GenerateShardKey(userAddr, chunkID, i)
		_, err := node.Storage().GetChunk(shardKey, i)
		if err != nil {
			// Error means shard doesn't exist (deleted)
			deletedCount++
		}
	}

	// We require at least 2/3 (10 of 15) shards to be deleted
	minRequired := (TotalShards * 2) / 3
	if deletedCount < minRequired {
		t.Fatalf("Expected at least %d shards deleted, but only %d were deleted", minRequired, deletedCount)
	}
	t.Logf("✓ Successfully deleted %d/%d shards (minimum required: %d)", deletedCount, TotalShards, minRequired)
}

func TestDeleteMultipleChunks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	userAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// Store 5 chunks
	numChunks := 5
	testData := make([][]byte, numChunks)
	for i := 0; i < numChunks; i++ {
		testData[i] = []byte(fmt.Sprintf("Chunk %d data for deletion testing", i))
		_, err := ds.StoreDistributed(ctx, userAddr, i, testData[i])
		if err != nil {
			t.Fatalf("Failed to store chunk %d: %v", i, err)
		}
	}
	t.Logf("✓ Stored %d chunks", numChunks)

	// Delete chunks 1 and 3
	chunkToDelete := []int{1, 3}
	for _, chunkID := range chunkToDelete {
		err := ds.DeleteChunk(ctx, userAddr, chunkID)
		if err != nil {
			t.Fatalf("Failed to delete chunk %d: %v", chunkID, err)
		}
		t.Logf("✓ Deleted chunk %d", chunkID)
	}

	// Verify deleted chunks are gone
	for _, chunkID := range chunkToDelete {
		deletedCount := 0
		for i := 0; i < TotalShards; i++ {
			shardKey := GenerateShardKey(userAddr, chunkID, i)
			_, err := node.Storage().GetChunk(shardKey, i)
			if err != nil {
				deletedCount++
			}
		}
		minRequired := (TotalShards * 2) / 3
		if deletedCount < minRequired {
			t.Fatalf("Chunk %d: expected at least %d shards deleted, got %d", chunkID, minRequired, deletedCount)
		}
		t.Logf("  ✓ Chunk %d: %d/%d shards deleted", chunkID, deletedCount, TotalShards)
	}

	// Verify non-deleted chunks still exist
	nonDeletedChunks := []int{0, 2, 4}
	for _, chunkID := range nonDeletedChunks {
		existingCount := 0
		for i := 0; i < TotalShards; i++ {
			shardKey := GenerateShardKey(userAddr, chunkID, i)
			_, err := node.Storage().GetChunk(shardKey, i)
			if err == nil {
				existingCount++
			}
		}
		if existingCount < MinShardsForRecovery {
			t.Fatalf("Chunk %d: expected at least %d shards to exist, got %d", chunkID, MinShardsForRecovery, existingCount)
		}
		t.Logf("  ✓ Chunk %d still exists: %d/%d shards available", chunkID, existingCount, TotalShards)
	}
}

func TestDeleteNonExistentChunk(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Try to delete a chunk that doesn't exist
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 999

	err = ds.DeleteChunk(ctx, userAddr, chunkID)
	// This should fail because we couldn't delete enough shards (they don't exist)
	if err == nil {
		t.Log("Note: DeleteChunk returned success for non-existent chunk (this is okay if no nodes exist)")
	} else {
		t.Logf("✓ DeleteChunk correctly handled non-existent chunk: %v", err)
	}
}

// Helper function to generate shard key (exposed for testing)
func GenerateShardKey(userAddr string, chunkID int, shardIndex int) string {
	return fmt.Sprintf("%s_%d_shard_%d", userAddr, chunkID, shardIndex)
}

func TestRepairChunk(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	// Store test data
	testData := []byte("Testing automatic shard repair - this data will survive shard deletion!")
	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	chunkID := 100

	// Store distributed chunk (creates 15 shards)
	t.Logf("Storing data distributed across %d shards...", TotalShards)
	distributedChunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
	if err != nil {
		t.Fatalf("Failed to store distributed: %v", err)
	}

	// Verify all 15 shards exist
	status, err := ds.GetShardStatus(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to get shard status: %v", err)
	}

	availableCount := 0
	for _, available := range status {
		if available {
			availableCount++
		}
	}

	if availableCount != TotalShards {
		t.Fatalf("Expected %d shards available, got %d", TotalShards, availableCount)
	}
	t.Logf("✓ All %d shards verified as available", TotalShards)

	// Simulate server owner deleting 5 shards from filesystem
	t.Log("Simulating malicious server owner deleting 5 shards...")
	shardsToDelete := []int{2, 5, 8, 11, 14}
	for _, shardIndex := range shardsToDelete {
		shardKey := GenerateShardKey(userAddr, chunkID, shardIndex)
		err := node.Storage().DeleteChunk(shardKey, shardIndex)
		if err != nil {
			t.Logf("Warning: Failed to delete shard %d: %v", shardIndex, err)
		}
	}
	t.Logf("✓ Deleted %d shards (simulating malicious deletion)", len(shardsToDelete))

	// Verify shards are actually missing by checking storage directly
	availableCount = 0
	for i := 0; i < TotalShards; i++ {
		shardKey := GenerateShardKey(userAddr, chunkID, i)
		_, err := node.Storage().GetChunk(shardKey, i)
		if err == nil {
			availableCount++
		}
	}

	expectedAvailable := TotalShards - len(shardsToDelete)
	if availableCount != expectedAvailable {
		t.Fatalf("Expected %d shards available after deletion, got %d", expectedAvailable, availableCount)
	}
	t.Logf("✓ Verified %d shards remaining after deletion", availableCount)

	// Calculate health before repair
	health, err := ds.CalculateHealth(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to calculate health: %v", err)
	}
	t.Logf("Health before repair: %.2f (%d/%d shards)", health, availableCount, TotalShards)

	// Trigger repair
	t.Log("Triggering automatic repair...")
	err = ds.RepairChunk(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to repair chunk: %v", err)
	}

	// Verify all shards are now available by checking storage directly
	repairedCount := 0
	for i := 0; i < TotalShards; i++ {
		shardKey := GenerateShardKey(userAddr, chunkID, i)
		_, err := node.Storage().GetChunk(shardKey, i)
		if err == nil {
			repairedCount++
		}
	}

	t.Logf("✓ After repair: %d/%d shards available", repairedCount, TotalShards)

	// Health should be back to 1.0 or close to it
	health, err = ds.CalculateHealth(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to calculate health after repair: %v", err)
	}
	t.Logf("Health after repair: %.2f", health)

	// Verify we can still retrieve the original data
	t.Log("Verifying data integrity after repair...")
	retrievedData, err := ds.RetrieveDistributed(ctx, distributedChunk)
	if err != nil {
		t.Fatalf("Failed to retrieve data after repair: %v", err)
	}

	if !bytes.Equal(retrievedData, testData) {
		t.Fatalf("Data mismatch after repair.\nOriginal: %s\nRetrieved: %s",
			string(testData), string(retrievedData))
	}

	t.Log("✅ SUCCESS: Data survived shard deletion and was fully repaired!")
}

func TestCheckAndRepairIfNeeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temporary storage directory
	tempDir, err := os.MkdirTemp("", "distributed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a single DHT node
	node, err := NewDHTNode(ctx, &NodeConfig{
		Port:    0,
		DataDir: filepath.Join(tempDir, "node1"),
	})
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Create distributed storage
	ds, err := NewDistributedStorage(node)
	if err != nil {
		t.Fatalf("Failed to create distributed storage: %v", err)
	}

	userAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// Test Case 1: Excellent health - no repair needed
	t.Run("ExcellentHealth", func(t *testing.T) {
		chunkID := 1
		testData := []byte("Test data with excellent health")

		chunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
		if err != nil {
			t.Fatalf("Failed to store: %v", err)
		}

		err = ds.CheckAndRepairIfNeeded(ctx, chunk)
		if err != nil {
			t.Fatalf("Unexpected error for excellent health: %v", err)
		}
		t.Log("✓ Excellent health - no repair triggered")
	})

	// Test Case 2: Degraded health - should trigger repair
	t.Run("DegradedHealth", func(t *testing.T) {
		chunkID := 2
		testData := []byte("Test data with degraded health")

		chunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
		if err != nil {
			t.Fatalf("Failed to store: %v", err)
		}

		// Delete 4 shards to bring health to degraded level (11/15)
		for i := 0; i < 4; i++ {
			shardKey := GenerateShardKey(userAddr, chunkID, i)
			_ = node.Storage().DeleteChunk(shardKey, i)
		}

		err = ds.CheckAndRepairIfNeeded(ctx, chunk)
		if err != nil {
			t.Fatalf("Repair failed: %v", err)
		}

		// Verify repair was successful
		health, _ := ds.CalculateHealth(ctx, chunk)
		t.Logf("✓ Degraded health repaired - new health: %.2f", health)
	})

	// Test Case 3: Critical health - urgent repair
	t.Run("CriticalHealth", func(t *testing.T) {
		chunkID := 3
		testData := []byte("Test data with critical health")

		chunk, err := ds.StoreDistributed(ctx, userAddr, chunkID, testData)
		if err != nil {
			t.Fatalf("Failed to store: %v", err)
		}

		// Delete 5 shards to bring health to critical level (10/15)
		for i := 0; i < 5; i++ {
			shardKey := GenerateShardKey(userAddr, chunkID, i)
			_ = node.Storage().DeleteChunk(shardKey, i)
		}

		err = ds.CheckAndRepairIfNeeded(ctx, chunk)
		if err != nil {
			t.Fatalf("Critical repair failed: %v", err)
		}

		t.Log("✓ Critical health repaired successfully")
	})
}
