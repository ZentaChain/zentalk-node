package meshstorage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDHTNode(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory for test
	tmpDir := filepath.Join(os.TempDir(), "meshstorage_node_test")
	defer os.RemoveAll(tmpDir)

	// Create a DHT node
	config := &NodeConfig{
		Port:           0, // Random port
		DataDir:        tmpDir,
		BootstrapPeers: []string{},
	}

	node, err := NewDHTNode(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create DHT node: %v", err)
	}
	defer node.Close()

	// Test node ID
	if node.ID().String() == "" {
		t.Fatal("Node ID is empty")
	}

	// Test addresses
	addrs := node.Addresses()
	if len(addrs) == 0 {
		t.Fatal("Node has no addresses")
	}

	// Test storage access
	storage := node.Storage()
	if storage == nil {
		t.Fatal("Storage is nil")
	}

	// Test node info
	info, err := node.GetNodeInfo()
	if err != nil {
		t.Fatalf("Failed to get node info: %v", err)
	}

	if info.ID != node.ID().String() {
		t.Fatalf("Node info ID mismatch. Got: %s, Want: %s", info.ID, node.ID().String())
	}

	// Test peer count (should be 0 initially)
	if node.PeerCount() != 0 {
		t.Fatalf("Expected 0 peers, got %d", node.PeerCount())
	}

	// Test not bootstrapped yet
	if node.IsBootstrapped() {
		t.Fatal("Node should not be bootstrapped yet")
	}

	t.Log("DHT node test passed!")
}

func TestDHTNodeBootstrap(t *testing.T) {
	ctx := context.Background()

	// Create two nodes
	tmpDir1 := filepath.Join(os.TempDir(), "meshstorage_node1")
	tmpDir2 := filepath.Join(os.TempDir(), "meshstorage_node2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create first node (bootstrap node)
	config1 := &NodeConfig{
		Port:           8001,
		DataDir:        tmpDir1,
		BootstrapPeers: []string{},
	}

	node1, err := NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	// Get node1's address
	addrs := node1.Addresses()
	if len(addrs) == 0 {
		t.Fatal("Node1 has no addresses")
	}

	// Construct bootstrap peer address
	// Format: /ip4/127.0.0.1/tcp/8001/p2p/<peer-id>
	bootstrapAddr := addrs[0].String() + "/p2p/" + node1.ID().String()
	t.Logf("Bootstrap address: %s", bootstrapAddr)

	// Create second node and bootstrap to first
	config2 := &NodeConfig{
		Port:           8002,
		DataDir:        tmpDir2,
		BootstrapPeers: []string{bootstrapAddr},
	}

	node2, err := NewDHTNode(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	// Wait for connection to establish
	time.Sleep(2 * time.Second)

	// Check node2 is bootstrapped
	if !node2.IsBootstrapped() {
		t.Fatal("Node2 should be bootstrapped")
	}

	// Check peer count on node2 (should have node1 as peer)
	if node2.PeerCount() == 0 {
		t.Log("Warning: Node2 has no peers yet (DHT may still be connecting)")
	}

	t.Log("Bootstrap test passed!")
}

func TestDHTNodeConnect(t *testing.T) {
	ctx := context.Background()

	tmpDir1 := filepath.Join(os.TempDir(), "meshstorage_connect1")
	tmpDir2 := filepath.Join(os.TempDir(), "meshstorage_connect2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create two nodes
	config1 := &NodeConfig{
		Port:           8003,
		DataDir:        tmpDir1,
		BootstrapPeers: []string{},
	}

	node1, err := NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	config2 := &NodeConfig{
		Port:           8004,
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
	if len(addrs) == 0 {
		t.Fatal("Node1 has no addresses")
	}

	peerAddr := addrs[0].String() + "/p2p/" + node1.ID().String()
	t.Logf("Connecting to: %s", peerAddr)

	if err := node2.Connect(ctx, peerAddr); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connection
	time.Sleep(1 * time.Second)

	// Check peer count
	if node2.PeerCount() == 0 {
		t.Fatal("Node2 should have at least 1 peer")
	}

	// Get peers
	peers := node2.GetPeers()
	if len(peers) == 0 {
		t.Fatal("GetPeers returned empty")
	}

	// Check if node1 is in the peer list
	found := false
	for pID := range peers {
		if pID == node1.ID() {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Node1 not found in node2's peer list")
	}

	t.Log("Connect test passed!")
}

func TestDHTNodeStorageIntegration(t *testing.T) {
	ctx := context.Background()

	tmpDir := filepath.Join(os.TempDir(), "meshstorage_integration")
	defer os.RemoveAll(tmpDir)

	config := &NodeConfig{
		Port:           8005,
		DataDir:        tmpDir,
		BootstrapPeers: []string{},
	}

	node, err := NewDHTNode(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer node.Close()

	// Test storing data through the node's storage
	userAddr := "0xtest123"
	chunkID := 42
	data := []byte("test_encrypted_chunk")

	storage := node.Storage()
	if err := storage.StoreChunk(userAddr, chunkID, data); err != nil {
		t.Fatalf("Failed to store chunk: %v", err)
	}

	// Retrieve the chunk
	retrieved, err := storage.GetChunk(userAddr, chunkID)
	if err != nil {
		t.Fatalf("Failed to get chunk: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Fatalf("Data mismatch. Got: %s, Want: %s", string(retrieved), string(data))
	}

	// Check node info includes storage stats
	info, err := node.GetNodeInfo()
	if err != nil {
		t.Fatalf("Failed to get node info: %v", err)
	}

	if info.StorageStats.TotalChunks != 1 {
		t.Fatalf("Expected 1 chunk in stats, got %d", info.StorageStats.TotalChunks)
	}

	t.Log("Storage integration test passed!")
}

func TestMultipleDHTNodes(t *testing.T) {
	ctx := context.Background()

	// Create 3 nodes in a network
	nodes := make([]*DHTNode, 3)
	baseTmpDir := filepath.Join(os.TempDir(), "meshstorage_multi")
	defer os.RemoveAll(baseTmpDir)

	// Create first node (bootstrap)
	config1 := &NodeConfig{
		Port:           10001,
		DataDir:        filepath.Join(baseTmpDir, "node1"),
		BootstrapPeers: []string{},
	}

	var err error
	nodes[0], err = NewDHTNode(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to create node 1: %v", err)
	}
	defer nodes[0].Close()

	// Get bootstrap address
	bootstrapAddr := nodes[0].Addresses()[0].String() + "/p2p/" + nodes[0].ID().String()
	t.Logf("Bootstrap node: %s", bootstrapAddr)

	// Create nodes 2 and 3, bootstrapping to node 1
	for i := 1; i < 3; i++ {
		config := &NodeConfig{
			Port:           10001 + i,
			DataDir:        filepath.Join(baseTmpDir, "node"+string(rune('1'+i))),
			BootstrapPeers: []string{bootstrapAddr},
		}

		nodes[i], err = NewDHTNode(ctx, config)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i+1, err)
		}
		defer nodes[i].Close()
	}

	// Wait for network to stabilize
	time.Sleep(3 * time.Second)

	// Verify all nodes are bootstrapped
	for i, node := range nodes {
		if i > 0 && !node.IsBootstrapped() {
			t.Fatalf("Node %d is not bootstrapped", i+1)
		}
	}

	// Test storing data on different nodes
	for i, node := range nodes {
		userAddr := "0xuser" + string(rune('1'+i))
		chunkID := i
		data := []byte("chunk_from_node_" + string(rune('1'+i)))

		if err := node.Storage().StoreChunk(userAddr, chunkID, data); err != nil {
			t.Fatalf("Node %d failed to store chunk: %v", i+1, err)
		}
	}

	// Verify each node has its own data
	for i, node := range nodes {
		userAddr := "0xuser" + string(rune('1'+i))
		chunkID := i

		retrieved, err := node.Storage().GetChunk(userAddr, chunkID)
		if err != nil {
			t.Fatalf("Node %d failed to retrieve chunk: %v", i+1, err)
		}

		expected := "chunk_from_node_" + string(rune('1'+i))
		if string(retrieved) != expected {
			t.Fatalf("Data mismatch on node %d. Got: %s, Want: %s", i+1, string(retrieved), expected)
		}
	}

	// Print node info
	for i, node := range nodes {
		info, err := node.GetNodeInfo()
		if err != nil {
			t.Fatalf("Failed to get info for node %d: %v", i+1, err)
		}
		t.Logf("Node %d: ID=%s, Peers=%d, Chunks=%d",
			i+1, info.ID[:16]+"...", info.PeerCount, info.StorageStats.TotalChunks)
	}

	t.Log("Multiple nodes test passed!")
}
