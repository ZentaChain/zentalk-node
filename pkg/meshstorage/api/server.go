// Package api provides HTTP REST API for mesh storage network
package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zentalk/protocol/pkg/meshstorage"
)

// Server represents the HTTP API server for mesh storage
type Server struct {
	node             *meshstorage.DHTNode
	distributedStore *meshstorage.DistributedStorage
	router           *gin.Engine
	port             int
	httpServer       *http.Server
	chunkMetadata    map[string]*meshstorage.DistributedChunk // Maps "userAddr:chunkID" to chunk metadata
	metadataMu       sync.RWMutex
	storagePath      string // Path to storage directory
	isBootstrap      bool   // Whether this node is a bootstrap node
}

// Config holds server configuration
type Config struct {
	Port            int
	EnableCORS      bool
	RateLimit       int // Requests per minute
	MaxUploadSizeMB int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	StoragePath     string // Path to storage directory (optional, defaults to node's storage path)
	IsBootstrap     bool   // Whether this node is a bootstrap node (optional, defaults to false)
}

// DefaultConfig returns default server configuration
func DefaultConfig() *Config {
	return &Config{
		Port:            8080,
		EnableCORS:      true,
		RateLimit:       100,
		MaxUploadSizeMB: 100,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
	}
}

// NewServer creates a new HTTP API server
func NewServer(node *meshstorage.DHTNode, config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create distributed storage instance
	distributedStore, err := meshstorage.NewDistributedStorage(node)
	if err != nil {
		return nil, fmt.Errorf("failed to create distributed storage: %w", err)
	}

	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Get storage path from node if not provided
	storagePath := config.StoragePath
	if storagePath == "" {
		storagePath = node.Storage().Path() // Get actual storage path from node
	}

	server := &Server{
		node:             node,
		distributedStore: distributedStore,
		router:           router,
		port:             config.Port,
		chunkMetadata:    make(map[string]*meshstorage.DistributedChunk),
		storagePath:      storagePath,
		isBootstrap:      config.IsBootstrap,
	}

	// Setup middleware
	server.setupMiddleware(config)

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware(config *Config) {
	// CORS middleware
	if config.EnableCORS {
		s.router.Use(CORSMiddleware())
	}

	// Rate limiting
	s.router.Use(RateLimitMiddleware(config.RateLimit))

	// Request logging
	s.router.Use(LoggingMiddleware())

	// Error recovery
	s.router.Use(gin.Recovery())

	// Max upload size
	s.router.MaxMultipartMemory = int64(config.MaxUploadSizeMB) << 20 // MB to bytes
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// API v1 group
	v1 := s.router.Group("/api/v1")
	{
		// Storage endpoints
		storage := v1.Group("/storage")
		{
			storage.POST("/upload", s.handleUpload)
			storage.GET("/download/:userAddr/:chunkID", s.handleDownload)
			storage.GET("/status/:userAddr/:chunkID", s.handleStatus)
			storage.DELETE("/delete/:userAddr/:chunkID", s.handleDelete)
		}

		// Network endpoints
		network := v1.Group("/network")
		{
			network.GET("/info", s.handleNetworkInfo)
			network.GET("/peers", s.handlePeers)
			network.GET("/health", s.handleHealth)
		}

		// Node endpoints
		node := v1.Group("/node")
		{
			node.GET("/info", s.handleNodeInfo)
			node.GET("/stats", s.handleNodeStats)
		}
	}

	// Health check endpoint (outside versioning)
	s.router.GET("/health", s.handleHealth)
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🌐 HTTP API server starting on port %d...\n", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ Server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	fmt.Println("\n🛑 Shutting down HTTP API server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(shutdownCtx)
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// getChunkKey generates a metadata key from userAddr and chunkID
func getChunkKey(userAddr string, chunkID int) string {
	return fmt.Sprintf("%s:%d", userAddr, chunkID)
}

// storeChunkMetadata stores distributed chunk metadata
func (s *Server) storeChunkMetadata(chunk *meshstorage.DistributedChunk) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	key := getChunkKey(chunk.UserAddr, chunk.ChunkID)
	s.chunkMetadata[key] = chunk
}

// getChunkMetadata retrieves distributed chunk metadata
func (s *Server) getChunkMetadata(userAddr string, chunkID int) (*meshstorage.DistributedChunk, bool) {
	s.metadataMu.RLock()
	defer s.metadataMu.RUnlock()
	key := getChunkKey(userAddr, chunkID)
	chunk, exists := s.chunkMetadata[key]
	return chunk, exists
}

// deleteChunkMetadata removes distributed chunk metadata
func (s *Server) deleteChunkMetadata(userAddr string, chunkID int) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	key := getChunkKey(userAddr, chunkID)
	delete(s.chunkMetadata, key)
}
