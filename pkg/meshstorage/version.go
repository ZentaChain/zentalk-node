// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"fmt"
	"strings"
)

// Protocol version constants
const (
	// CurrentVersion is the current protocol version
	CurrentVersion = "1.0.0"

	// MinSupportedVersion is the minimum version we can communicate with
	// This allows newer nodes to talk to older nodes (backward compatibility)
	MinSupportedVersion = "1.0.0"

	// MaxSupportedVersion is the maximum version we can communicate with
	// This prevents newer nodes from using features we don't understand
	MaxSupportedVersion = "1.0.0"
)

// VersionInfo contains information about protocol version and capabilities
type VersionInfo struct {
	// Current version of this node
	Version string `json:"version"`

	// List of all versions this node can communicate with
	SupportedVersions []string `json:"supported_versions"`

	// Optional features this node supports
	Features []string `json:"features,omitempty"`
}

// GetVersionInfo returns version information for this node
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:           CurrentVersion,
		SupportedVersions: getSupportedVersions(),
		Features:          getSupportedFeatures(),
	}
}

// getSupportedVersions returns all protocol versions this node supports
func getSupportedVersions() []string {
	// For now, only support 1.0.0
	// When adding 1.1.0, this would become: []string{"1.0.0", "1.1.0"}
	return []string{"1.0.0"}
}

// getSupportedFeatures returns optional features this node supports
func getSupportedFeatures() []string {
	return []string{
		"erasure_coding",      // Reed-Solomon 10+5
		"signature_auth",      // Cryptographic signatures for deletion
		"automatic_repair",    // Automatic shard repair
		"health_monitoring",   // Background health checks
	}
}

// IsVersionSupported checks if a given version is supported by this node
func IsVersionSupported(version string) bool {
	if version == "" {
		// Empty version defaults to 1.0.0 for backward compatibility
		version = "1.0.0"
	}

	supported := getSupportedVersions()
	for _, v := range supported {
		if v == version {
			return true
		}
	}
	return false
}

// NegotiateVersion finds the best compatible version between two nodes
// Returns the highest version both nodes support, or error if incompatible
func NegotiateVersion(myVersions, theirVersions []string) (string, error) {
	if len(myVersions) == 0 || len(theirVersions) == 0 {
		return "", fmt.Errorf("version list cannot be empty")
	}

	// Try to find highest common version
	// Check in descending order: 2.0.0, 1.1.0, 1.0.0, etc
	versionPriority := []string{
		"2.0.0", // Future versions
		"1.1.0",
		"1.0.0",
	}

	for _, version := range versionPriority {
		if contains(myVersions, version) && contains(theirVersions, version) {
			return version, nil
		}
	}

	return "", fmt.Errorf("no compatible version found: my versions=%v, their versions=%v",
		myVersions, theirVersions)
}

// CompareVersions compares two semantic versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	// Simple semantic version comparison
	// Format: major.minor.patch
	v1parts := strings.Split(v1, ".")
	v2parts := strings.Split(v2, ".")

	for i := 0; i < 3; i++ {
		var n1, n2 int
		if i < len(v1parts) {
			fmt.Sscanf(v1parts[i], "%d", &n1)
		}
		if i < len(v2parts) {
			fmt.Sscanf(v2parts[i], "%d", &n2)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}

// IsBackwardCompatible checks if version v2 is backward compatible with v1
// Returns true if a node running v2 can communicate with a node running v1
func IsBackwardCompatible(v1, v2 string) bool {
	// Same version is always compatible
	if v1 == v2 {
		return true
	}

	// Check if v2 >= MinSupportedVersion
	return CompareVersions(v2, MinSupportedVersion) >= 0
}

// ValidateVersion checks if a version string is valid
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Check format: major.minor.patch
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid version format: expected major.minor.patch, got %s", version)
	}

	// Check each part is numeric
	for i, part := range parts {
		var num int
		if _, err := fmt.Sscanf(part, "%d", &num); err != nil {
			return fmt.Errorf("invalid version part %d: %s is not numeric", i, part)
		}
		if num < 0 {
			return fmt.Errorf("invalid version part %d: %d cannot be negative", i, num)
		}
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// VersionCompatibilityError represents a version incompatibility error
type VersionCompatibilityError struct {
	MyVersion    string
	TheirVersion string
	Message      string
}

func (e *VersionCompatibilityError) Error() string {
	return fmt.Sprintf("version incompatibility: my version=%s, their version=%s: %s",
		e.MyVersion, e.TheirVersion, e.Message)
}

// NewVersionCompatibilityError creates a new version compatibility error
func NewVersionCompatibilityError(myVersion, theirVersion, message string) error {
	return &VersionCompatibilityError{
		MyVersion:    myVersion,
		TheirVersion: theirVersion,
		Message:      message,
	}
}
