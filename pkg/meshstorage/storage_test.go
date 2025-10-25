package meshstorage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage(t *testing.T) {
	// Create temporary directory for test
	tmpDir := filepath.Join(os.TempDir(), "meshstorage_test")
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test data
	userAddr := "0xabc123"
	chunkID := 1
	data := []byte("encrypted_test_data_chunk")

	// Test StoreChunk
	err = storage.StoreChunk(userAddr, chunkID, data)
	if err != nil {
		t.Fatalf("Failed to store chunk: %v", err)
	}

	// Test GetChunk
	retrieved, err := storage.GetChunk(userAddr, chunkID)
	if err != nil {
		t.Fatalf("Failed to get chunk: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Fatalf("Retrieved data doesn't match. Got: %s, Want: %s", string(retrieved), string(data))
	}

	// Test ListChunks
	chunks, err := storage.ListChunks(userAddr)
	if err != nil {
		t.Fatalf("Failed to list chunks: %v", err)
	}

	if len(chunks) != 1 || chunks[0] != chunkID {
		t.Fatalf("ListChunks failed. Got: %v, Want: [%d]", chunks, chunkID)
	}

	// Test GetStats
	stats, err := storage.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalChunks != 1 {
		t.Fatalf("Expected 1 chunk, got %d", stats.TotalChunks)
	}

	if stats.TotalUsers != 1 {
		t.Fatalf("Expected 1 user, got %d", stats.TotalUsers)
	}

	// Test DeleteChunk
	err = storage.DeleteChunk(userAddr, chunkID)
	if err != nil {
		t.Fatalf("Failed to delete chunk: %v", err)
	}

	// Verify deletion
	_, err = storage.GetChunk(userAddr, chunkID)
	if err == nil {
		t.Fatal("Expected error when getting deleted chunk")
	}

	t.Log("All storage tests passed!")
}

func TestMultipleChunks(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "meshstorage_test_multi")
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	userAddr := "0xabc123"

	// Store multiple chunks
	for i := 0; i < 15; i++ {
		data := []byte("chunk_" + string(rune(i)))
		err := storage.StoreChunk(userAddr, i, data)
		if err != nil {
			t.Fatalf("Failed to store chunk %d: %v", i, err)
		}
	}

	// Verify all chunks
	allChunks, err := storage.GetAllChunks(userAddr)
	if err != nil {
		t.Fatalf("Failed to get all chunks: %v", err)
	}

	if len(allChunks) != 15 {
		t.Fatalf("Expected 15 chunks, got %d", len(allChunks))
	}

	// Test DeleteAllChunks
	err = storage.DeleteAllChunks(userAddr)
	if err != nil {
		t.Fatalf("Failed to delete all chunks: %v", err)
	}

	// Verify deletion
	chunks, err := storage.ListChunks(userAddr)
	if err != nil {
		t.Fatalf("Failed to list chunks: %v", err)
	}

	if len(chunks) != 0 {
		t.Fatalf("Expected 0 chunks after deletion, got %d", len(chunks))
	}

	t.Log("Multiple chunks test passed!")
}
