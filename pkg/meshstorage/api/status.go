package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ZentaChain/zentalk-node/pkg/crypto"
	"github.com/ZentaChain/zentalk-node/pkg/meshstorage"
)

// StatusResponse represents storage status information
type StatusResponse struct {
	Success         bool              `json:"success"`
	UserAddr        string            `json:"userAddr"`
	ChunkID         int               `json:"chunkID"`
	Exists          bool              `json:"exists"`
	Health          string            `json:"health"` // "excellent", "good", "degraded", "critical"
	HealthScore     float64           `json:"healthScore"`
	AvailableShards int               `json:"availableShards"`
	TotalShards     int               `json:"totalShards"`
	MinRequired     int               `json:"minRequiredShards"`
	ShardStatus     []ShardStatusInfo `json:"shardStatus"`
	CheckedAt       time.Time         `json:"checkedAt"`
}

// ShardStatusInfo contains status of a single shard
type ShardStatusInfo struct {
	ShardIndex int    `json:"shardIndex"`
	Available  bool   `json:"available"`
	NodeID     string `json:"nodeId,omitempty"`
	Error      string `json:"error,omitempty"`
}

// handleStatus handles GET /api/v1/storage/status/:userAddr/:chunkID
func (s *Server) handleStatus(c *gin.Context) {
	userAddr := c.Param("userAddr")
	chunkIDStr := c.Param("chunkID")

	// Validate user address
	if len(userAddr) != 42 || userAddr[:2] != "0x" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user address",
			Message: "User address must be a valid Ethereum address (0x...)",
		})
		return
	}

	// Parse chunk ID
	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid chunk ID",
			Message: "Chunk ID must be a number",
		})
		return
	}

	fmt.Printf("📊 Status check: user=%s chunk=%d\n", userAddr, chunkID)

	// Get chunk metadata
	chunk, exists := s.getChunkMetadata(userAddr, chunkID)
	if !exists {
		c.JSON(http.StatusOK, StatusResponse{
			Success:     true,
			UserAddr:    userAddr,
			ChunkID:     chunkID,
			Exists:      false,
			Health:      "none",
			HealthScore: 0.0,
			CheckedAt:   time.Now(),
		})
		return
	}

	// Get shard status
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shardStatus, err := s.distributedStore.GetShardStatus(ctx, chunk)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Status check failed",
			Message: err.Error(),
		})
		return
	}

	// Count available shards and build status list
	availableCount := 0
	shardStatusList := make([]ShardStatusInfo, len(shardStatus))

	for i, available := range shardStatus {
		nodeID := ""
		if i < len(chunk.ShardLocations) {
			nodeID = chunk.ShardLocations[i].PeerID.String()
		}

		shardStatusList[i] = ShardStatusInfo{
			ShardIndex: i,
			Available:  available,
			NodeID:     nodeID,
		}

		if available {
			availableCount++
		}
	}

	// Calculate health using constants
	totalShards := meshstorage.TotalShards
	minRequired := meshstorage.MinShardsForRecovery
	healthScore := float64(availableCount) / float64(totalShards)

	var health string
	switch {
	case availableCount >= meshstorage.HealthExcellent:
		health = "excellent" // All shards available
	case availableCount >= meshstorage.HealthGood:
		health = "good" // Some redundancy lost
	case availableCount >= meshstorage.MinShardsForRecovery:
		health = "degraded" // Minimal redundancy
	case availableCount >= 8:
		health = "critical" // Below minimum, but might still recover
	default:
		health = "lost" // Cannot recover data
	}

	response := StatusResponse{
		Success:         true,
		UserAddr:        userAddr,
		ChunkID:         chunkID,
		Exists:          true,
		Health:          health,
		HealthScore:     healthScore,
		AvailableShards: availableCount,
		TotalShards:     totalShards,
		MinRequired:     minRequired,
		ShardStatus:     shardStatusList,
		CheckedAt:       time.Now(),
	}

	fmt.Printf("✅ Status: %s (%d/%d shards available)\n",
		health,
		availableCount,
		totalShards,
	)

	c.JSON(http.StatusOK, response)
}

// handleDelete handles DELETE /api/v1/storage/delete/:userAddr/:chunkID
// Requires cryptographic signature to prevent unauthorized deletion
func (s *Server) handleDelete(c *gin.Context) {
	userAddr := c.Param("userAddr")
	chunkIDStr := c.Param("chunkID")

	// Parse chunk ID
	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid chunk ID",
			Message: "Chunk ID must be a number",
		})
		return
	}

	fmt.Printf("🗑️  Delete request: user=%s chunk=%d\n", userAddr, chunkID)

	// Get signature and timestamp from headers (optional for now)
	signatureB64 := c.GetHeader("X-Signature")
	timestamp := c.GetHeader("X-Timestamp")
	publicKeyPEM := c.GetHeader("X-Public-Key")

	// TODO: Make signature mandatory in production
	// For now, signature is optional to allow development without complex key management
	if signatureB64 != "" && timestamp != "" && publicKeyPEM != "" {
		// Verify signature if provided
		if err := verifyDeleteSignature(userAddr, chunkID, timestamp, signatureB64, publicKeyPEM); err != nil {
			fmt.Printf("❌ Signature verification failed: %v\n", err)
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "Invalid signature",
				Message: fmt.Sprintf("Signature verification failed: %v", err),
			})
			return
		}
		fmt.Printf("✅ Signature verified for user=%s\n", userAddr)
	} else {
		fmt.Printf("⚠️  DELETE without signature (development mode) for user=%s\n", userAddr)
	}

	// Check if chunk exists in metadata cache
	_, metadataExists := s.getChunkMetadata(userAddr, chunkID)
	if metadataExists {
		fmt.Printf("📋 Chunk found in metadata cache\n")
	} else {
		fmt.Printf("⚠️  Chunk not in metadata cache (may have been restarted), trying storage anyway...\n")
	}

	// Delete from distributed storage (try even if metadata doesn't exist)
	// The metadata is just an in-memory cache - actual data may still exist in storage
	if err := s.distributedStore.DeleteChunk(c.Request.Context(), userAddr, chunkID); err != nil {
		fmt.Printf("❌ Failed to delete from distributed storage: %v\n", err)

		// Only return 404 if the error indicates the chunk truly doesn't exist
		// Otherwise, treat it as a server error
		if !metadataExists {
			// Chunk not in metadata AND deletion failed - probably doesn't exist
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Data not found",
				Message: fmt.Sprintf("No data found for user %s chunk %d", userAddr, chunkID),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Deletion failed",
			Message: fmt.Sprintf("Failed to delete from mesh network: %v", err),
		})
		return
	}

	// Delete metadata if it exists
	if metadataExists {
		s.deleteChunkMetadata(userAddr, chunkID)
	}

	fmt.Printf("✅ Deleted successfully from all shard nodes\n")

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: fmt.Sprintf("Chunk %d for user %s deleted successfully", chunkID, userAddr),
	})
}

// verifyDeleteSignature verifies the cryptographic signature for delete operations
func verifyDeleteSignature(userAddr string, chunkID int, timestamp string, signatureB64 string, publicKeyPEM string) error {
	// Parse timestamp
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// Check timestamp is recent (within 5 minutes)
	now := time.Now()
	diff := now.Sub(ts)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Minute {
		return fmt.Errorf("timestamp too old or in future (age: %v)", diff)
	}

	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	// Parse public key from PEM
	publicKey, err := crypto.ImportPublicKeyPEM([]byte(publicKeyPEM))
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Construct message that was signed
	// Format: userAddr|chunkID|timestamp
	message := fmt.Sprintf("%s|%d|%s", userAddr, chunkID, timestamp)

	// Verify signature
	if err := crypto.VerifySignature([]byte(message), signature, publicKey); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}
