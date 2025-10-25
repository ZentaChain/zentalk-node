package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
	"github.com/ZentaChain/zentalk-node/pkg/network"
	"github.com/ZentaChain/zentalk-node/pkg/storage"
)

const (
	defaultPort     = 8080
	defaultKeyPath  = "./keys/relay.pem"
	heartbeatInterval = 5 * time.Minute
)

var (
	port           = flag.Int("port", defaultPort, "Port to listen on")
	keyPath        = flag.String("key", defaultKeyPath, "Path to private key file")
	generateKey    = flag.Bool("genkey", false, "Generate new private key")
	operatorAddr   = flag.String("operator", "", "Operator ETH address (required)")
	contractAddr   = flag.String("contract", "", "Registry contract address (required)")
	rpcURL         = flag.String("rpc", "https://rpc.sepolia.org", "RPC URL")
	enableMesh     = flag.Bool("mesh", true, "Enable auto-mesh formation")
	targetPeers    = flag.Int("peers", 5, "Target number of relay peers for mesh")
)

func main() {
	flag.Parse()

	printBanner()

	// Validate required flags
	if *operatorAddr == "" {
		log.Fatal("Error: -operator flag is required (your ETH wallet address)")
	}

	if *contractAddr == "" {
		log.Fatal("Error: -contract flag is required (registry contract address)")
	}

	// Load or generate private key
	privateKey, err := loadOrGenerateKey(*keyPath, *generateKey)
	if err != nil {
		log.Fatalf("Failed to load/generate key: %v", err)
	}

	log.Printf("✓ Private key loaded from %s", *keyPath)

	// Create relay server
	relay := network.NewRelayServer(*port, privateKey)

	// Set callback for relay counting
	relay.OnMessageRelayed = func() {
		// TODO: Implement batch reporting to blockchain
		log.Println("Message relayed (will report to blockchain)")
	}

	// Create message queue for offline message persistence
	queuePath := fmt.Sprintf("./data/relay-%d-queue.db", *port)
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	messageQueue, err := storage.NewRelayMessageQueue(queuePath, 30*24*time.Hour) // 30 days TTL
	if err != nil {
		log.Fatalf("Failed to create message queue: %v", err)
	}
	relay.AttachMessageQueue(messageQueue)
	log.Printf("📬 Message queue initialized at %s (TTL: 30 days)", queuePath)

	// Start relay server
	if err := relay.Start(); err != nil {
		log.Fatalf("Failed to start relay server: %v", err)
	}

	log.Printf("✓ Relay server listening on port %d", *port)

	// Start auto-mesh formation if enabled
	var meshManager *network.MeshManager
	if *enableMesh {
		meshManager = network.NewMeshManager(relay, *targetPeers)
		if err := meshManager.Start(); err != nil {
			log.Fatalf("Failed to start mesh manager: %v", err)
		}
		log.Printf("✓ Auto-mesh formation enabled (target: %d peers)", *targetPeers)
	} else {
		log.Println("⚠️  Auto-mesh formation disabled")
	}

	// TODO: Register on blockchain
	log.Println("⏳ Registering on blockchain...")
	log.Printf("   Operator: %s", *operatorAddr)
	log.Printf("   Contract: %s", *contractAddr)
	log.Printf("   RPC URL: %s", *rpcURL)
	log.Println("   (Blockchain integration coming soon)")

	// Start heartbeat loop
	go startHeartbeatLoop(relay, meshManager)

	// Print status
	printStatus(relay, meshManager)

	// Wait for shutdown signal
	waitForShutdown(relay, meshManager, messageQueue)
}

func printBanner() {
	fmt.Println("╔═══════════════════════════════════════════════════╗")
	fmt.Println("║         Zentalk Mesh Relay Server v1.0           ║")
	fmt.Println("║      Privacy-preserving decentralized chat       ║")
	fmt.Println("╚═══════════════════════════════════════════════════╝")
	fmt.Println()
}

func loadOrGenerateKey(keyPath string, generate bool) (*rsa.PrivateKey, error) {
	// Check if key file exists
	if _, err := os.Stat(keyPath); err == nil && !generate {
		// Load existing key
		log.Println("Loading existing private key...")
		pemData, err := crypto.LoadKeyFromFile(keyPath)
		if err != nil {
			return nil, err
		}

		return crypto.ImportPrivateKeyPEM(pemData)
	}

	// Generate new key
	log.Println("Generating new RSA-4096 key pair...")
	privateKey, err := crypto.GenerateRSAKeyPair()
	if err != nil {
		return nil, err
	}

	// Save to file
	pemData, err := crypto.ExportPrivateKeyPEM(privateKey)
	if err != nil {
		return nil, err
	}

	// Create directory if needed
	if err := os.MkdirAll("./keys", 0700); err != nil {
		return nil, err
	}

	if err := crypto.SaveKeyToFile(keyPath, pemData); err != nil {
		return nil, err
	}

	log.Printf("✓ New key saved to %s", keyPath)

	// Also save public key
	pubPEM, err := crypto.ExportPublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	pubPath := keyPath + ".pub"
	if err := crypto.SaveKeyToFile(pubPath, pubPEM); err != nil {
		return nil, err
	}

	log.Printf("✓ Public key saved to %s", pubPath)

	return privateKey, nil
}

func startHeartbeatLoop(relay *network.RelayServer, meshManager *network.MeshManager) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		stats := relay.GetStats()

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("💓 Heartbeat")
		log.Printf("   Messages relayed: %v", stats["messages_relayed"])
		log.Printf("   Connected peers: %v", stats["connected_peers"])

		// Show mesh status if enabled
		if meshManager != nil {
			meshStatus := meshManager.GetMeshStatus()
			log.Printf("   Relay peers: %v/%v", meshStatus["relay_peers"], meshStatus["target_peers"])
			log.Printf("   Mesh healthy: %v", meshStatus["mesh_healthy"])
		}

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// TODO: Send heartbeat to blockchain
		// blockchain.SendHeartbeat()

		// TODO: Report relay count to blockchain
		// if messagesRelayed > 0 {
		//     blockchain.RecordRelays(messagesRelayed)
		// }
	}
}

func printStatus(relay *network.RelayServer, meshManager *network.MeshManager) {
	stats := relay.GetStats()

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 Relay Server Status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   Status: ✅ RUNNING\n")
	fmt.Printf("   Port: %d\n", *port)
	fmt.Printf("   Operator: %s\n", *operatorAddr)
	fmt.Printf("   Messages relayed: %v\n", stats["messages_relayed"])
	fmt.Printf("   Connected peers: %v\n", stats["connected_peers"])

	// Show mesh status if enabled
	if meshManager != nil {
		meshStatus := meshManager.GetMeshStatus()
		fmt.Printf("   Mesh auto-formation: ✅ ENABLED\n")
		fmt.Printf("   Relay peers: %v/%v\n", meshStatus["relay_peers"], meshStatus["target_peers"])
		if meshStatus["mesh_healthy"].(bool) {
			fmt.Printf("   Mesh health: ✅ HEALTHY\n")
		} else {
			fmt.Printf("   Mesh health: ⚠️  ESTABLISHING\n")
		}
	} else {
		fmt.Printf("   Mesh auto-formation: ⚠️  DISABLED\n")
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("💡 Keep this running to earn rewards!")
	fmt.Println("   Heartbeat: Every 5 minutes")
	fmt.Println("   Rewards: Automatic")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()
}

func waitForShutdown(relay *network.RelayServer, meshManager *network.MeshManager, messageQueue *storage.RelayMessageQueue) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	fmt.Println()
	log.Println("Shutting down gracefully...")

	// Stop mesh manager first
	if meshManager != nil {
		meshManager.Stop()
		log.Println("✓ Mesh manager stopped")
	}

	// Stop relay server
	if err := relay.Stop(); err != nil {
		log.Printf("Error stopping relay: %v", err)
	}

	// Close message queue database
	if messageQueue != nil {
		if err := messageQueue.Close(); err != nil {
			log.Printf("Error closing message queue: %v", err)
		} else {
			log.Println("✓ Message queue closed")
		}
	}

	log.Println("✓ Relay server stopped")
	log.Println("Goodbye! 👋")
	os.Exit(0)
}

// TODO: Implement these functions with actual blockchain integration
// func registerOnBlockchain() error
// func sendHeartbeat() error
// func recordRelays(count uint64) error
// func claimRewards() error
