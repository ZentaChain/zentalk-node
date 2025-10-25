// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"fmt"

	"github.com/klauspost/reedsolomon"
)

const (
	// DataShards is the number of data shards (10)
	DataShards = 10
	// ParityShards is the number of parity shards (5)
	ParityShards = 5
	// TotalShards is the total number of shards (15)
	TotalShards = DataShards + ParityShards
	// MinShardsForRecovery is the minimum number of shards needed to reconstruct data
	MinShardsForRecovery = DataShards

	// Health thresholds for automatic repair
	// HealthExcellent: All shards available (15/15)
	HealthExcellent = 15
	// HealthGood: Minor redundancy loss (13-14/15) - monitor but don't repair yet
	HealthGood = 13
	// HealthDegraded: Significant loss (11-12/15) - trigger repair
	HealthDegraded = 11
	// HealthCritical: Minimal redundancy (10/15) - urgent repair needed
	HealthCritical = 10
)

// ErasureEncoder handles erasure coding of data
type ErasureEncoder struct {
	encoder reedsolomon.Encoder
}

// EncodedData represents data split into shards
type EncodedData struct {
	Shards     [][]byte // All 15 shards (10 data + 5 parity)
	ShardSize  int      // Size of each shard in bytes
	OriginalSize int    // Original data size in bytes
}

// NewErasureEncoder creates a new erasure encoder
func NewErasureEncoder() (*ErasureEncoder, error) {
	enc, err := reedsolomon.New(DataShards, ParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &ErasureEncoder{
		encoder: enc,
	}, nil
}

// Encode splits data into shards using Reed-Solomon encoding
// Returns 15 shards (10 data + 5 parity), where any 10 shards can reconstruct the original data
func (e *ErasureEncoder) Encode(data []byte) (*EncodedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot encode empty data")
	}

	originalSize := len(data)

	// Split data into shards
	shards, err := e.encoder.Split(data)
	if err != nil {
		return nil, fmt.Errorf("failed to split data: %w", err)
	}

	// Encode parity shards
	if err := e.encoder.Encode(shards); err != nil {
		return nil, fmt.Errorf("failed to encode parity: %w", err)
	}

	// Get shard size (all shards are the same size)
	shardSize := len(shards[0])

	return &EncodedData{
		Shards:       shards,
		ShardSize:    shardSize,
		OriginalSize: originalSize,
	}, nil
}

// Decode reconstructs original data from available shards
// Requires at least 10 of the 15 shards to succeed
// Missing shards should be nil in the input slice
func (e *ErasureEncoder) Decode(encodedData *EncodedData) ([]byte, error) {
	if encodedData == nil {
		return nil, fmt.Errorf("encoded data is nil")
	}

	if len(encodedData.Shards) != TotalShards {
		return nil, fmt.Errorf("invalid number of shards: expected %d, got %d", TotalShards, len(encodedData.Shards))
	}

	// Count available shards
	availableCount := 0
	for _, shard := range encodedData.Shards {
		if shard != nil {
			availableCount++
		}
	}

	if availableCount < MinShardsForRecovery {
		return nil, fmt.Errorf("insufficient shards for recovery: have %d, need %d", availableCount, MinShardsForRecovery)
	}

	// Make a copy of shards to avoid modifying the original
	shardsCopy := make([][]byte, TotalShards)
	copy(shardsCopy, encodedData.Shards)

	// Verify shards and reconstruct missing ones
	if err := e.encoder.Reconstruct(shardsCopy); err != nil {
		return nil, fmt.Errorf("failed to reconstruct shards: %w", err)
	}

	// Verify the reconstructed data
	ok, err := e.encoder.Verify(shardsCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to verify shards: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("shard verification failed")
	}

	// Join the data shards back together
	buf := make([]byte, 0, encodedData.OriginalSize)
	for i := 0; i < DataShards; i++ {
		buf = append(buf, shardsCopy[i]...)
	}

	// Trim to original size (remove padding)
	if len(buf) > encodedData.OriginalSize {
		buf = buf[:encodedData.OriginalSize]
	}

	return buf, nil
}

// VerifyShards checks if the shards are valid and can reconstruct data
func (e *ErasureEncoder) VerifyShards(shards [][]byte) (bool, error) {
	if len(shards) != TotalShards {
		return false, fmt.Errorf("invalid number of shards: expected %d, got %d", TotalShards, len(shards))
	}

	return e.encoder.Verify(shards)
}

// ErasureShardInfo contains metadata about erasure coding shard distribution
type ErasureShardInfo struct {
	ShardIndex    int    // Index of this shard (0-14)
	IsDataShard   bool   // True if data shard (0-9), false if parity shard (10-14)
	ShardSize     int    // Size of this shard in bytes
	OriginalSize  int    // Original data size before encoding
	TotalShards   int    // Total number of shards
	MinForRecovery int   // Minimum shards needed for recovery
}

// GetShardInfo returns metadata for a given shard index
func GetShardInfo(shardIndex int, shardSize int, originalSize int) (*ErasureShardInfo, error) {
	if shardIndex < 0 || shardIndex >= TotalShards {
		return nil, fmt.Errorf("invalid shard index: %d (must be 0-%d)", shardIndex, TotalShards-1)
	}

	return &ErasureShardInfo{
		ShardIndex:    shardIndex,
		IsDataShard:   shardIndex < DataShards,
		ShardSize:     shardSize,
		OriginalSize:  originalSize,
		TotalShards:   TotalShards,
		MinForRecovery: MinShardsForRecovery,
	}, nil
}

// CalculateRedundancy returns the redundancy factor (storage overhead)
// For 10+5 configuration, this is 1.5x (store 15 shards, need 10)
func CalculateRedundancy() float64 {
	return float64(TotalShards) / float64(DataShards)
}

// CalculateFaultTolerance returns how many shards can be lost while still recovering data
// For 10+5 configuration, can lose up to 5 shards
func CalculateFaultTolerance() int {
	return TotalShards - MinShardsForRecovery
}
