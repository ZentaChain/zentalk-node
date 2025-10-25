package api

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zentalk/protocol/pkg/meshstorage"
)

// NetworkInfoResponse contains network-wide information
type NetworkInfoResponse struct {
	Success    bool      `json:"success"`
	NetworkID  string    `json:"networkId"`
	NodeCount  int       `json:"nodeCount"`
	TotalPeers int       `json:"totalPeers"`
	UpSince    time.Time `json:"upSince"`
	Version    string    `json:"version"`
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	PeerID     string    `json:"peerId"`
	Addresses  []string  `json:"addresses"`
	Connected  bool      `json:"connected"`
	LastSeen   time.Time `json:"lastSeen,omitempty"`
	ChunkCount int       `json:"chunkCount,omitempty"`
}

// PeersResponse contains list of connected peers
type PeersResponse struct {
	Success bool       `json:"success"`
	Count   int        `json:"count"`
	Peers   []PeerInfo `json:"peers"`
}

// HealthResponse contains system health information
type HealthResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime  string `json:"uptime"`
	Checks  struct {
		DHTReachable     bool `json:"dhtReachable"`
		StorageWritable  bool `json:"storageWritable"`
		PeersConnected   bool `json:"peersConnected"`
		MemoryOK         bool `json:"memoryOk"`
	} `json:"checks"`
}

// NodeInfoResponse contains information about this node
type NodeInfoResponse struct {
	Success       bool     `json:"success"`
	NodeID        string   `json:"nodeId"`
	Addresses     []string `json:"addresses"`
	IsBootstrap   bool     `json:"isBootstrap"`
	Bootstrapped  bool     `json:"bootstrapped"`
	ConnectedAt   int      `json:"connectedPeers"`
	StoragePath   string   `json:"storagePath"`
	StartedAt     time.Time `json:"startedAt"`
}

// NodeStatsResponse contains statistics about this node
type NodeStatsResponse struct {
	Success bool `json:"success"`
	Stats   struct {
		TotalChunks       int     `json:"totalChunks"`
		TotalSizeBytes    int64   `json:"totalSizeBytes"`
		TotalSizeGB       float64 `json:"totalSizeGb"`
		UniqueUsers       int     `json:"uniqueUsers"`
		AverageChunkSize  int     `json:"averageChunkSizeBytes"`
		UploadCount       int64   `json:"uploadCount"`
		DownloadCount     int64   `json:"downloadCount"`
		SuccessRate       float64 `json:"successRate"`
	} `json:"stats"`
}

var (
	nodeStartTime = time.Now()
	uploadCounter int64
	downloadCounter int64
	successCounter int64
	failureCounter int64
)

// handleNetworkInfo handles GET /api/v1/network/info
func (s *Server) handleNetworkInfo(c *gin.Context) {
	peers := s.node.GetPeers()

	response := NetworkInfoResponse{
		Success:    true,
		NetworkID:  "zentalk-mesh-v1",
		NodeCount:  1 + len(peers), // This node + connected peers
		TotalPeers: len(peers),
		UpSince:    nodeStartTime,
		Version:    meshstorage.CurrentVersion,
	}

	c.JSON(http.StatusOK, response)
}

// handlePeers handles GET /api/v1/network/peers
func (s *Server) handlePeers(c *gin.Context) {
	peers := s.node.GetPeers()

	peerList := make([]PeerInfo, 0, len(peers))

	for peerID, peerInfo := range peers {
		addrs := make([]string, len(peerInfo.Addresses))
		for i, addr := range peerInfo.Addresses {
			addrs[i] = addr.String()
		}

		peerList = append(peerList, PeerInfo{
			PeerID:    peerID.String(),
			Addresses: addrs,
			Connected: peerInfo.Active,
			LastSeen:  peerInfo.LastSeen,
		})
	}

	response := PeersResponse{
		Success: true,
		Count:   len(peerList),
		Peers:   peerList,
	}

	c.JSON(http.StatusOK, response)
}

// handleHealth handles GET /api/v1/network/health and GET /health
func (s *Server) handleHealth(c *gin.Context) {
	// Perform health checks

	// 1. Check DHT reachability (check if node is bootstrapped and has routing table entries)
	dhtReachable := s.node.IsBootstrapped() && len(s.node.GetPeers()) > 0

	// 2. Check storage writability with a test write
	storageWritable := true
	testData := []byte("health-check-test")
	testAddr := "0x0000000000000000000000000000000000000000"
	testChunkID := -1 // Special negative ID for health checks
	if err := s.node.Storage().StoreChunk(testAddr, testChunkID, testData); err != nil {
		storageWritable = false
	} else {
		// Clean up test data
		s.node.Storage().DeleteChunk(testAddr, testChunkID)
	}

	// 3. Check peer connectivity
	peersConnected := len(s.node.GetPeers()) > 0

	// 4. Check memory usage (warn if > 1GB allocated)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryOK := m.Alloc < 1024*1024*1024 // Less than 1GB

	checks := struct {
		DHTReachable     bool `json:"dhtReachable"`
		StorageWritable  bool `json:"storageWritable"`
		PeersConnected   bool `json:"peersConnected"`
		MemoryOK         bool `json:"memoryOk"`
	}{
		DHTReachable:    dhtReachable,
		StorageWritable: storageWritable,
		PeersConnected:  peersConnected,
		MemoryOK:        memoryOK,
	}

	// Determine overall status
	status := "healthy"
	if !checks.PeersConnected {
		status = "degraded" // No peers, but still functional
	}
	if !checks.DHTReachable || !checks.StorageWritable {
		status = "unhealthy"
	}

	uptime := time.Since(nodeStartTime)

	response := HealthResponse{
		Success: true,
		Status:  status,
		Uptime:  formatDuration(uptime),
		Checks:  checks,
	}

	c.JSON(http.StatusOK, response)
}

// handleNodeInfo handles GET /api/v1/node/info
func (s *Server) handleNodeInfo(c *gin.Context) {
	addrs := s.node.Addresses()
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}

	response := NodeInfoResponse{
		Success:       true,
		NodeID:        s.node.ID().String(),
		Addresses:     addrStrs,
		IsBootstrap:   s.isBootstrap,
		Bootstrapped:  s.node.IsBootstrapped(),
		ConnectedAt:   len(s.node.GetPeers()),
		StoragePath:   s.storagePath,
		StartedAt:     nodeStartTime,
	}

	c.JSON(http.StatusOK, response)
}

// handleNodeStats handles GET /api/v1/node/stats
func (s *Server) handleNodeStats(c *gin.Context) {
	// Get storage statistics
	stats, err := s.node.Storage().GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get stats",
			Message: err.Error(),
		})
		return
	}

	// Calculate success rate
	totalRequests := successCounter + failureCounter
	var successRate float64
	if totalRequests > 0 {
		successRate = float64(successCounter) / float64(totalRequests) * 100
	} else {
		successRate = 100.0 // No requests yet, assume 100%
	}

	// Calculate average chunk size
	var avgChunkSize int
	if stats.TotalChunks > 0 {
		avgChunkSize = int(stats.TotalSize / int64(stats.TotalChunks))
	}

	response := NodeStatsResponse{
		Success: true,
	}

	response.Stats.TotalChunks = stats.TotalChunks
	response.Stats.TotalSizeBytes = stats.TotalSize
	response.Stats.TotalSizeGB = float64(stats.TotalSize) / (1024 * 1024 * 1024)
	response.Stats.UniqueUsers = stats.TotalUsers
	response.Stats.AverageChunkSize = avgChunkSize
	response.Stats.UploadCount = uploadCounter
	response.Stats.DownloadCount = downloadCounter
	response.Stats.SuccessRate = successRate

	c.JSON(http.StatusOK, response)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
