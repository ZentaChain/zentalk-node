package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zentalk/protocol/pkg/meshstorage"
)

// TestAPIUploadDownload tests the complete upload/download flow
func TestAPIUploadDownload(t *testing.T) {
	// Create test node
	ctx := context.Background()
	config := &meshstorage.NodeConfig{
		Port:    9100,
		DataDir: t.TempDir(),
	}
	node, err := meshstorage.NewDHTNode(ctx, config)
	assert.NoError(t, err)
	defer node.Close()

	// Create API server
	server, err := NewServer(node, DefaultConfig())
	assert.NoError(t, err)

	// Test data
	testData := []byte("Hello, ZenTalk Mesh Storage! This is a test message.")
	testUserAddr := "0x1234567890abcdef1234567890abcdef12345678"
	testChunkID := 42

	// Test upload
	t.Run("Upload", func(t *testing.T) {
		uploadReq := UploadRequest{
			UserAddr: testUserAddr,
			ChunkID:  testChunkID,
			Data:     base64Encode(testData),
		}

		reqBody, _ := json.Marshal(uploadReq)
		req := httptest.NewRequest("POST", "/api/v1/storage/upload", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response UploadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, testUserAddr, response.UserAddr)
		assert.Equal(t, testChunkID, response.ChunkID)
		assert.Equal(t, len(testData), response.OriginalSize)
		assert.Equal(t, 15, response.ShardCount)
	})

	// Test download
	t.Run("Download", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/storage/download/%s/%d", testUserAddr, testChunkID)
		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response DownloadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, testUserAddr, response.UserAddr)
		assert.Equal(t, testChunkID, response.ChunkID)

		// Decode and verify data
		decodedData := base64Decode(response.Data)
		assert.Equal(t, testData, decodedData)
	})

	// Test status
	t.Run("Status", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/storage/status/%s/%d", testUserAddr, testChunkID)
		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response StatusResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.True(t, response.Exists)
		assert.Greater(t, response.AvailableShards, 0)
	})
}

// TestAPINetworkEndpoints tests network information endpoints
func TestAPINetworkEndpoints(t *testing.T) {
	ctx := context.Background()
	config := &meshstorage.NodeConfig{
		Port:    9101,
		DataDir: t.TempDir(),
	}
	node, err := meshstorage.NewDHTNode(ctx, config)
	assert.NoError(t, err)
	defer node.Close()

	server, err := NewServer(node, DefaultConfig())
	assert.NoError(t, err)

	t.Run("NetworkInfo", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/network/info", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response NetworkInfoResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "zentalk-mesh-v1", response.NetworkID)
	})

	t.Run("Health", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.NotEmpty(t, response.Status)
	})

	t.Run("NodeInfo", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/node/info", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response NodeInfoResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response.Success)
		assert.NotEmpty(t, response.NodeID)
	})
}

// TestAPIRateLimiting tests rate limiting middleware
func TestAPIRateLimiting(t *testing.T) {
	ctx := context.Background()
	nodeConfig := &meshstorage.NodeConfig{
		Port:    9102,
		DataDir: t.TempDir(),
	}
	node, err := meshstorage.NewDHTNode(ctx, nodeConfig)
	assert.NoError(t, err)
	defer node.Close()

	serverConfig := &Config{
		Port:            8082,
		EnableCORS:      true,
		RateLimit:       5, // Very low limit for testing
		MaxUploadSizeMB: 10,
	}

	server, err := NewServer(node, serverConfig)
	assert.NoError(t, err)

	// Make requests until rate limited
	limitExceeded := false
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			limitExceeded = true
			break
		}
	}

	assert.True(t, limitExceeded, "Rate limit should have been exceeded")
}

// TestAPIValidation tests input validation
func TestAPIValidation(t *testing.T) {
	ctx := context.Background()
	config := &meshstorage.NodeConfig{
		Port:    9103,
		DataDir: t.TempDir(),
	}
	node, err := meshstorage.NewDHTNode(ctx, config)
	assert.NoError(t, err)
	defer node.Close()

	server, err := NewServer(node, DefaultConfig())
	assert.NoError(t, err)

	t.Run("InvalidUserAddress", func(t *testing.T) {
		uploadReq := UploadRequest{
			UserAddr: "invalid", // Not an Ethereum address
			ChunkID:  1,
			Data:     "dGVzdA==",
		}

		reqBody, _ := json.Marshal(uploadReq)
		req := httptest.NewRequest("POST", "/api/v1/storage/upload", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("EmptyData", func(t *testing.T) {
		uploadReq := UploadRequest{
			UserAddr: "0x1234567890abcdef1234567890abcdef12345678",
			ChunkID:  1,
			Data:     "", // Empty
		}

		reqBody, _ := json.Marshal(uploadReq)
		req := httptest.NewRequest("POST", "/api/v1/storage/upload", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestAPIConcurrency tests concurrent uploads
func TestAPIConcurrency(t *testing.T) {
	ctx := context.Background()
	config := &meshstorage.NodeConfig{
		Port:    9104,
		DataDir: t.TempDir(),
	}
	node, err := meshstorage.NewDHTNode(ctx, config)
	assert.NoError(t, err)
	defer node.Close()

	server, err := NewServer(node, DefaultConfig())
	assert.NoError(t, err)

	userAddr := "0x1234567890abcdef1234567890abcdef12345678"
	concurrentUploads := 10

	errors := make(chan error, concurrentUploads)

	for i := 0; i < concurrentUploads; i++ {
		go func(chunkID int) {
			data := fmt.Sprintf("Test data chunk %d", chunkID)
			uploadReq := UploadRequest{
				UserAddr: userAddr,
				ChunkID:  chunkID,
				Data:     base64Encode([]byte(data)),
			}

			reqBody, _ := json.Marshal(uploadReq)
			req := httptest.NewRequest("POST", "/api/v1/storage/upload", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("upload failed with status %d", w.Code)
			} else {
				errors <- nil
			}
		}(i)
	}

	// Wait for all uploads
	for i := 0; i < concurrentUploads; i++ {
		err := <-errors
		assert.NoError(t, err)
	}
}

// Helper functions

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(encoded string) []byte {
	decoded, _ := base64.StdEncoding.DecodeString(encoded)
	return decoded
}
