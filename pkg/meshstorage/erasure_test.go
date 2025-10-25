package meshstorage

import (
	"bytes"
	"fmt"
	"testing"
)

func TestErasureEncoderCreation(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	if encoder == nil {
		t.Fatal("Encoder is nil")
	}

	t.Log("Erasure encoder created successfully!")
}

func TestBasicEncodeAndDecode(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Test data
	originalData := []byte("This is test data for Reed-Solomon erasure coding in ZenTalk mesh storage!")

	// Encode
	encoded, err := encoder.Encode(originalData)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Verify we have correct number of shards
	if len(encoded.Shards) != TotalShards {
		t.Fatalf("Expected %d shards, got %d", TotalShards, len(encoded.Shards))
	}

	// Verify original size is preserved
	if encoded.OriginalSize != len(originalData) {
		t.Fatalf("Expected original size %d, got %d", len(originalData), encoded.OriginalSize)
	}

	// Decode with all shards available
	decoded, err := encoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Verify decoded data matches original
	if !bytes.Equal(decoded, originalData) {
		t.Fatalf("Decoded data doesn't match original.\nOriginal: %s\nDecoded: %s", string(originalData), string(decoded))
	}

	t.Log("Basic encode/decode test passed!")
}

func TestRecoveryWithMissingShards(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	originalData := []byte("Testing recovery with missing shards in distributed storage system")

	// Encode
	encoded, err := encoder.Encode(originalData)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Test with different numbers of missing shards
	testCases := []struct {
		name          string
		missingShard  int // Number of shards to remove
		shouldSucceed bool
	}{
		{"No missing shards", 0, true},
		{"1 missing shard", 1, true},
		{"3 missing shards", 3, true},
		{"5 missing shards (max tolerance)", 5, true},
		{"6 missing shards (should fail)", 6, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy of encoded data
			encodedCopy := &EncodedData{
				Shards:       make([][]byte, len(encoded.Shards)),
				ShardSize:    encoded.ShardSize,
				OriginalSize: encoded.OriginalSize,
			}
			copy(encodedCopy.Shards, encoded.Shards)

			// Remove specified number of shards (set to nil)
			for i := 0; i < tc.missingShard && i < len(encodedCopy.Shards); i++ {
				encodedCopy.Shards[i] = nil
			}

			// Try to decode
			decoded, err := encoder.Decode(encodedCopy)

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected successful decode, got error: %v", err)
				}

				if !bytes.Equal(decoded, originalData) {
					t.Fatalf("Decoded data doesn't match original with %d missing shards", tc.missingShard)
				}

				t.Logf("Successfully recovered data with %d missing shards", tc.missingShard)
			} else {
				if err == nil {
					t.Fatalf("Expected decode to fail with %d missing shards, but it succeeded", tc.missingShard)
				}
				t.Logf("Correctly failed to decode with %d missing shards: %v", tc.missingShard, err)
			}
		})
	}
}

func TestRecoveryWithMixedShardLoss(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	originalData := []byte("Testing mixed shard loss patterns - data and parity shards")

	// Encode
	encoded, err := encoder.Encode(originalData)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	testCases := []struct {
		name             string
		missingIndices   []int // Specific shard indices to remove
		shouldSucceed    bool
	}{
		{
			name:           "Lose first 5 data shards",
			missingIndices: []int{0, 1, 2, 3, 4},
			shouldSucceed:  true,
		},
		{
			name:           "Lose all 5 parity shards",
			missingIndices: []int{10, 11, 12, 13, 14},
			shouldSucceed:  true,
		},
		{
			name:           "Lose mix of data and parity",
			missingIndices: []int{1, 3, 5, 11, 13},
			shouldSucceed:  true,
		},
		{
			name:           "Lose last 3 data + 2 parity",
			missingIndices: []int{7, 8, 9, 10, 11},
			shouldSucceed:  true,
		},
		{
			name:           "Lose 6 random shards (should fail)",
			missingIndices: []int{0, 2, 4, 6, 8, 10},
			shouldSucceed:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy
			encodedCopy := &EncodedData{
				Shards:       make([][]byte, len(encoded.Shards)),
				ShardSize:    encoded.ShardSize,
				OriginalSize: encoded.OriginalSize,
			}
			copy(encodedCopy.Shards, encoded.Shards)

			// Remove specific shards
			for _, idx := range tc.missingIndices {
				if idx < len(encodedCopy.Shards) {
					encodedCopy.Shards[idx] = nil
				}
			}

			// Try to decode
			decoded, err := encoder.Decode(encodedCopy)

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected successful decode, got error: %v", err)
				}

				if !bytes.Equal(decoded, originalData) {
					t.Fatalf("Decoded data doesn't match original")
				}

				t.Logf("Successfully recovered with missing indices: %v", tc.missingIndices)
			} else {
				if err == nil {
					t.Fatalf("Expected decode to fail, but it succeeded")
				}
				t.Logf("Correctly failed with missing indices %v: %v", tc.missingIndices, err)
			}
		})
	}
}

func TestLargeDataEncoding(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Test with larger data (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// Encode
	encoded, err := encoder.Encode(largeData)
	if err != nil {
		t.Fatalf("Failed to encode large data: %v", err)
	}

	// Remove 5 shards (maximum tolerance)
	for i := 0; i < 5; i++ {
		encoded.Shards[i] = nil
	}

	// Decode
	decoded, err := encoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode large data: %v", err)
	}

	// Verify
	if !bytes.Equal(decoded, largeData) {
		t.Fatal("Large data decode mismatch")
	}

	t.Logf("Successfully encoded/decoded 1MB data with 5 missing shards")
}

func TestEmptyData(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Test with empty data
	emptyData := []byte{}

	_, err = encoder.Encode(emptyData)
	if err == nil {
		t.Fatal("Expected error when encoding empty data, got nil")
	}

	t.Logf("Correctly rejected empty data: %v", err)
}

func TestVerifyShards(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	originalData := []byte("Test data for shard verification")

	// Encode
	encoded, err := encoder.Encode(originalData)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Verify valid shards
	ok, err := encoder.VerifyShards(encoded.Shards)
	if err != nil {
		t.Fatalf("Failed to verify shards: %v", err)
	}
	if !ok {
		t.Fatal("Valid shards failed verification")
	}

	// Corrupt a shard and verify it fails
	corruptedShards := make([][]byte, len(encoded.Shards))
	copy(corruptedShards, encoded.Shards)
	if len(corruptedShards[0]) > 0 {
		corruptedShards[0][0] ^= 0xFF // Flip bits
	}

	ok, err = encoder.VerifyShards(corruptedShards)
	if err != nil {
		t.Fatalf("Failed to run verification on corrupted shards: %v", err)
	}
	if ok {
		t.Fatal("Corrupted shards passed verification (should have failed)")
	}

	t.Log("Shard verification test passed!")
}

func TestShardInfo(t *testing.T) {
	testCases := []struct {
		index       int
		shouldBeData bool
	}{
		{0, true},
		{5, true},
		{9, true},
		{10, false},
		{14, false},
	}

	for _, tc := range testCases {
		info, err := GetShardInfo(tc.index, 1024, 10240)
		if err != nil {
			t.Fatalf("Failed to get shard info for index %d: %v", tc.index, err)
		}

		if info.IsDataShard != tc.shouldBeData {
			t.Fatalf("Shard %d: expected IsDataShard=%v, got %v", tc.index, tc.shouldBeData, info.IsDataShard)
		}

		if info.ShardIndex != tc.index {
			t.Fatalf("Shard index mismatch: expected %d, got %d", tc.index, info.ShardIndex)
		}

		if info.TotalShards != TotalShards {
			t.Fatalf("Expected %d total shards, got %d", TotalShards, info.TotalShards)
		}
	}

	// Test invalid index
	_, err := GetShardInfo(15, 1024, 10240)
	if err == nil {
		t.Fatal("Expected error for invalid shard index, got nil")
	}

	t.Log("Shard info test passed!")
}

func TestCalculations(t *testing.T) {
	// Test redundancy calculation
	redundancy := CalculateRedundancy()
	expected := 1.5
	if redundancy != expected {
		t.Fatalf("Expected redundancy %f, got %f", expected, redundancy)
	}

	// Test fault tolerance calculation
	faultTolerance := CalculateFaultTolerance()
	expectedFT := 5
	if faultTolerance != expectedFT {
		t.Fatalf("Expected fault tolerance %d, got %d", expectedFT, faultTolerance)
	}

	t.Logf("Redundancy: %.1fx", redundancy)
	t.Logf("Fault Tolerance: %d shards", faultTolerance)
	t.Log("Calculation tests passed!")
}

func TestMultipleEncodeDecode(t *testing.T) {
	encoder, err := NewErasureEncoder()
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Test encoding and decoding multiple different data sets
	testData := [][]byte{
		[]byte("First piece of data"),
		[]byte("Second piece with different content"),
		[]byte("Third piece - some encrypted chat history"),
		[]byte(fmt.Sprintf("Fourth piece with special chars: %s", string([]byte{0x00, 0x01, 0xFF, 0xFE}))),
		[]byte("Fifth and final test piece"),
	}

	for i, data := range testData {
		// Encode
		encoded, err := encoder.Encode(data)
		if err != nil {
			t.Fatalf("Failed to encode data %d: %v", i, err)
		}

		// Remove some shards (different pattern for each)
		for j := 0; j < (i % 6); j++ {
			if j < len(encoded.Shards) {
				encoded.Shards[j] = nil
			}
		}

		// Decode
		decoded, err := encoder.Decode(encoded)
		if err != nil {
			t.Fatalf("Failed to decode data %d: %v", i, err)
		}

		// Verify
		if !bytes.Equal(decoded, data) {
			t.Fatalf("Data %d mismatch after encode/decode", i)
		}
	}

	t.Log("Multiple encode/decode test passed!")
}
