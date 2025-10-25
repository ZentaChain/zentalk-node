package network

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zentalk/protocol/pkg/protocol"
)

// RelayMetadata contains information about a relay server
type RelayMetadata struct {
	Address        protocol.Address `json:"address"`         // Relay's protocol address
	NetworkAddress string           `json:"network_address"` // IP:Port for connection
	PublicKeyPEM   string           `json:"public_key"`      // RSA public key in PEM format
	Region         string           `json:"region"`          // Geographic region (e.g., "us-west", "eu-central")
	Operator       string           `json:"operator"`        // Relay operator identifier
	Version        string           `json:"version"`         // Protocol version
	MaxConnections int              `json:"max_connections"` // Maximum concurrent connections
	Uptime         uint64           `json:"uptime"`          // Uptime in seconds
	LastSeen       int64            `json:"last_seen"`       // Unix timestamp (seconds)

	// Health metrics (optional, may be empty when first published)
	Latency        int64  `json:"latency,omitempty"`        // Average latency in milliseconds
	PacketLoss     float64 `json:"packet_loss,omitempty"`   // Packet loss rate (0.0-1.0)
	Reliability    float64 `json:"reliability,omitempty"`   // Reliability score (0.0-1.0)
}

// RelayScore represents a scored relay for circuit selection
type RelayScore struct {
	Metadata *RelayMetadata
	Score    float64 // Higher is better
}

// Encode serializes relay metadata to JSON
func (r *RelayMetadata) Encode() ([]byte, error) {
	return json.Marshal(r)
}

// DecodeRelayMetadata deserializes relay metadata from JSON
func DecodeRelayMetadata(data []byte) (*RelayMetadata, error) {
	var meta RelayMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to decode relay metadata: %w", err)
	}
	return &meta, nil
}

// IsHealthy returns true if the relay appears healthy
func (r *RelayMetadata) IsHealthy(maxAge time.Duration) bool {
	// Check if last seen is recent
	age := time.Since(time.Unix(r.LastSeen, 0))
	if age > maxAge {
		return false
	}

	// Check reliability if available
	if r.Reliability > 0 && r.Reliability < 0.5 {
		return false
	}

	// Check packet loss if available
	if r.PacketLoss > 0 && r.PacketLoss > 0.3 {
		return false
	}

	return true
}

// CalculateScore calculates a quality score for relay selection
// Higher score = better relay
func (r *RelayMetadata) CalculateScore() float64 {
	score := 100.0

	// Factor 1: Recency (max 30 points)
	age := time.Since(time.Unix(r.LastSeen, 0))
	recencyScore := 30.0
	if age > 24*time.Hour {
		recencyScore = 0.0
	} else if age > 1*time.Hour {
		recencyScore = 30.0 * (1.0 - float64(age)/(24*time.Hour.Seconds()))
	}
	score += recencyScore

	// Factor 2: Reliability (max 30 points)
	if r.Reliability > 0 {
		score += r.Reliability * 30.0
	} else {
		score += 15.0 // Default neutral score if unknown
	}

	// Factor 3: Latency (max 20 points)
	if r.Latency > 0 {
		// Lower latency = higher score
		// 0-100ms = full points, 500ms+ = 0 points
		latencyScore := 20.0
		if r.Latency > 500 {
			latencyScore = 0.0
		} else if r.Latency > 100 {
			latencyScore = 20.0 * (1.0 - float64(r.Latency-100)/400.0)
		}
		score += latencyScore
	} else {
		score += 10.0 // Default neutral score
	}

	// Factor 4: Packet loss (max 10 points)
	if r.PacketLoss >= 0 {
		score += (1.0 - r.PacketLoss) * 10.0
	} else {
		score += 5.0 // Default neutral score
	}

	// Factor 5: Uptime (max 10 points)
	uptimeHours := float64(r.Uptime) / 3600.0
	if uptimeHours > 168 { // 1 week
		score += 10.0
	} else {
		score += uptimeHours / 168.0 * 10.0
	}

	return score
}

// String returns a human-readable representation
func (r *RelayMetadata) String() string {
	return fmt.Sprintf("Relay{addr=%x, net=%s, region=%s, operator=%s, uptime=%ds, score=%.1f}",
		r.Address[:8], r.NetworkAddress, r.Region, r.Operator, r.Uptime, r.CalculateScore())
}
