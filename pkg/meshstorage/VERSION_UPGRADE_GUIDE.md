# Version Upgrade Guide for ZenTalk Mesh Storage

## Overview

ZenTalk's mesh storage uses **semantic versioning** (major.minor.patch) for protocol compatibility. This guide explains how to safely upgrade the mesh network without breaking existing nodes.

## Current Version: 1.0.0

- **Supported Versions**: 1.0.0
- **Minimum Version**: 1.0.0
- **Features**: Erasure coding, signature auth, automatic repair, health monitoring

## Version Negotiation Strategy

### Backward Compatibility

**Key Principle**: New nodes must be able to communicate with old nodes.

**How it works**:
1. Every RPC message includes a `version` field (defaults to "1.0.0" if empty)
2. Receiving node checks if version is supported
3. If unsupported, returns error with list of supported versions
4. Client can retry with compatible version

### Example: Adding Version 1.1.0

When you want to add new features in version 1.1.0:

#### Step 1: Update version.go

```go
const (
    CurrentVersion = "1.1.0"          // Update to new version
    MinSupportedVersion = "1.0.0"     // Keep old minimum
    MaxSupportedVersion = "1.1.0"     // Update to new max
)

func getSupportedVersions() []string {
    // Add new version to list
    return []string{"1.0.0", "1.1.0"}
}

func getSupportedFeatures() []string {
    return []string{
        "erasure_coding",
        "signature_auth",
        "automatic_repair",
        "health_monitoring",
        "compression",  // NEW feature in 1.1.0
    }
}
```

#### Step 2: Handle Version-Specific Features

Add version checks in your code:

```go
func (h *RPCHandler) handleStoreShard(payload []byte, version string) RPCResponse {
    var req StoreShardRequest
    if err := json.Unmarshal(payload, &req); err != nil {
        return RPCResponse{Success: false, Error: err.Error()}
    }

    // Check if compression is supported
    if req.Compressed && CompareVersions(version, "1.1.0") < 0 {
        // Old node can't handle compression - decompress first
        req.Data = decompress(req.Data)
    }

    // Store the shard...
}
```

#### Step 3: Test Compatibility

Create tests for cross-version communication:

```go
func TestV1_1_CanTalkToV1_0(t *testing.T) {
    // Create v1.0 node (simulated)
    oldNode := createNodeWithVersion("1.0.0")

    // Create v1.1 node
    newNode := createNodeWithVersion("1.1.0")

    // v1.1 should successfully store on v1.0 node
    // (without using v1.1 features)
    err := newNode.StoreShard(oldNode, data)
    if err != nil {
        t.Fatal("v1.1 failed to communicate with v1.0")
    }
}
```

#### Step 4: Deploy Gradually

```
Week 1: Deploy v1.1.0 to 10% of nodes
Week 2: Monitor for errors, deploy to 30%
Week 3: Deploy to 60%
Week 4: Deploy to 100%
```

During this period:
- v1.1.0 nodes use new features when talking to each other
- v1.1.0 nodes fall back to v1.0.0 behavior when talking to old nodes
- No network disruption!

## Breaking Changes (Major Version)

If you need to make **incompatible changes** (e.g., changing erasure coding from 10+5 to 20+10), use a major version bump:

### Version 2.0.0

#### Step 1: Update version.go

```go
const (
    CurrentVersion = "2.0.0"
    MinSupportedVersion = "1.0.0"  // Still support old nodes temporarily
    MaxSupportedVersion = "2.0.0"
)

func getSupportedVersions() []string {
    // Support both old and new for migration period
    return []string{"1.0.0", "1.1.0", "2.0.0"}
}
```

#### Step 2: Implement Migration Path

```go
func (ds *DistributedStorage) migrateToV2() error {
    // For each chunk stored in v1 format:
    // 1. Retrieve with 10+5 erasure coding
    // 2. Re-encode with 20+10 erasure coding
    // 3. Store in new format
    // 4. Mark as migrated
}
```

#### Step 3: Announce Deprecation

```
Month 1: Announce v2.0.0 release, encourage upgrades
Month 2-3: Migration period (support both v1 and v2)
Month 4: Drop v1 support, set MinSupportedVersion = "2.0.0"
```

## Common Scenarios

### Scenario 1: Adding Optional Feature

**Example**: Add compression support

**Solution**: Minor version bump (1.0.0 → 1.1.0)

```go
// In v1.1.0
type StoreShardRequest struct {
    ShardKey   string `json:"shard_key"`
    Data       []byte `json:"data"`
    Compressed bool   `json:"compressed,omitempty"` // NEW, optional
}

// Handler checks if peer supports compression
if peerSupportsVersion("1.1.0") {
    req.Compressed = true
    req.Data = compress(data)
}
```

### Scenario 2: Changing Message Format

**Example**: Rename field from `ShardKey` to `ChunkKey`

**Solution**: Major version bump (1.x.x → 2.0.0) + migration

```go
// v2.0.0 - Support both during migration
func unmarshalRequest(data []byte, version string) (*StoreShardRequest, error) {
    if version == "1.0.0" {
        // Use old format
        var oldReq OldStoreShardRequest
        json.Unmarshal(data, &oldReq)
        return convertOldToNew(oldReq), nil
    } else {
        // Use new format
        var newReq StoreShardRequest
        json.Unmarshal(data, &newReq)
        return &newReq, nil
    }
}
```

### Scenario 3: Bug Fix

**Example**: Fix off-by-one error in shard indexing

**Solution**: Patch version bump (1.0.0 → 1.0.1)

```go
const (
    CurrentVersion = "1.0.1"  // Patch bump
    MinSupportedVersion = "1.0.0"  // Still compatible
)

// No protocol changes needed - just fix the bug
// Old and new versions can still communicate
```

## Deployment Checklist

Before deploying a new version:

- [ ] Update `CurrentVersion` in version.go
- [ ] Add new version to `getSupportedVersions()`
- [ ] Write cross-version compatibility tests
- [ ] Document breaking changes (if any)
- [ ] Test with simulated old nodes
- [ ] Deploy to staging environment first
- [ ] Monitor for version compatibility errors
- [ ] Deploy gradually (10% → 30% → 60% → 100%)
- [ ] After 3 months, consider dropping old version support

## Error Messages

If you see version compatibility errors:

```
❌ unsupported protocol version: 2.0.0 (supported: [1.0.0, 1.1.0])
```

**Solution**: The node needs to be upgraded to support v2.0.0, or the client needs to downgrade the request to v1.1.0.

## Testing Version Negotiation

Run version tests:

```bash
cd pkg/meshstorage
go test -v -run TestVersion
```

Expected output:
```
✅ TestIsVersionSupported
✅ TestNegotiateVersion
✅ TestCompareVersions
✅ TestValidateVersion
```

## Key Files

- `version.go` - Version constants and negotiation logic
- `version_test.go` - Version compatibility tests
- `rpc.go` - RPC message handling with version checks
- `VERSION_UPGRADE_GUIDE.md` - This file

## Support Matrix

| Version | Release Date | Support Status | End of Life |
|---------|--------------|----------------|-------------|
| 1.0.0   | 2024-01     | ✅ Current     | TBD         |
| 1.1.0   | TBD         | 🔮 Planned     | -           |
| 2.0.0   | TBD         | 🔮 Planned     | -           |

## Contact

For questions about version upgrades, contact the ZenTalk development team.
