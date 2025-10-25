package meshstorage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRPCPing(t *testing.T) {
	ctx := context.Background()

	// Create two nodes
	tmpDir1 := filepath.Join(os.TempDir(), "meshstorage_rpc_ping1")
	tmpDir2 := filepath.Join(os.TempDir(), "meshstorage_rpc_ping2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create first node
	config1 := &NodeConfig{
		Port:           11001,
		DataDir:        tmpDir1,
		BootstrapPeers: []string{},
	}

	node1, err := NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	// Setup RPC handler on node1
	handler1 := NewRPCHandler(node1)
	handler1.SetupStreamHandler()

	// Create second node
	config2 := &NodeConfig{
		Port:           11002,
		DataDir:        tmpDir2,
		BootstrapPeers: []string{},
	}

	node2, err := NewDHTNode(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	// Connect node2 to node1
	addrs := node1.Addresses()
	peerAddr := addrs[0].String() + "/p2p/" + node1.ID().String()
	if err := node2.Connect(ctx, peerAddr); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	// Create RPC client for node2
	client := NewRPCClient(node2)

	// Test ping
	if err := client.Ping(ctx, node1.ID()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	t.Log("RPC ping test passed!")
}

func TestRPCStoreAndGetChunk(t *testing.T) {
	ctx := context.Background()

	// Create two nodes
	tmpDir1 := filepath.Join(os.TempDir(), "meshstorage_rpc_store1")
	tmpDir2 := filepath.Join(os.TempDir(), "meshstorage_rpc_store2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create first node
	config1 := &NodeConfig{
		Port:           11003,
		DataDir:        tmpDir1,
		BootstrapPeers: []string{},
	}

	node1, err := NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	// Setup RPC handler on node1
	handler1 := NewRPCHandler(node1)
	handler1.SetupStreamHandler()

	// Create second node
	config2 := &NodeConfig{
		Port:           11004,
		DataDir:        tmpDir2,
		BootstrapPeers: []string{},
	}

	node2, err := NewDHTNode(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	// Setup RPC handler on node2
	handler2 := NewRPCHandler(node2)
	handler2.SetupStreamHandler()

	// Connect node2 to node1
	addrs := node1.Addresses()
	peerAddr := addrs[0].String() + "/p2p/" + node1.ID().String()
	if err := node2.Connect(ctx, peerAddr); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	// Create RPC client for node2
	client := NewRPCClient(node2)

	// Test data
	userAddr := "0xrpctest"
	chunkID := 99
	data := []byte("encrypted_rpc_test_data")

	// Store chunk on node1 from node2
	if err := client.StoreChunk(ctx, node1.ID(), userAddr, chunkID, data); err != nil {
		t.Fatalf("StoreChunk failed: %v", err)
	}

	// Verify chunk was stored on node1
	stored, err := node1.Storage().GetChunk(userAddr, chunkID)
	if err != nil {
		t.Fatalf("Failed to get chunk from node1: %v", err)
	}

	if string(stored) != string(data) {
		t.Fatalf("Data mismatch. Got: %s, Want: %s", string(stored), string(data))
	}

	// Get chunk from node1 using RPC
	retrieved, err := client.GetChunk(ctx, node1.ID(), userAddr, chunkID)
	if err != nil {
		t.Fatalf("GetChunk failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Fatalf("Retrieved data mismatch. Got: %s, Want: %s", string(retrieved), string(data))
	}

	t.Log("RPC store and get chunk test passed!")
}

func TestRPCMultipleChunks(t *testing.T) {
	ctx := context.Background()

	// Create two nodes
	tmpDir1 := filepath.Join(os.TempDir(), "meshstorage_rpc_multi1")
	tmpDir2 := filepath.Join(os.TempDir(), "meshstorage_rpc_multi2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create first node
	config1 := &NodeConfig{
		Port:           11005,
		DataDir:        tmpDir1,
		BootstrapPeers: []string{},
	}

	node1, err := NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	// Setup RPC handler on node1
	handler1 := NewRPCHandler(node1)
	handler1.SetupStreamHandler()

	// Create second node
	config2 := &NodeConfig{
		Port:           11006,
		DataDir:        tmpDir2,
		BootstrapPeers: []string{},
	}

	node2, err := NewDHTNode(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	// Connect node2 to node1
	addrs := node1.Addresses()
	peerAddr := addrs[0].String() + "/p2p/" + node1.ID().String()
	if err := node2.Connect(ctx, peerAddr); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	// Create RPC client for node2
	client := NewRPCClient(node2)

	// Store multiple chunks on node1
	userAddr := "0xmultitest"
	chunkCount := 10

	for i := 0; i < chunkCount; i++ {
		data := []byte("chunk_" + string(rune('0'+i)))
		if err := client.StoreChunk(ctx, node1.ID(), userAddr, i, data); err != nil {
			t.Fatalf("Failed to store chunk %d: %v", i, err)
		}
	}

	// Verify all chunks can be retrieved
	for i := 0; i < chunkCount; i++ {
		retrieved, err := client.GetChunk(ctx, node1.ID(), userAddr, i)
		if err != nil {
			t.Fatalf("Failed to get chunk %d: %v", i, err)
		}

		expected := "chunk_" + string(rune('0'+i))
		if string(retrieved) != expected {
			t.Fatalf("Chunk %d mismatch. Got: %s, Want: %s", i, string(retrieved), expected)
		}
	}

	// Verify chunk count on node1
	chunks, err := node1.Storage().ListChunks(userAddr)
	if err != nil {
		t.Fatalf("Failed to list chunks: %v", err)
	}

	if len(chunks) != chunkCount {
		t.Fatalf("Expected %d chunks, got %d", chunkCount, len(chunks))
	}

	t.Log("RPC multiple chunks test passed!")
}
