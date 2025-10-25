package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ZentaChain/zentalk-node/pkg/meshstorage"
)

// UploadRequest represents a storage upload request
type UploadRequest struct {
	UserAddr  string `json:"userAddr" binding:"required"`  // Ethereum address
	ChunkID   int    `json:"chunkID" binding:"required"`   // Chunk identifier
	Data      string `json:"data" binding:"required"`      // Base64 encoded data
	Signature string `json:"signature"`                    // Optional: wallet signature for encryption
	Password  string `json:"password"`                     // Optional: password for encryption
	Encrypted bool   `json:"encrypted"`                    // Whether data is already client-encrypted
}

// UploadResponse represents a successful upload response
type UploadResponse struct {
	Success        bool              `json:"success"`
	UserAddr       string            `json:"userAddr"`
	ChunkID        int               `json:"chunkID"`
	OriginalSize   int               `json:"originalSizeBytes"`
	EncryptedSize  int               `json:"encryptedSizeBytes"`
	ShardCount     int               `json:"shardCount"`
	ShardSize      int               `json:"shardSizeBytes"`
	StorageNodes   []string          `json:"storageNodes"`
	Redundancy     float64           `json:"redundancy"`
	FaultTolerance int               `json:"faultTolerance"`
	Encrypted      bool              `json:"encrypted"`
	EncryptionInfo string            `json:"encryptionInfo"`
	UploadedAt     time.Time         `json:"uploadedAt"`
	ShardLocations []ShardLocationInfo `json:"shardLocations"`
}

// ShardLocationInfo contains info about where a shard is stored
type ShardLocationInfo struct {
	ShardIndex int      `json:"shardIndex"`
	NodeID     string   `json:"nodeId"`
	Addresses  []string `json:"addresses"`
}

// handleUpload handles POST /api/v1/storage/upload
func (s *Server) handleUpload(c *gin.Context) {
	var req UploadRequest

	// Parse JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Validate user address (basic Ethereum address format)
	if len(req.UserAddr) != 42 || req.UserAddr[:2] != "0x" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user address",
			Message: "User address must be a valid Ethereum address (0x...)",
		})
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid data encoding",
			Message: "Data must be base64 encoded",
		})
		return
	}

	// Validate data size (max 100MB)
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Empty data",
			Message: "Cannot upload empty data",
		})
		return
	}

	maxSize := 100 * 1024 * 1024 // 100 MB
	if len(data) > maxSize {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Data too large",
			Message: fmt.Sprintf("Maximum upload size is %d MB", maxSize/(1024*1024)),
		})
		return
	}

	// Encryption: Encrypt data before storage if not already encrypted
	var dataToStore []byte
	var encryptionInfo string
	var isEncrypted bool
	originalSize := len(data)

	if !req.Encrypted {
		// Data needs to be encrypted on server-side
		var encryptionKey *meshstorage.EncryptionKey
		var err error

		if req.Signature != "" {
			// Derive key from wallet signature (most secure)
			encryptionKey, err = meshstorage.DeriveKeyFromSignature(req.Signature)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "Invalid signature",
					Message: fmt.Sprintf("Failed to derive key from signature: %v", err),
				})
				return
			}
			encryptionInfo = "AES-256-GCM (signature-derived)"
		} else if req.Password != "" {
			// Use password-based encryption
			encrypted, err := meshstorage.EncryptWithPassword(data, req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Encryption failed",
					Message: err.Error(),
				})
				return
			}

			// Convert encrypted data to JSON for storage
			encryptedJSON, err := json.Marshal(encrypted)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Serialization failed",
					Message: err.Error(),
				})
				return
			}

			dataToStore = encryptedJSON
			encryptionInfo = "AES-256-GCM (password-based)"
			isEncrypted = true
		} else {
			// Default: Use wallet address for key derivation
			encryptionKey, err = meshstorage.DeriveKeyFromWalletAddress(req.UserAddr)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Key derivation failed",
					Message: err.Error(),
				})
				return
			}
			encryptionInfo = "AES-256-GCM (wallet-derived)"
		}

		// Encrypt with derived key (if not password-encrypted)
		if encryptionKey != nil {
			encrypted, err := meshstorage.Encrypt(data, encryptionKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Encryption failed",
					Message: err.Error(),
				})
				return
			}

			// Convert encrypted data to JSON for storage
			encryptedJSON, err := json.Marshal(encrypted)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Serialization failed",
					Message: err.Error(),
				})
				return
			}

			dataToStore = encryptedJSON
			isEncrypted = true
		}

		fmt.Printf("🔒 Encrypting data: %d bytes → %d bytes (%s)\n",
			originalSize, len(dataToStore), encryptionInfo)
	} else {
		// Data is already encrypted by client
		dataToStore = data
		encryptionInfo = "Client-side encrypted"
		isEncrypted = true
		fmt.Printf("🔒 Storing client-encrypted data: %d bytes\n", len(dataToStore))
	}

	// Log upload
	fmt.Printf("📤 Upload request: user=%s chunk=%d size=%d bytes (encrypted: %v)\n",
		req.UserAddr, req.ChunkID, len(dataToStore), isEncrypted)

	// Store encrypted data in distributed storage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()

	distributedChunk, err := s.distributedStore.StoreDistributed(
		ctx,
		req.UserAddr,
		req.ChunkID,
		dataToStore,
	)

	if err != nil {
		fmt.Printf("❌ Upload failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Storage failed",
			Message: err.Error(),
		})
		return
	}

	uploadDuration := time.Since(startTime)

	// Store chunk metadata for later retrieval
	s.storeChunkMetadata(distributedChunk)

	// Build shard location info
	shardLocations := make([]ShardLocationInfo, len(distributedChunk.ShardLocations))
	nodeIDs := make([]string, 0)
	nodeIDMap := make(map[string]bool)

	for i, loc := range distributedChunk.ShardLocations {
		nodeIDStr := loc.PeerID.String()
		shardLocations[i] = ShardLocationInfo{
			ShardIndex: loc.ShardIndex,
			NodeID:     nodeIDStr,
			Addresses:  loc.PeerAddrs,
		}

		// Collect unique node IDs
		if !nodeIDMap[nodeIDStr] {
			nodeIDs = append(nodeIDs, nodeIDStr)
			nodeIDMap[nodeIDStr] = true
		}
	}

	// Calculate redundancy and fault tolerance using constants
	redundancy := meshstorage.CalculateRedundancy()
	faultTolerance := meshstorage.CalculateFaultTolerance()

	response := UploadResponse{
		Success:        true,
		UserAddr:       req.UserAddr,
		ChunkID:        req.ChunkID,
		OriginalSize:   originalSize,
		EncryptedSize:  len(dataToStore),
		ShardCount:     len(distributedChunk.ShardLocations),
		ShardSize:      distributedChunk.ShardSize,
		StorageNodes:   nodeIDs,
		Redundancy:     redundancy,
		FaultTolerance: faultTolerance,
		Encrypted:      isEncrypted,
		EncryptionInfo: encryptionInfo,
		UploadedAt:     time.Now(),
		ShardLocations: shardLocations,
	}

	fmt.Printf("✅ Upload successful: %d bytes (encrypted: %d bytes) → %d shards across %d nodes (%.2fs)\n",
		originalSize,
		len(dataToStore),
		len(shardLocations),
		len(nodeIDs),
		uploadDuration.Seconds(),
	)

	c.JSON(http.StatusOK, response)
}

// handleUploadMultipart handles multipart file uploads
// Alternative endpoint for binary file uploads
func (s *Server) handleUploadMultipart(c *gin.Context) {
	// Get form data
	userAddr := c.PostForm("userAddr")
	chunkIDStr := c.PostForm("chunkID")

	if userAddr == "" || chunkIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing parameters",
			Message: "userAddr and chunkID are required",
		})
		return
	}

	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid chunkID",
			Message: "chunkID must be a number",
		})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "No file uploaded",
			Message: err.Error(),
		})
		return
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to read file",
			Message: err.Error(),
		})
		return
	}
	defer src.Close()

	// Read file data
	data, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to read file data",
			Message: err.Error(),
		})
		return
	}

	// Validate size
	maxSize := 100 * 1024 * 1024 // 100 MB
	if len(data) > maxSize {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "File too large",
			Message: fmt.Sprintf("Maximum file size is %d MB", maxSize/(1024*1024)),
		})
		return
	}

	// Validate Ethereum address format
	if len(userAddr) != 42 || userAddr[:2] != "0x" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user address",
			Message: "User address must be a valid Ethereum address (0x...)",
		})
		return
	}

	originalSize := len(data)

	fmt.Printf("📤 Upload (multipart): user=%s chunk=%d file=%s size=%d bytes\n",
		userAddr, chunkID, file.Filename, len(data))

	// SECURITY: Encrypt data before storage (use wallet-derived key as default)
	// This prevents node operators from reading user data
	encryptionKey, err := meshstorage.DeriveKeyFromWalletAddress(userAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Key derivation failed",
			Message: err.Error(),
		})
		return
	}

	encrypted, err := meshstorage.Encrypt(data, encryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Encryption failed",
			Message: err.Error(),
		})
		return
	}

	// Convert encrypted data to JSON for storage
	encryptedJSON, err := json.Marshal(encrypted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Serialization failed",
			Message: err.Error(),
		})
		return
	}

	fmt.Printf("🔒 Encrypting multipart upload: %d bytes → %d bytes (AES-256-GCM, wallet-derived)\n",
		originalSize, len(encryptedJSON))

	// Store encrypted data using distributed storage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	distributedChunk, err := s.distributedStore.StoreDistributed(
		ctx,
		userAddr,
		chunkID,
		encryptedJSON,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Storage failed",
			Message: err.Error(),
		})
		return
	}

	// Store chunk metadata for later retrieval
	s.storeChunkMetadata(distributedChunk)

	// Build response (similar to JSON upload)
	shardLocations := make([]ShardLocationInfo, len(distributedChunk.ShardLocations))
	for i, loc := range distributedChunk.ShardLocations {
		shardLocations[i] = ShardLocationInfo{
			ShardIndex: loc.ShardIndex,
			NodeID:     loc.PeerID.String(),
			Addresses:  loc.PeerAddrs,
		}
	}

	response := UploadResponse{
		Success:        true,
		UserAddr:       userAddr,
		ChunkID:        chunkID,
		OriginalSize:   originalSize,
		EncryptedSize:  len(encryptedJSON),
		ShardCount:     len(distributedChunk.ShardLocations),
		ShardSize:      distributedChunk.ShardSize,
		Redundancy:     meshstorage.CalculateRedundancy(),
		FaultTolerance: meshstorage.CalculateFaultTolerance(),
		Encrypted:      true,
		EncryptionInfo: "AES-256-GCM (wallet-derived)",
		UploadedAt:     time.Now(),
		ShardLocations: shardLocations,
	}

	c.JSON(http.StatusOK, response)
}
