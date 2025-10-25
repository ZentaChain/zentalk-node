// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// DistributedStorage manages distributed storage across the mesh network
type DistributedStorage struct {
	node    *DHTNode
	encoder *ErasureEncoder
	client  *RPCClient
	mu      sync.RWMutex

	// Health monitoring
	monitorInterval time.Duration
	monitorStop     chan struct{}
	monitorWg       sync.WaitGroup
	chunks          map[string]*DistributedChunk // Track chunks for monitoring
	chunksMu        sync.RWMutex
}

// NewDistributedStorage creates a new distributed storage manager
func NewDistributedStorage(node *DHTNode) (*DistributedStorage, error) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		return nil, fmt.Errorf("failed to create erasure encoder: %w", err)
	}

	client := NewRPCClient(node)

	ds := &DistributedStorage{
		node:            node,
		encoder:         encoder,
		client:          client,
		monitorInterval: 10 * time.Minute, // Check health every 10 minutes
		monitorStop:     make(chan struct{}),
		chunks:          make(map[string]*DistributedChunk),
	}

	// Start background health monitoring
	ds.StartMonitoring()

	return ds, nil
}

// ShardLocation represents where a shard is stored
type ShardLocation struct {
	ShardIndex int       // Index of the shard (0-14)
	PeerID     peer.ID   // Peer storing this shard
	PeerAddrs  []string  // Peer addresses
}

// DistributedChunk represents a chunk distributed across the network
type DistributedChunk struct {
	UserAddr      string          // User's Ethereum address
	ChunkID       int             // Chunk identifier
	OriginalSize  int             // Original data size
	ShardSize     int             // Size of each shard
	ShardLocations []ShardLocation // Where each shard is stored
}

// StoreDistributed encodes data and distributes shards across the network
func (ds *DistributedStorage) StoreDistributed(ctx context.Context, userAddr string, chunkID int, data []byte) (*DistributedChunk, error) {
	// Encode data into shards
	encoded, err := ds.encoder.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	// Generate a deterministic key for finding storage nodes
	key := generateStorageKey(userAddr, chunkID)

	// Find nodes to store shards
	targetPeers, err := ds.findStorageNodes(ctx, key, TotalShards)
	if err != nil {
		return nil, fmt.Errorf("failed to find storage nodes: %w", err)
	}

	// If we don't have enough peers, store locally and on available peers
	if len(targetPeers) < TotalShards {
		// Store remaining shards locally
		for i := len(targetPeers); i < TotalShards; i++ {
			shardKey := fmt.Sprintf("%s_%d_shard_%d", userAddr, chunkID, i)
			if err := ds.node.Storage().StoreChunk(shardKey, i, encoded.Shards[i]); err != nil {
				return nil, fmt.Errorf("failed to store local shard %d: %w", i, err)
			}
		}

		// Add local node to target peers for the remaining shards
		for i := len(targetPeers); i < TotalShards; i++ {
			targetPeers = append(targetPeers, ds.node.ID())
		}
	}

	// Distribute shards to peers
	shardLocations := make([]ShardLocation, TotalShards)
	var wg sync.WaitGroup
	errChan := make(chan error, TotalShards)

	for i := 0; i < TotalShards; i++ {
		wg.Add(1)
		go func(shardIndex int) {
			defer wg.Done()

			targetPeer := targetPeers[shardIndex]
			shardKey := fmt.Sprintf("%s_%d_shard_%d", userAddr, chunkID, shardIndex)

			// If it's the local node, store locally
			if targetPeer == ds.node.ID() {
				if err := ds.node.Storage().StoreChunk(shardKey, shardIndex, encoded.Shards[shardIndex]); err != nil {
					errChan <- fmt.Errorf("failed to store local shard %d: %w", shardIndex, err)
					return
				}
			} else {
				// Store on remote peer via RPC
				if err := ds.client.StoreChunk(ctx, targetPeer, shardKey, shardIndex, encoded.Shards[shardIndex]); err != nil {
					errChan <- fmt.Errorf("failed to store shard %d on peer %s: %w", shardIndex, targetPeer, err)
					return
				}
			}

			// Record shard location
			peerAddrs := ds.node.Host().Peerstore().Addrs(targetPeer)
			addrs := make([]string, len(peerAddrs))
			for j, addr := range peerAddrs {
				addrs[j] = addr.String()
			}

			shardLocations[shardIndex] = ShardLocation{
				ShardIndex: shardIndex,
				PeerID:     targetPeer,
				PeerAddrs:  addrs,
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// If we failed to store more than 5 shards, return error
		if len(errs) > ParityShards {
			return nil, fmt.Errorf("failed to store %d shards (too many failures): %v", len(errs), errs)
		}
		// Otherwise, just log the errors but continue (we have redundancy)
		fmt.Printf("Warning: failed to store %d shards, but continuing due to redundancy: %v\n", len(errs), errs)
	}

	chunk := &DistributedChunk{
		UserAddr:       userAddr,
		ChunkID:        chunkID,
		OriginalSize:   encoded.OriginalSize,
		ShardSize:      encoded.ShardSize,
		ShardLocations: shardLocations,
	}

	// Register chunk for automatic health monitoring
	ds.RegisterChunk(chunk)

	return chunk, nil
}

// RetrieveDistributed retrieves and reconstructs data from distributed shards
func (ds *DistributedStorage) RetrieveDistributed(ctx context.Context, distributedChunk *DistributedChunk) ([]byte, error) {
	if distributedChunk == nil {
		return nil, fmt.Errorf("distributed chunk is nil")
	}

	// Prepare encoded data structure
	encoded := &EncodedData{
		Shards:       make([][]byte, TotalShards),
		ShardSize:    distributedChunk.ShardSize,
		OriginalSize: distributedChunk.OriginalSize,
	}

	// Retrieve shards from peers in parallel
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	successCount := 0

	for _, location := range distributedChunk.ShardLocations {
		wg.Add(1)
		go func(loc ShardLocation) {
			defer wg.Done()

			shardKey := fmt.Sprintf("%s_%d_shard_%d", distributedChunk.UserAddr, distributedChunk.ChunkID, loc.ShardIndex)

			var shard []byte
			var err error

			// If it's the local node, retrieve locally
			if loc.PeerID == ds.node.ID() {
				shard, err = ds.node.Storage().GetChunk(shardKey, loc.ShardIndex)
			} else {
				// Retrieve from remote peer via RPC
				shard, err = ds.client.GetChunk(ctx, loc.PeerID, shardKey, loc.ShardIndex)
			}

			if err != nil {
				fmt.Printf("Failed to retrieve shard %d from peer %s: %v\n", loc.ShardIndex, loc.PeerID, err)
				return
			}

			mu.Lock()
			encoded.Shards[loc.ShardIndex] = shard
			successCount++
			mu.Unlock()
		}(location)
	}

	wg.Wait()

	// Check if we have enough shards to reconstruct
	if successCount < MinShardsForRecovery {
		return nil, fmt.Errorf("insufficient shards retrieved: have %d, need %d", successCount, MinShardsForRecovery)
	}

	// Decode the data
	data, err := ds.encoder.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return data, nil
}

// findStorageNodes finds the best nodes to store shards based on DHT proximity
func (ds *DistributedStorage) findStorageNodes(ctx context.Context, key string, count int) ([]peer.ID, error) {
	// Use the DHT to find closest nodes to the key
	closestPeers, err := ds.node.FindClosestNodes(ctx, key, count)
	if err != nil {
		return nil, fmt.Errorf("failed to find closest nodes: %w", err)
	}

	// Extract peer IDs
	peerIDs := make([]peer.ID, 0, len(closestPeers))
	for _, peerInfo := range closestPeers {
		// Don't include ourselves unless necessary
		if peerInfo.ID != ds.node.ID() {
			peerIDs = append(peerIDs, peerInfo.ID)
		}
	}

	// If we don't have enough peers, include ourselves
	if len(peerIDs) < count {
		peerIDs = append(peerIDs, ds.node.ID())
	}

	return peerIDs, nil
}

// generateStorageKey creates a deterministic key for DHT lookups
func generateStorageKey(userAddr string, chunkID int) string {
	data := fmt.Sprintf("%s:%d", userAddr, chunkID)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GetShardStatus returns the status of all shards for a distributed chunk
func (ds *DistributedStorage) GetShardStatus(ctx context.Context, distributedChunk *DistributedChunk) ([]bool, error) {
	status := make([]bool, TotalShards)
	var wg sync.WaitGroup
	mu := &sync.Mutex{}

	for _, location := range distributedChunk.ShardLocations {
		wg.Add(1)
		go func(loc ShardLocation) {
			defer wg.Done()

			// Try to ping the peer
			var available bool
			if loc.PeerID == ds.node.ID() {
				// Local node is always available
				available = true
			} else {
				// Check if peer is reachable
				err := ds.client.Ping(ctx, loc.PeerID)
				available = (err == nil)
			}

			mu.Lock()
			status[loc.ShardIndex] = available
			mu.Unlock()
		}(location)
	}

	wg.Wait()

	return status, nil
}

// CalculateHealth returns a health score for the distributed chunk (0.0 - 1.0)
func (ds *DistributedStorage) CalculateHealth(ctx context.Context, distributedChunk *DistributedChunk) (float64, error) {
	status, err := ds.GetShardStatus(ctx, distributedChunk)
	if err != nil {
		return 0, err
	}

	availableCount := 0
	for _, available := range status {
		if available {
			availableCount++
		}
	}

	return float64(availableCount) / float64(TotalShards), nil
}

// DeleteChunk deletes a chunk from all distributed shard nodes
func (ds *DistributedStorage) DeleteChunk(ctx context.Context, userAddr string, chunkID int) error {
	// Create deletion key
	key := fmt.Sprintf("%s:%d", userAddr, chunkID)

	// Find the nodes that should have stored this chunk
	// This returns unique peers, but we need to map them to all 15 shards
	storageNodes, err := ds.findStorageNodes(ctx, key, TotalShards)
	if err != nil {
		return fmt.Errorf("failed to find storage nodes: %w", err)
	}

	// Build shard-to-node mapping (same logic as StoreDistributed)
	// If we don't have enough unique peers, local node stores remaining shards
	shardNodes := make([]peer.ID, TotalShards)
	for i := 0; i < TotalShards; i++ {
		if i < len(storageNodes) {
			shardNodes[i] = storageNodes[i]
		} else {
			// Remaining shards are on local node
			shardNodes[i] = ds.node.ID()
		}
	}

	// Delete each of the 15 shards
	successCount := 0
	var lastErr error

	for shardIndex := 0; shardIndex < TotalShards; shardIndex++ {
		peerID := shardNodes[shardIndex]

		// If it's the local node, delete locally
		if peerID == ds.node.ID() {
			shardKey := fmt.Sprintf("%s_%d_shard_%d", userAddr, chunkID, shardIndex)
			err := ds.node.Storage().DeleteChunk(shardKey, shardIndex)
			if err != nil {
				fmt.Printf("⚠️  Failed to delete local shard %d: %v\n", shardIndex, err)
				lastErr = err
				continue
			}
		} else {
			// Send delete RPC to remote shard node
			err := ds.client.DeleteShard(ctx, peerID, userAddr, chunkID, shardIndex)
			if err != nil {
				fmt.Printf("⚠️  Failed to delete shard %d from %s: %v\n", shardIndex, peerID, err)
				lastErr = err
				continue
			}
		}
		successCount++
	}

	// Require at least 2/3 of shards deleted
	minRequired := (TotalShards * 2) / 3
	if successCount < minRequired {
		return fmt.Errorf("failed to delete enough shards (%d/%d deleted, %d required): %w",
			successCount, TotalShards, minRequired, lastErr)
	}

	fmt.Printf("✅ Deleted chunk from %d/%d shard nodes\n", successCount, TotalShards)

	// Unregister chunk from monitoring
	ds.UnregisterChunk(userAddr, chunkID)

	return nil
}

// RepairChunk repairs a degraded chunk by recreating missing shards
// This is called when shard count drops below HealthDegraded threshold
func (ds *DistributedStorage) RepairChunk(ctx context.Context, distributedChunk *DistributedChunk) error {
	if distributedChunk == nil {
		return fmt.Errorf("distributed chunk is nil")
	}

	// Check current shard status
	status, err := ds.GetShardStatus(ctx, distributedChunk)
	if err != nil {
		return fmt.Errorf("failed to get shard status: %w", err)
	}

	// Count available shards
	availableCount := 0
	availableShards := make([]int, 0, TotalShards)
	missingShards := make([]int, 0, TotalShards)

	for i, available := range status {
		if available {
			availableCount++
			availableShards = append(availableShards, i)
		} else {
			missingShards = append(missingShards, i)
		}
	}

	// Check if repair is needed
	if availableCount >= HealthExcellent {
		fmt.Printf("✅ Chunk health excellent (%d/%d shards), no repair needed\n", availableCount, TotalShards)
		return nil
	}

	if availableCount < MinShardsForRecovery {
		return fmt.Errorf("insufficient shards for recovery: have %d, need %d", availableCount, MinShardsForRecovery)
	}

	fmt.Printf("🔧 Repairing chunk: %d/%d shards available, %d missing\n", availableCount, TotalShards, len(missingShards))

	// Step 1: Retrieve available shards
	encoded := &EncodedData{
		Shards:       make([][]byte, TotalShards),
		ShardSize:    distributedChunk.ShardSize,
		OriginalSize: distributedChunk.OriginalSize,
	}

	var retrieveMu sync.Mutex
	var wg sync.WaitGroup
	retrievedCount := 0

	for _, shardIndex := range availableShards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			location := distributedChunk.ShardLocations[idx]
			shardKey := fmt.Sprintf("%s_%d_shard_%d", distributedChunk.UserAddr, distributedChunk.ChunkID, idx)

			var shard []byte
			var err error

			if location.PeerID == ds.node.ID() {
				shard, err = ds.node.Storage().GetChunk(shardKey, idx)
			} else {
				shard, err = ds.client.GetChunk(ctx, location.PeerID, shardKey, idx)
			}

			if err != nil {
				fmt.Printf("⚠️  Failed to retrieve shard %d: %v\n", idx, err)
				return
			}

			retrieveMu.Lock()
			encoded.Shards[idx] = shard
			retrievedCount++
			retrieveMu.Unlock()
		}(shardIndex)
	}

	wg.Wait()

	if retrievedCount < MinShardsForRecovery {
		return fmt.Errorf("failed to retrieve enough shards: got %d, need %d", retrievedCount, MinShardsForRecovery)
	}

	fmt.Printf("✅ Retrieved %d shards for reconstruction\n", retrievedCount)

	// Step 2: Reconstruct missing shards using erasure coding
	err = ds.encoder.encoder.Reconstruct(encoded.Shards)
	if err != nil {
		return fmt.Errorf("failed to reconstruct shards: %w", err)
	}

	fmt.Printf("✅ Reconstructed %d missing shards\n", len(missingShards))

	// Step 3: Find new storage nodes for missing shards
	key := generateStorageKey(distributedChunk.UserAddr, distributedChunk.ChunkID)
	storageNodes, err := ds.findStorageNodes(ctx, key, TotalShards)
	if err != nil {
		return fmt.Errorf("failed to find storage nodes: %w", err)
	}

	// Build shard-to-node mapping
	shardNodes := make([]peer.ID, TotalShards)
	for i := 0; i < TotalShards; i++ {
		if i < len(storageNodes) {
			shardNodes[i] = storageNodes[i]
		} else {
			shardNodes[i] = ds.node.ID()
		}
	}

	// Step 4: Store recreated shards on new nodes
	successCount := 0
	var storeMu sync.Mutex
	var storeWg sync.WaitGroup

	for _, shardIndex := range missingShards {
		storeWg.Add(1)
		go func(idx int) {
			defer storeWg.Done()

			targetPeer := shardNodes[idx]
			shardKey := fmt.Sprintf("%s_%d_shard_%d", distributedChunk.UserAddr, distributedChunk.ChunkID, idx)

			var err error
			if targetPeer == ds.node.ID() {
				err = ds.node.Storage().StoreChunk(shardKey, idx, encoded.Shards[idx])
			} else {
				err = ds.client.StoreChunk(ctx, targetPeer, shardKey, idx, encoded.Shards[idx])
			}

			if err != nil {
				fmt.Printf("⚠️  Failed to store repaired shard %d: %v\n", idx, err)
				return
			}

			storeMu.Lock()
			successCount++
			// Update shard location in metadata
			peerAddrs := ds.node.Host().Peerstore().Addrs(targetPeer)
			addrs := make([]string, len(peerAddrs))
			for j, addr := range peerAddrs {
				addrs[j] = addr.String()
			}
			distributedChunk.ShardLocations[idx] = ShardLocation{
				ShardIndex: idx,
				PeerID:     targetPeer,
				PeerAddrs:  addrs,
			}
			storeMu.Unlock()
		}(shardIndex)
	}

	storeWg.Wait()

	if successCount == 0 {
		return fmt.Errorf("failed to store any repaired shards")
	}

	fmt.Printf("✅ Repair complete: stored %d/%d missing shards\n", successCount, len(missingShards))
	fmt.Printf("📊 New health: %d/%d shards available\n", availableCount+successCount, TotalShards)

	return nil
}

// CheckAndRepairIfNeeded checks chunk health and repairs if below threshold
func (ds *DistributedStorage) CheckAndRepairIfNeeded(ctx context.Context, distributedChunk *DistributedChunk) error {
	// Calculate current health
	health, err := ds.CalculateHealth(ctx, distributedChunk)
	if err != nil {
		return fmt.Errorf("failed to calculate health: %w", err)
	}

	availableShards := int(health * float64(TotalShards))

	// Determine if repair is needed
	if availableShards >= HealthGood {
		// Health is good, no repair needed
		return nil
	}

	if availableShards >= HealthDegraded {
		fmt.Printf("⚠️  Chunk health degraded (%d/%d shards), triggering repair...\n", availableShards, TotalShards)
		return ds.RepairChunk(ctx, distributedChunk)
	}

	if availableShards >= HealthCritical {
		fmt.Printf("🚨 Chunk health CRITICAL (%d/%d shards), urgent repair needed!\n", availableShards, TotalShards)
		return ds.RepairChunk(ctx, distributedChunk)
	}

	// Below critical threshold - cannot recover
	return fmt.Errorf("chunk health too low for repair: %d/%d shards (need at least %d)", availableShards, TotalShards, HealthCritical)
}

// RegisterChunk registers a chunk for health monitoring
func (ds *DistributedStorage) RegisterChunk(chunk *DistributedChunk) {
	if chunk == nil {
		return
	}

	key := fmt.Sprintf("%s:%d", chunk.UserAddr, chunk.ChunkID)

	ds.chunksMu.Lock()
	ds.chunks[key] = chunk
	ds.chunksMu.Unlock()

	fmt.Printf("📋 Registered chunk for monitoring: %s\n", key)
}

// UnregisterChunk removes a chunk from health monitoring
func (ds *DistributedStorage) UnregisterChunk(userAddr string, chunkID int) {
	key := fmt.Sprintf("%s:%d", userAddr, chunkID)

	ds.chunksMu.Lock()
	delete(ds.chunks, key)
	ds.chunksMu.Unlock()

	fmt.Printf("📋 Unregistered chunk from monitoring: %s\n", key)
}

// StartMonitoring starts the background health monitoring goroutine
func (ds *DistributedStorage) StartMonitoring() {
	ds.monitorWg.Add(1)
	go ds.monitorLoop()
	fmt.Printf("🔍 Started background health monitoring (interval: %v)\n", ds.monitorInterval)
}

// StopMonitoring stops the background health monitoring
func (ds *DistributedStorage) StopMonitoring() {
	close(ds.monitorStop)
	ds.monitorWg.Wait()
	fmt.Printf("🔍 Stopped background health monitoring\n")
}

// monitorLoop is the main monitoring loop that runs in the background
func (ds *DistributedStorage) monitorLoop() {
	defer ds.monitorWg.Done()

	ticker := time.NewTicker(ds.monitorInterval)
	defer ticker.Stop()

	fmt.Printf("🔍 Health monitor started\n")

	for {
		select {
		case <-ticker.C:
			ds.checkAllChunks()
		case <-ds.monitorStop:
			fmt.Printf("🔍 Health monitor stopping...\n")
			return
		}
	}
}

// checkAllChunks checks health of all registered chunks and repairs if needed
func (ds *DistributedStorage) checkAllChunks() {
	ds.chunksMu.RLock()
	chunks := make([]*DistributedChunk, 0, len(ds.chunks))
	for _, chunk := range ds.chunks {
		chunks = append(chunks, chunk)
	}
	ds.chunksMu.RUnlock()

	if len(chunks) == 0 {
		return
	}

	fmt.Printf("\n🔍 Health check starting for %d chunks...\n", len(chunks))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	for _, chunk := range chunks {
		wg.Add(1)
		go func(c *DistributedChunk) {
			defer wg.Done()

			key := fmt.Sprintf("%s:%d", c.UserAddr, c.ChunkID)

			// Calculate health
			health, err := ds.CalculateHealth(ctx, c)
			if err != nil {
				fmt.Printf("⚠️  %s: failed to check health: %v\n", key, err)
				return
			}

			availableShards := int(health * float64(TotalShards))

			// Check if repair is needed
			if availableShards >= HealthGood {
				// Health is good
				fmt.Printf("✅ %s: health excellent (%d/%d shards)\n", key, availableShards, TotalShards)
				return
			}

			if availableShards >= HealthDegraded {
				fmt.Printf("⚠️  %s: health degraded (%d/%d shards), triggering repair...\n", key, availableShards, TotalShards)
				if err := ds.RepairChunk(ctx, c); err != nil {
					fmt.Printf("❌ %s: repair failed: %v\n", key, err)
				}
				return
			}

			if availableShards >= HealthCritical {
				fmt.Printf("🚨 %s: health CRITICAL (%d/%d shards), urgent repair!\n", key, availableShards, TotalShards)
				if err := ds.RepairChunk(ctx, c); err != nil {
					fmt.Printf("❌ %s: critical repair failed: %v\n", key, err)
				}
				return
			}

			// Below critical - data may be lost
			fmt.Printf("💀 %s: health too low (%d/%d shards), cannot recover\n", key, availableShards, TotalShards)
		}(chunk)
	}

	wg.Wait()
	fmt.Printf("🔍 Health check completed\n\n")
}

// SetMonitorInterval changes the monitoring interval
func (ds *DistributedStorage) SetMonitorInterval(interval time.Duration) {
	ds.monitorInterval = interval
	fmt.Printf("🔍 Monitor interval changed to %v\n", interval)
}
