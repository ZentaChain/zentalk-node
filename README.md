# Zentalk Node

> Join the decentralized Zentalk network and earn $CHAIN rewards.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## ⚠️ UNDER DEVELOPMENT - NOT FOR PRODUCTION

**This project is currently under active development and is NOT ready for production use.**

- ❌ Security audits not completed
- ❌ Reward system not yet implemented
- ❌ Network stability not guaranteed
- ❌ May contain bugs and vulnerabilities

**Use at your own risk. Do not run nodes with sensitive data or in production environments.**

---

## Overview

Zentalk-Node is a command-line tool that allows you to run relay and mesh storage nodes for the Zentalk decentralized messaging network. By running a node, you contribute to the network's infrastructure and earn CHAIN token rewards.

## Features

- **Relay Server** - Route encrypted messages between users via onion routing
- **Mesh Storage** - Store encrypted message attachments with erasure coding
- **DHT Integration** - Automatic peer and relay discovery
- **Offline Message Queue** - Queue messages for offline users
- **CHAIN Rewards** - Earn rewards for network contribution
- **Privacy-First** - Cannot decrypt messages (zero-knowledge routing)
- **Distributed Architecture** - No central point of failure

## Installation

### Prerequisites

- Go 1.21 or higher
- SQLite3
- Stable internet connection
- Open port for relay connections

### Build from Source

```bash
# Clone repository
git clone https://github.com/ZentaChain/zentalk-node.git
cd zentalk-node

# Build relay server
go build -o relay cmd/relay/main.go

# Build mesh storage server
go build -o mesh-api cmd/mesh-api/main.go
```

## Quick Start

### 1. Start Relay Server

```bash
./relay --port 9001
```

This starts a relay server on port 9001 that will:
- Accept encrypted message routing requests
- Connect to other relays in the network
- Publish itself to the DHT for discovery
- Queue offline messages

### 2. Start Mesh Storage Node

```bash
./mesh-api --port 8081
```

This starts a mesh storage node that will:
- Store encrypted file chunks
- Serve files to authorized users
- Participate in distributed storage network
- Use erasure coding for redundancy

### 3. Run Both Together

Use the convenience script:

```bash
./clean-restart.sh
```

This will start both relay and mesh storage servers with default configuration.

## Configuration

### Relay Server Options

```bash
./relay \
  --port 9001 \
  --dht-port 9002 \
  --region "us-west" \
  --operator "your-name"
```

### Mesh Storage Options

```bash
./mesh-api \
  --port 8081 \
  --storage-path ./mesh-data \
  --max-size 10GB
```

### Environment Variables

- `RELAY_PORT` - Relay server port (default: 9001)
- `MESH_PORT` - Mesh storage port (default: 8081)
- `DHT_PORT` - DHT port (default: 9002)
- `DATA_DIR` - Data directory for databases
- `STORAGE_DIR` - Directory for mesh storage

## Network Participation

### How It Works

1. **Message Relay**: Your relay server routes encrypted messages through multi-hop onion routing. You earn rewards for each message relayed.

2. **File Storage**: Your mesh node stores encrypted file chunks. You earn rewards based on storage provided and files served.

3. **Network Discovery**: Your node publishes itself to the DHT, making it discoverable by clients and other nodes.

4. **Offline Queue**: Messages for offline users are queued and delivered when they come online.

### Earning Rewards

Rewards are distributed based on:
- **Uptime**: How long your node stays online
- **Messages Relayed**: Number of messages routed
- **Storage Provided**: Amount of data stored
- **Network Quality**: Reliability and response time

## Node Management Scripts

### Status Check

```bash
./status.sh
```

Shows status of all running services, including:
- Relay server status and stats
- Mesh storage usage
- DHT connections
- Message queue size

### Stop All Services

```bash
./stop-all.sh
```

Gracefully stops all running node services.

### View Logs

```bash
./view-logs.sh
```

Displays logs from relay and mesh storage servers.

### Clean Restart

```bash
./clean-restart.sh
```

Stops all services, cleans temporary data, and restarts.

## Testing

### Test Message Relay

```bash
./test_messaging.sh
```

Runs end-to-end message relay test.

### Test Multi-Relay Routing

```bash
./test_multi_relay.sh
```

Tests multi-hop onion routing through multiple relays.

### Verify Encryption

```bash
./verify_relay_cannot_decrypt.sh
```

Verifies that relay servers cannot decrypt message content.

## Security

### Privacy Guarantees

- **Zero Knowledge**: Relays cannot decrypt message content
- **Onion Routing**: Multi-layer encryption hides sender/receiver
- **Metadata Protection**: Relay cannot link sender to receiver
- **Encrypted Storage**: All file chunks encrypted before storage

### Node Security

- Keep your node software updated
- Use firewall rules to restrict access
- Monitor logs for suspicious activity
- Backup your data directory regularly

## Monitoring

Your node automatically publishes statistics to the network:

- Uptime
- Messages relayed
- Storage provided
- Network latency
- Reliability score

These stats determine your reward allocation.

## Troubleshooting

### Port Already in Use

```bash
# Check what's using the port
lsof -i :9001

# Kill the process
kill -9 <PID>
```

### DHT Not Connecting

- Check firewall settings
- Ensure DHT port is accessible
- Verify internet connection
- Try different bootstrap nodes

### Low Rewards

- Increase uptime (run 24/7)
- Ensure stable internet connection
- Open ports for better connectivity
- Increase storage capacity

## Roadmap

- [ ] Auto-updates for node software
- [ ] Web dashboard for node monitoring
- [ ] Docker deployment support
- [ ] Automated reward distribution
- [ ] Node reputation system

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md)

## License

MIT License - see LICENSE file for details

## Links

- [Zentalk API](https://github.com/ZentaChain/zentalk-api) - Client API server
- [Zentalk Protocol](https://github.com/ZentaChain/zentalk-protocol) - Protocol specification
- [Website](https://zentachain.io)
- [Block Explorer](https://explorer.zentachain.io)

---

**Start earning CHAIN rewards today by running a ZenTalk node!** 🚀
