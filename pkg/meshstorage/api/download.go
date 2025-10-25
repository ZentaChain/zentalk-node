package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ZentaChain/zentalk-node/pkg/meshstorage"
)

// DownloadResponse represents a successful download response
type DownloadResponse struct {
	Success      bool      `json:"success"`
	UserAddr     string    `json:"userAddr"`
	ChunkID      int       `json:"chunkID"`
	Data         string    `json:"data"` // Base64 encoded
	SizeBytes    int       `json:"sizeBytes"`
	ShardsUsed   int       `json:"shardsUsed"`
	ShardsTotal  int       `json:"shardsTotal"`
	DownloadedAt time.Time `json:"downloadedAt"`
}

// handleDownload handles GET /api/v1/storage/download/:userAddr/:chunkID
func (s *Server) handleDownload(c *gin.Context) {
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

	// Get decryption parameters from query or headers
	signature := c.Query("signature")     // From query: ?signature=0x...
	password := c.Query("password")       // From query: ?password=...
	if signature == "" {
		signature = c.GetHeader("X-Signature") // From header
	}
	if password == "" {
		password = c.GetHeader("X-Password") // From header
	}

	fmt.Printf("📥 Download request: user=%s chunk=%d (signature: %v, password: %v)\n",
		userAddr, chunkID, signature != "", password != "")

	// Get chunk metadata
	chunk, exists := s.getChunkMetadata(userAddr, chunkID)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Data not found",
			Message: fmt.Sprintf("No data found for user %s chunk %d", userAddr, chunkID),
		})
		return
	}

	// Retrieve from distributed storage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()

	encryptedData, err := s.distributedStore.RetrieveDistributed(ctx, chunk)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)

		// Check if data not found vs other errors
		if err.Error() == "chunk not found" || err.Error() == "insufficient shards" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Data not found",
				Message: fmt.Sprintf("Failed to retrieve data for user %s chunk %d", userAddr, chunkID),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Retrieval failed",
			Message: err.Error(),
		})
		return
	}

	downloadDuration := time.Since(startTime)

	// Decrypt the data
	var decryptedData []byte

	// Parse encrypted data from JSON
	var encrypted meshstorage.EncryptedData
	if err := json.Unmarshal(encryptedData, &encrypted); err != nil {
		// Data might not be encrypted (backward compatibility)
		fmt.Printf("⚠️  Data not in encrypted format, returning as-is\n")
		decryptedData = encryptedData
	} else {
		// Data is encrypted, decrypt it
		var encryptionKey *meshstorage.EncryptionKey

		if signature != "" {
			// Derive key from signature
			encryptionKey, err = meshstorage.DeriveKeyFromSignature(signature)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "Invalid signature",
					Message: fmt.Sprintf("Failed to derive key: %v", err),
				})
				return
			}
			fmt.Printf("🔓 Decrypting with signature-derived key\n")
		} else if password != "" {
			// Decrypt with password
			decryptedData, err = meshstorage.DecryptWithPassword(&encrypted, password)
			if err != nil {
				c.JSON(http.StatusUnauthorized, ErrorResponse{
					Error:   "Decryption failed",
					Message: "Wrong password or corrupted data",
				})
				return
			}
			fmt.Printf("🔓 Decrypted with password\n")
		} else {
			// Default: Use wallet address
			encryptionKey, err = meshstorage.DeriveKeyFromWalletAddress(userAddr)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Key derivation failed",
					Message: err.Error(),
				})
				return
			}
			fmt.Printf("🔓 Decrypting with wallet-derived key\n")
		}

		// Decrypt with derived key (if not already decrypted with password)
		if encryptionKey != nil {
			decryptedData, err = meshstorage.Decrypt(&encrypted, encryptionKey)
			if err != nil {
				c.JSON(http.StatusUnauthorized, ErrorResponse{
					Error:   "Decryption failed",
					Message: "Wrong key or corrupted data",
				})
				return
			}
		}

		fmt.Printf("✅ Decrypted: %d bytes → %d bytes\n", len(encryptedData), len(decryptedData))
	}

	// Encode data as base64
	encodedData := base64.StdEncoding.EncodeToString(decryptedData)

	response := DownloadResponse{
		Success:      true,
		UserAddr:     userAddr,
		ChunkID:      chunkID,
		Data:         encodedData,
		SizeBytes:    len(decryptedData),
		ShardsUsed:   10, // Minimum needed for recovery
		ShardsTotal:  15, // Total distributed
		DownloadedAt: time.Now(),
	}

	fmt.Printf("✅ Download successful: %d bytes decrypted (%.2fs)\n",
		len(decryptedData),
		downloadDuration.Seconds(),
	)

	c.JSON(http.StatusOK, response)
}

// handleDownloadBinary handles binary file downloads
// GET /api/v1/storage/download/:userAddr/:chunkID/binary
func (s *Server) handleDownloadBinary(c *gin.Context) {
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

	fmt.Printf("📥 Download (binary): user=%s chunk=%d\n", userAddr, chunkID)

	// Get chunk metadata
	chunk, exists := s.getChunkMetadata(userAddr, chunkID)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Data not found",
			Message: fmt.Sprintf("No data found for user %s chunk %d", userAddr, chunkID),
		})
		return
	}

	// Retrieve from distributed storage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, err := s.distributedStore.RetrieveDistributed(ctx, chunk)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Retrieval failed",
			Message: err.Error(),
		})
		return
	}

	// Return binary data
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%d.bin", userAddr, chunkID))
	c.Header("Content-Length", fmt.Sprintf("%d", len(data)))

	c.Data(http.StatusOK, "application/octet-stream", data)
}
