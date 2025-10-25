# ZenTalk Mesh Storage HTTP API

REST API server for the ZenTalk decentralized mesh storage network. This API allows web applications to upload, download, and manage data stored across a distributed network of nodes using erasure coding for redundancy.

## Features

- **Distributed Storage**: Data is split into 15 shards using Reed-Solomon erasure coding (10+5)
- **Fault Tolerance**: Can recover data with only 10 out of 15 shards
- **RESTful Design**: Standard HTTP methods and JSON responses
- **Rate Limiting**: Configurable request throttling per IP
- **CORS Support**: Enable cross-origin requests for web apps
- **Health Monitoring**: Real-time storage and network status
- **Base64 Encoding**: Binary-safe data transfer in JSON

## Quick Start

### Build and Run

```bash
# Build the API server
cd cmd/mesh-api
go build -o mesh-api

# Run with defaults (DHT port 9000, API port 8080)
./mesh-api

# Run with custom configuration
./mesh-api \
  --port 9100 \
  --api-port 8080 \
  --data ./my-storage \
  --bootstrap /ip4/10.0.0.1/tcp/9000/p2p/QmBootstrap... \
  --cors true \
  --rate-limit 100 \
  --max-upload 50
```

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 9000 | DHT node port |
| `--api-port` | 8080 | HTTP API port |
| `--data` | ./mesh-data | Data directory for storage |
| `--bootstrap` | "" | Bootstrap node multiaddr |
| `--cors` | true | Enable CORS headers |
| `--rate-limit` | 100 | Requests per minute per IP |
| `--max-upload` | 100 | Maximum upload size in MB |

## API Endpoints

Base URL: `http://localhost:8080`

### Storage Operations

#### Upload Data

Store data in the mesh network with erasure coding.

**Endpoint**: `POST /api/v1/storage/upload`

**Request Body**:
```json
{
  "userAddr": "0x1234567890abcdef1234567890abcdef12345678",
  "chunkID": 42,
  "data": "SGVsbG8sIFplblRhbGshIFRoaXMgaXMgYSB0ZXN0IG1lc3NhZ2Uu"
}
```

**Fields**:
- `userAddr` (string, required): Ethereum address (0x format, 42 chars)
- `chunkID` (int, required): Unique chunk identifier
- `data` (string, required): Base64-encoded binary data

**Response** (200 OK):
```json
{
  "success": true,
  "userAddr": "0x1234567890abcdef1234567890abcdef12345678",
  "chunkID": 42,
  "originalSize": 1024,
  "shardCount": 15,
  "redundancy": 1.5,
  "faultTolerance": 5,
  "shardLocations": [
    {
      "shardIndex": 0,
      "nodeId": "QmNode1...",
      "addresses": ["/ip4/10.0.0.1/tcp/9000"]
    }
  ],
  "uploadedAt": "2025-01-20T10:30:00Z"
}
```

**Example**:
```bash
# Upload text data
echo -n "Hello, ZenTalk!" | base64
# Output: SGVsbG8sIFplblRhbGsh

curl -X POST http://localhost:8080/api/v1/storage/upload \
  -H "Content-Type: application/json" \
  -d '{
    "userAddr": "0x1234567890abcdef1234567890abcdef12345678",
    "chunkID": 1,
    "data": "SGVsbG8sIFplblRhbGsh"
  }'
```

#### Download Data

Retrieve data from the mesh network.

**Endpoint**: `GET /api/v1/storage/download/:userAddr/:chunkID`

**Parameters**:
- `userAddr`: Ethereum address
- `chunkID`: Chunk identifier

**Response** (200 OK):
```json
{
  "success": true,
  "userAddr": "0x1234567890abcdef1234567890abcdef12345678",
  "chunkID": 1,
  "data": "SGVsbG8sIFplblRhbGsh",
  "sizeBytes": 14,
  "shardsUsed": 10,
  "shardsTotal": 15,
  "downloadedAt": "2025-01-20T10:31:00Z"
}
```

**Response** (404 Not Found):
```json
{
  "error": "Data not found",
  "message": "No data found for user 0x... chunk 1"
}
```

**Example**:
```bash
# Download data
curl http://localhost:8080/api/v1/storage/download/0x1234567890abcdef1234567890abcdef12345678/1

# Download and decode
curl -s http://localhost:8080/api/v1/storage/download/0x.../1 | \
  jq -r '.data' | \
  base64 -d
```

#### Check Storage Status

Check health and availability of stored data.

**Endpoint**: `GET /api/v1/storage/status/:userAddr/:chunkID`

**Response** (200 OK):
```json
{
  "success": true,
  "userAddr": "0x1234567890abcdef1234567890abcdef12345678",
  "chunkID": 1,
  "exists": true,
  "health": "excellent",
  "healthScore": 1.0,
  "availableShards": 15,
  "totalShards": 15,
  "minRequiredShards": 10,
  "shardStatus": [
    {
      "shardIndex": 0,
      "available": true,
      "nodeId": "QmNode1..."
    }
  ],
  "checkedAt": "2025-01-20T10:32:00Z"
}
```

**Health Levels**:
- `excellent`: All 15 shards available
- `good`: 13-14 shards available (some redundancy lost)
- `degraded`: 10-12 shards available (minimal redundancy)
- `critical`: 8-9 shards available (below minimum, might still recover)
- `lost`: <8 shards (cannot recover data)

**Example**:
```bash
curl http://localhost:8080/api/v1/storage/status/0x1234567890abcdef1234567890abcdef12345678/1
```

#### Delete Data

Remove data from the mesh network.

**Endpoint**: `DELETE /api/v1/storage/delete/:userAddr/:chunkID`

**Response** (200 OK):
```json
{
  "success": true,
  "message": "Chunk 1 for user 0x... deleted successfully"
}
```

**Example**:
```bash
curl -X DELETE http://localhost:8080/api/v1/storage/delete/0x1234567890abcdef1234567890abcdef12345678/1
```

### Network Information

#### Get Network Info

Get network-wide statistics.

**Endpoint**: `GET /api/v1/network/info`

**Response**:
```json
{
  "success": true,
  "networkId": "zentalk-mesh-v1",
  "nodeCount": 5,
  "totalPeers": 4,
  "upSince": "2025-01-20T10:00:00Z",
  "version": "1.0.0-beta"
}
```

#### List Connected Peers

Get information about connected peer nodes.

**Endpoint**: `GET /api/v1/network/peers`

**Response**:
```json
{
  "success": true,
  "count": 3,
  "peers": [
    {
      "peerId": "QmNode1...",
      "addresses": [
        "/ip4/10.0.0.1/tcp/9000",
        "/ip6/::1/tcp/9000"
      ],
      "connected": true,
      "lastSeen": "2025-01-20T10:30:00Z"
    }
  ]
}
```

#### Health Check

Check system health status.

**Endpoint**: `GET /health`

**Response**:
```json
{
  "success": true,
  "status": "healthy",
  "uptime": "2h 15m 30s",
  "checks": {
    "dhtReachable": true,
    "storageWritable": true,
    "peersConnected": true,
    "memoryOk": true
  }
}
```

**Status Values**:
- `healthy`: All systems operational
- `degraded`: No peers connected, but functional
- `unhealthy`: Critical systems failing

### Node Information

#### Get Node Info

Get information about this specific node.

**Endpoint**: `GET /api/v1/node/info`

**Response**:
```json
{
  "success": true,
  "nodeId": "QmThisNode...",
  "addresses": [
    "/ip4/192.168.1.100/tcp/9000",
    "/ip6/::1/tcp/9000"
  ],
  "isBootstrap": false,
  "bootstrapped": true,
  "connectedPeers": 4,
  "storagePath": "data/chunks.db",
  "startedAt": "2025-01-20T10:00:00Z"
}
```

#### Get Node Statistics

Get storage and performance statistics.

**Endpoint**: `GET /api/v1/node/stats`

**Response**:
```json
{
  "success": true,
  "stats": {
    "totalChunks": 1250,
    "totalSizeBytes": 52428800,
    "totalSizeGb": 0.05,
    "uniqueUsers": 42,
    "averageChunkSizeBytes": 41943,
    "uploadCount": 1500,
    "downloadCount": 3200,
    "successRate": 99.8
  }
}
```

## Rate Limiting

The API implements IP-based rate limiting to prevent abuse.

**Default**: 100 requests per minute per IP address

**Response** (429 Too Many Requests):
```json
{
  "error": "Rate limit exceeded",
  "message": "Too many requests. Please try again in 60 seconds."
}
```

**Customization**:
```bash
./mesh-api --rate-limit 200  # 200 requests per minute
```

## Error Handling

All errors return standard JSON responses:

```json
{
  "error": "Error type",
  "message": "Detailed error message"
}
```

**Common HTTP Status Codes**:
- `200 OK`: Successful operation
- `400 Bad Request`: Invalid input (e.g., malformed address, missing fields)
- `404 Not Found`: Data not found in network
- `413 Payload Too Large`: Upload exceeds max size limit
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server-side error

## CORS Configuration

CORS is enabled by default to allow browser-based web applications to access the API.

**Default Configuration**:
- Allows all origins (`*`)
- Allows methods: GET, POST, PUT, DELETE, OPTIONS
- Allows headers: Content-Type, Authorization
- Max age: 12 hours

**Disable CORS** (if needed):
```bash
./mesh-api --cors=false
```

## Integration with ZenTalk Web App

### JavaScript/TypeScript Example

```typescript
// api.ts
const API_BASE_URL = 'http://localhost:8080';

export async function uploadMessage(
  userAddr: string,
  chunkID: number,
  messageData: Uint8Array
): Promise<UploadResponse> {
  // Convert binary data to base64
  const base64Data = btoa(String.fromCharCode(...messageData));

  const response = await fetch(`${API_BASE_URL}/api/v1/storage/upload`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      userAddr,
      chunkID,
      data: base64Data,
    }),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message);
  }

  return response.json();
}

export async function downloadMessage(
  userAddr: string,
  chunkID: number
): Promise<Uint8Array> {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/storage/download/${userAddr}/${chunkID}`
  );

  if (!response.ok) {
    throw new Error('Message not found');
  }

  const data = await response.json();

  // Convert base64 back to binary
  const binaryString = atob(data.data);
  return Uint8Array.from(binaryString, c => c.charCodeAt(0));
}

export async function checkMessageHealth(
  userAddr: string,
  chunkID: number
): Promise<StatusResponse> {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/storage/status/${userAddr}/${chunkID}`
  );

  return response.json();
}
```

### React Hook Example

```typescript
// useMessages.ts
import { useState, useEffect } from 'react';

export function useMessageStorage(userAddr: string) {
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const uploadMessage = async (chunkID: number, data: Uint8Array) => {
    setUploading(true);
    setError(null);

    try {
      const result = await uploadMessage(userAddr, chunkID, data);
      console.log('Uploaded:', result.shardCount, 'shards');
      return result;
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setUploading(false);
    }
  };

  return { uploadMessage, uploading, error };
}
```

## Testing

### Run Integration Tests

```bash
cd pkg/meshstorage/api
go test -v
```

**Test Coverage**:
- ✅ Upload/Download cycle
- ✅ Status checking
- ✅ Network endpoints
- ✅ Rate limiting
- ✅ Input validation
- ✅ Concurrent uploads

### Manual Testing with curl

```bash
# 1. Start the server
./mesh-api --port 9000 --api-port 8080

# 2. Check health
curl http://localhost:8080/health

# 3. Upload test data
TEST_DATA=$(echo -n "Test message from curl" | base64)
curl -X POST http://localhost:8080/api/v1/storage/upload \
  -H "Content-Type: application/json" \
  -d "{\"userAddr\":\"0x1234567890abcdef1234567890abcdef12345678\",\"chunkID\":999,\"data\":\"$TEST_DATA\"}"

# 4. Check status
curl http://localhost:8080/api/v1/storage/status/0x1234567890abcdef1234567890abcdef12345678/999

# 5. Download and verify
curl -s http://localhost:8080/api/v1/storage/download/0x1234567890abcdef1234567890abcdef12345678/999 | \
  jq -r '.data' | \
  base64 -d

# 6. Get node stats
curl http://localhost:8080/api/v1/node/stats

# 7. List peers
curl http://localhost:8080/api/v1/network/peers

# 8. Delete data
curl -X DELETE http://localhost:8080/api/v1/storage/delete/0x1234567890abcdef1234567890abcdef12345678/999
```

## Performance Considerations

### Upload Performance
- **Erasure Coding**: ~50-100ms for 1MB file
- **Network Distribution**: 15 parallel uploads to peers
- **Total Upload Time**: ~500ms for 1MB (depends on network)

### Download Performance
- **Parallel Retrieval**: Downloads 10+ shards in parallel
- **Reconstruction**: ~30-50ms for 1MB file
- **Total Download Time**: ~300ms for 1MB (depends on network)

### Scalability
- **Max Upload Size**: 100MB (configurable)
- **Concurrent Requests**: Unlimited (rate-limited per IP)
- **Storage Capacity**: Limited by disk space and peer availability

## Security

### Current Implementation
- ✅ Input validation (address format, data size)
- ✅ Rate limiting per IP
- ✅ CORS configuration
- ⚠️ No authentication (add before production)

### Recommended Enhancements
1. **API Key Authentication**: Add `Authorization` header validation
2. **User Signature Verification**: Verify Ethereum signatures
3. **TLS/HTTPS**: Enable encrypted connections
4. **Request Signing**: Prevent replay attacks

### Example: Adding API Key Auth

Uncomment the auth middleware in `server.go`:

```go
// Add to setupRoutes() in server.go
if s.config.RequireAuth {
    api.Use(AuthMiddleware(s.config.APIKey))
}
```

Set API key:
```bash
./mesh-api --api-key "your-secret-key-here"
```

Use in requests:
```bash
curl -H "Authorization: Bearer your-secret-key-here" \
  http://localhost:8080/api/v1/storage/upload
```

## Monitoring

### Health Checks
Use `/health` endpoint for monitoring systems:

```bash
# Nagios/Icinga check
curl -f http://localhost:8080/health || exit 1

# Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

### Metrics to Monitor
- Peer count (should be > 0)
- Success rate (should be > 95%)
- Storage size growth
- Upload/download counts
- Response times

## Troubleshooting

### No Peers Connected

**Problem**: `"connectedPeers": 0` in node info

**Solution**:
1. Check if bootstrap node is reachable
2. Verify firewall allows DHT port
3. Use bootstrap flag: `--bootstrap /ip4/...`

### Upload Fails

**Problem**: Upload returns 500 error

**Check**:
1. Is data size within limit? (check `--max-upload`)
2. Are there enough peers? (need 15 for full redundancy)
3. Check logs for storage errors

### Download Returns 404

**Problem**: Data not found

**Check**:
1. Verify upload succeeded
2. Check shard status: `/api/v1/storage/status/:addr/:id`
3. Ensure at least 10 shards available

### Rate Limit Too Low

**Problem**: Frequent 429 errors

**Solution**:
```bash
./mesh-api --rate-limit 500  # Increase to 500 req/min
```

## Next Steps

After deploying the API:

1. **Multi-Node Testing**: Run 3+ nodes and test cross-node data retrieval
2. **Frontend Integration**: Update ZenTalk web app to use these endpoints
3. **Authentication**: Add signature verification for production
4. **Monitoring**: Set up dashboards for health/stats endpoints
5. **Load Testing**: Test with realistic user loads
6. **Blockchain Integration**: Add smart contract hooks (Phase 3)

## API Version

**Current Version**: v1
**Status**: Beta
**Network ID**: zentalk-mesh-v1

## Support

For issues or questions:
- GitHub: [zentalk/protocol](https://github.com/zentalk/protocol)
- Documentation: See main project README
- Tests: Run `go test ./...` for validation
