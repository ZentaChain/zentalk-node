// Package main provides the HTTP API server for mesh storage
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ZentaChain/zentalk-node/pkg/meshstorage"
	"github.com/ZentaChain/zentalk-node/pkg/meshstorage/api"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 9000, "DHT node port")
	apiPort := flag.Int("api-port", 8080, "HTTP API port")
	dataDir := flag.String("data", "./mesh-data", "Data directory for storage")
	bootstrap := flag.String("bootstrap", "", "Bootstrap node address")
	enableCORS := flag.Bool("cors", true, "Enable CORS headers")
	rateLimit := flag.Int("rate-limit", 100, "Rate limit (requests per minute)")
	maxUploadMB := flag.Int("max-upload", 100, "Maximum upload size in MB")

	flag.Parse()

	fmt.Println("🚀 ZenTalk Mesh Storage API Server")
	fmt.Println("===================================")
	fmt.Println()

	// Create context for node
	ctx := context.Background()

	// Create DHT node
	fmt.Printf("📡 Starting DHT node on port %d...\n", *port)
	nodeConfig := &meshstorage.NodeConfig{
		Port:    *port,
		DataDir: *dataDir,
	}

	node, err := meshstorage.NewDHTNode(ctx, nodeConfig)
	if err != nil {
		log.Fatalf("Failed to create DHT node: %v", err)
	}

	// Bootstrap if address provided
	if *bootstrap != "" {
		fmt.Printf("🔗 Connecting to bootstrap node: %s\n", *bootstrap)
		if err := node.Bootstrap([]string{*bootstrap}); err != nil {
			log.Fatalf("Failed to bootstrap: %v", err)
		}
		fmt.Println("✅ Connected to bootstrap node")
	}

	// Set up RPC handler
	rpcHandler := meshstorage.NewRPCHandler(node)
	rpcHandler.SetupStreamHandler()

	// Display node info
	fmt.Println()
	fmt.Println("Node Information:")
	fmt.Printf("  ID: %s\n", node.ID())
	fmt.Printf("  Addresses:\n")
	for _, addr := range node.Addresses() {
		fmt.Printf("    %s\n", addr)
	}
	fmt.Printf("  Storage: %s/chunks.db\n", *dataDir)
	fmt.Printf("  Peers: %d\n", node.PeerCount())
	fmt.Println()

	// Create HTTP API server
	fmt.Printf("🌐 Starting HTTP API server on port %d...\n", *apiPort)

	apiConfig := &api.Config{
		Port:            *apiPort,
		EnableCORS:      *enableCORS,
		RateLimit:       *rateLimit,
		MaxUploadSizeMB: *maxUploadMB,
	}

	apiServer, err := api.NewServer(node, apiConfig)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}

	// Create context for graceful shutdown
	apiCtx, apiCancel := context.WithCancel(context.Background())
	defer apiCancel()

	// Start API server
	go func() {
		if err := apiServer.Start(apiCtx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	fmt.Println()
	fmt.Println("✅ Server is ready!")
	fmt.Println()
	fmt.Println("API Endpoints:")
	fmt.Printf("  POST   http://localhost:%d/api/v1/storage/upload\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/storage/download/:userAddr/:chunkID\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/storage/status/:userAddr/:chunkID\n", *apiPort)
	fmt.Printf("  DELETE http://localhost:%d/api/v1/storage/delete/:userAddr/:chunkID\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/network/info\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/network/peers\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/node/info\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/api/v1/node/stats\n", *apiPort)
	fmt.Printf("  GET    http://localhost:%d/health\n", *apiPort)
	fmt.Println()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh

	fmt.Println("\n🛑 Shutting down...")

	// Graceful shutdown
	apiCancel() // Cancel context to stop API server

	// Close node
	if err := node.Close(); err != nil {
		fmt.Printf("Error closing node: %v\n", err)
	}

	fmt.Println("👋 Goodbye!")
}
