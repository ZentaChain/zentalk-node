package meshstorage

import (
	"strings"
	"testing"
)

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{"current version", "1.0.0", true},
		{"empty version (backward compat)", "", true},
		{"future version", "2.0.0", false},
		{"unsupported version", "0.9.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionSupported(tt.version)
			if result != tt.expected {
				t.Errorf("IsVersionSupported(%s) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

func TestNegotiateVersion(t *testing.T) {
	tests := []struct {
		name        string
		myVersions  []string
		theirVersions []string
		expected    string
		shouldError bool
	}{
		{
			name:        "same version",
			myVersions:  []string{"1.0.0"},
			theirVersions: []string{"1.0.0"},
			expected:    "1.0.0",
			shouldError: false,
		},
		{
			name:        "multiple compatible versions",
			myVersions:  []string{"1.0.0", "1.1.0"},
			theirVersions: []string{"1.0.0", "1.1.0"},
			expected:    "1.1.0", // Should pick highest
			shouldError: false,
		},
		{
			name:        "backward compatibility",
			myVersions:  []string{"1.0.0", "1.1.0"},
			theirVersions: []string{"1.0.0"},
			expected:    "1.0.0",
			shouldError: false,
		},
		{
			name:        "no compatible version",
			myVersions:  []string{"2.0.0"},
			theirVersions: []string{"1.0.0"},
			expected:    "",
			shouldError: true,
		},
		{
			name:        "empty version list",
			myVersions:  []string{},
			theirVersions: []string{"1.0.0"},
			expected:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NegotiateVersion(tt.myVersions, tt.theirVersions)

			if tt.shouldError {
				if err == nil {
					t.Errorf("NegotiateVersion() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("NegotiateVersion() unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("NegotiateVersion() = %s, want %s", result, tt.expected)
				}
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"v1 less than v2 (major)", "1.0.0", "2.0.0", -1},
		{"v1 greater than v2 (major)", "2.0.0", "1.0.0", 1},
		{"v1 less than v2 (minor)", "1.0.0", "1.1.0", -1},
		{"v1 greater than v2 (minor)", "1.1.0", "1.0.0", 1},
		{"v1 less than v2 (patch)", "1.0.0", "1.0.1", -1},
		{"v1 greater than v2 (patch)", "1.0.1", "1.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("CompareVersions(%s, %s) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestIsBackwardCompatible(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"same version", "1.0.0", "1.0.0", true},
		{"v2 newer", "1.0.0", "1.1.0", true},
		{"v2 below minimum", "2.0.0", "0.9.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBackwardCompatible(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("IsBackwardCompatible(%s, %s) = %v, want %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		shouldError bool
	}{
		{"valid version", "1.0.0", false},
		{"valid version 2", "2.1.5", false},
		{"empty version", "", true},
		{"invalid format (missing patch)", "1.0", true},
		{"invalid format (too many parts)", "1.0.0.0", true},
		{"invalid format (non-numeric)", "1.a.0", true},
		{"invalid format (negative)", "1.-1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.version)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateVersion(%s) expected error but got none", tt.version)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateVersion(%s) unexpected error: %v", tt.version, err)
				}
			}
		})
	}
}

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()

	if info.Version != CurrentVersion {
		t.Errorf("GetVersionInfo().Version = %s, want %s", info.Version, CurrentVersion)
	}

	if len(info.SupportedVersions) == 0 {
		t.Error("GetVersionInfo().SupportedVersions is empty")
	}

	// Current version should be in supported versions
	found := false
	for _, v := range info.SupportedVersions {
		if v == CurrentVersion {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Current version %s not in supported versions: %v", CurrentVersion, info.SupportedVersions)
	}

	// Should have some features
	if len(info.Features) == 0 {
		t.Error("GetVersionInfo().Features is empty")
	}
}

func TestVersionCompatibilityError(t *testing.T) {
	err := NewVersionCompatibilityError("1.0.0", "2.0.0", "incompatible")

	if err == nil {
		t.Fatal("NewVersionCompatibilityError returned nil")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}

	// Check if error message contains key information
	if !strings.Contains(errMsg, "1.0.0") {
		t.Errorf("Error message doesn't contain my version: %s", errMsg)
	}
	if !strings.Contains(errMsg, "2.0.0") {
		t.Errorf("Error message doesn't contain their version: %s", errMsg)
	}
}
