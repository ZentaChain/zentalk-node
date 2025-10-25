#!/bin/bash

# ZenTalk Clean Restart Script
# Cleans all databases, stops all processes, and restarts everything fresh

set -e  # Exit on error

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Configuration
RELAY_PORT=9001
API_PORT=3001
MESH_API_PORT=8080

# Default addresses for relay (can be overridden)
DEFAULT_OPERATOR="0xfe9472d0f49b424042fd66403840b5144f9f9999"
DEFAULT_CONTRACT="0xAbCdEf1234567890AbCdEf1234567890AbCdEf12"
OPERATOR_WALLET="${1:-$DEFAULT_OPERATOR}"
CONTRACT_ADDR="${2:-$DEFAULT_CONTRACT}"

# Parse command line arguments
if [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
  echo "Usage: ./clean-restart.sh [OPERATOR_WALLET] [CONTRACT_ADDRESS]"
  echo ""
  echo "Arguments:"
  echo "  OPERATOR_WALLET   ETH wallet address for relay operator (default: $DEFAULT_OPERATOR)"
  echo "  CONTRACT_ADDRESS  Registry contract address (default: $DEFAULT_CONTRACT)"
  echo ""
  echo "Examples:"
  echo "  ./clean-restart.sh"
  echo "  ./clean-restart.sh 0x1234567890abcdef1234567890abcdef12345678"
  echo "  ./clean-restart.sh 0x1234567890abcdef1234567890abcdef12345678 0xabcd..."
  exit 0
fi

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  ZenTalk Clean Restart Script${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Stop all running processes
echo -e "${YELLOW}[1/6]${NC} Stopping all ZenTalk processes..."

# Kill relay server
RELAY_PID=$(lsof -ti:$RELAY_PORT 2>/dev/null || true)
if [ ! -z "$RELAY_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping relay server (PID: $RELAY_PID)"
  kill -9 $RELAY_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} Relay server stopped"
else
  echo -e "  ${CYAN}→${NC} Relay server not running"
fi

# Kill API server
API_PID=$(lsof -ti:$API_PORT 2>/dev/null || true)
if [ ! -z "$API_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping API server (PID: $API_PID)"
  kill -9 $API_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} API server stopped"
else
  echo -e "  ${CYAN}→${NC} API server not running"
fi

# Kill MeshStorage API
MESH_PID=$(lsof -ti:$MESH_API_PORT 2>/dev/null || true)
if [ ! -z "$MESH_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping MeshStorage API (PID: $MESH_PID)"
  kill -9 $MESH_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} MeshStorage API stopped"
else
  echo -e "  ${CYAN}→${NC} MeshStorage API not running"
fi

sleep 1
echo -e "${GREEN}✓${NC} All processes stopped"

# Step 2: Clean databases
echo -e "\n${YELLOW}[2/6]${NC} Cleaning databases..."

DB_COUNT=0

# Main database
if [ -f "data/zentalk.db" ]; then
  rm -f data/zentalk.db
  echo -e "  ${CYAN}→${NC} Deleted zentalk.db"
  ((DB_COUNT++))
fi

if [ -f "data/zentalk.db-shm" ]; then
  rm -f data/zentalk.db-shm
  ((DB_COUNT++))
fi

if [ -f "data/zentalk.db-wal" ]; then
  rm -f data/zentalk.db-wal
  ((DB_COUNT++))
fi

# Relay database
if [ -f "data/relay-9001-queue.db" ]; then
  rm -f data/relay-9001-queue.db
  echo -e "  ${CYAN}→${NC} Deleted relay-9001-queue.db"
  ((DB_COUNT++))
fi

if [ -f "data/relay-9001-queue.db-shm" ]; then
  rm -f data/relay-9001-queue.db-shm
  ((DB_COUNT++))
fi

if [ -f "data/relay-9001-queue.db-wal" ]; then
  rm -f data/relay-9001-queue.db-wal
  ((DB_COUNT++))
fi

# Messages database (if exists)
if [ -f "data/messages.db" ]; then
  rm -f data/messages.db
  echo -e "  ${CYAN}→${NC} Deleted messages.db"
  ((DB_COUNT++))
fi

if [ -f "data/messages.db-shm" ]; then
  rm -f data/messages.db-shm
  ((DB_COUNT++))
fi

if [ -f "data/messages.db-wal" ]; then
  rm -f data/messages.db-wal
  ((DB_COUNT++))
fi

echo -e "${GREEN}✓${NC} Deleted $DB_COUNT database files"

# Step 3: Clean MeshStorage data
echo -e "\n${YELLOW}[3/6]${NC} Cleaning MeshStorage data..."

MESH_COUNT=0

# Clean all mesh storage directories
for DIR in mesh-data-clean cmd/mesh-api/test-data cmd/mesh-api/test-data-encrypted; do
  if [ -d "$DIR" ]; then
    # Delete chunks.db
    if [ -f "$DIR/chunks.db" ]; then
      rm -f "$DIR/chunks.db"
      rm -f "$DIR/chunks.db-shm" 2>/dev/null || true
      rm -f "$DIR/chunks.db-wal" 2>/dev/null || true
      echo -e "  ${CYAN}→${NC} Deleted $DIR/chunks.db"
      ((MESH_COUNT++))
    fi

    # Delete all shard files
    if [ -d "$DIR/shards" ]; then
      SHARD_FILES=$(find "$DIR/shards" -type f 2>/dev/null | wc -l)
      if [ $SHARD_FILES -gt 0 ]; then
        rm -rf "$DIR/shards"
        mkdir -p "$DIR/shards"
        echo -e "  ${CYAN}→${NC} Deleted $SHARD_FILES shard files from $DIR"
        ((MESH_COUNT++))
      fi
    fi
  fi
done

echo -e "${GREEN}✓${NC} Cleaned $MESH_COUNT MeshStorage locations"

# Step 4: Start Relay Server
echo -e "\n${YELLOW}[4/6]${NC} Starting Relay Server..."
echo -e "  ${CYAN}→${NC} Operator wallet: $OPERATOR_WALLET"
echo -e "  ${CYAN}→${NC} Contract address: $CONTRACT_ADDR"

# Create data directory if it doesn't exist
mkdir -p data

# Start relay server in background with required flags
nohup go run cmd/relay/main.go -port $RELAY_PORT -operator "$OPERATOR_WALLET" -contract "$CONTRACT_ADDR" > data/relay.log 2>&1 &
RELAY_NEW_PID=$!

sleep 2

# Verify relay is running
if kill -0 $RELAY_NEW_PID 2>/dev/null; then
  echo -e "${GREEN}✓${NC} Relay server started (PID: $RELAY_NEW_PID, Port: $RELAY_PORT)"
  echo -e "  ${CYAN}→${NC} Log: data/relay.log"
else
  echo -e "${RED}✗${NC} Failed to start relay server"
  echo -e "  ${CYAN}→${NC} Check data/relay.log for errors"
  exit 1
fi

# Copy relay public key to API directory (required for message encryption)
echo -e "  ${CYAN}→${NC} Copying relay public key to API directory..."
mkdir -p ../zentalk-api/keys
if [ -f "keys/relay.pem.pub" ]; then
  cp keys/relay.pem.pub ../zentalk-api/keys/
  echo -e "  ${GREEN}✓${NC} Relay public key copied to API"
else
  echo -e "  ${YELLOW}⚠${NC} Relay public key not found (will be generated on first use)"
fi

# Step 5: Start MeshStorage API
echo -e "\n${YELLOW}[5/6]${NC} Starting MeshStorage API..."

# Start mesh-api in background
nohup go run cmd/mesh-api/main.go > data/mesh-api.log 2>&1 &
MESH_NEW_PID=$!

sleep 2

# Verify mesh-api is running
if kill -0 $MESH_NEW_PID 2>/dev/null; then
  echo -e "${GREEN}✓${NC} MeshStorage API started (PID: $MESH_NEW_PID, Port: $MESH_API_PORT)"
  echo -e "  ${CYAN}→${NC} Log: data/mesh-api.log"
else
  echo -e "${RED}✗${NC} Failed to start MeshStorage API"
  echo -e "  ${CYAN}→${NC} Check data/mesh-api.log for errors"
  exit 1
fi

# Step 6: Start API Server (from zentalk-api folder)
echo -e "\n${YELLOW}[6/6]${NC} Starting API Server..."

# Check if zentalk-api binary exists
API_BINARY="../zentalk-api/api-server"
if [ -f "$API_BINARY" ]; then
  # Run the compiled binary
  cd ../zentalk-api
  nohup ./api-server > data/api-server.log 2>&1 &
  API_NEW_PID=$!
  cd - > /dev/null

  sleep 2

  # Verify api-server is running
  if kill -0 $API_NEW_PID 2>/dev/null; then
    echo -e "${GREEN}✓${NC} API server started (PID: $API_NEW_PID, Port: $API_PORT)"
    echo -e "  ${CYAN}→${NC} Log: ../zentalk-api/data/api-server.log"
  else
    echo -e "${RED}✗${NC} Failed to start API server"
    echo -e "  ${CYAN}→${NC} Check ../zentalk-api/data/api-server.log for errors"
    exit 1
  fi
else
  echo -e "${YELLOW}⚠${NC} API server binary not found at $API_BINARY"
  echo -e "  ${CYAN}→${NC} Build it first: cd ../zentalk-api && go build -o api-server cmd/api-server/main.go"
  echo -e "  ${CYAN}→${NC} Or start manually: cd ../zentalk-api && ./api-server"
  API_NEW_PID="N/A"
fi

# Summary
echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ Clean restart completed successfully!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${RED}⚠️  IMPORTANT: Refresh All Browser Windows!${NC}"
echo -e "${YELLOW}   All databases have been wiped.${NC}"
echo -e "${YELLOW}   Simply refresh (Cmd+R / Ctrl+R) all browser windows.${NC}"
echo -e "${YELLOW}   The app will automatically detect the backend was wiped${NC}"
echo -e "${YELLOW}   and clear localStorage, showing the registration modal.${NC}"
echo ""
echo -e "${CYAN}Running Services:${NC}"
echo -e "  • Relay Server:      http://localhost:$RELAY_PORT (PID: $RELAY_NEW_PID)"
echo -e "  • API Server:        http://localhost:$API_PORT (PID: $API_NEW_PID)"
echo -e "  • MeshStorage API:   http://localhost:$MESH_API_PORT (PID: $MESH_NEW_PID)"
echo ""
echo -e "${CYAN}Logs:${NC}"
echo -e "  • Relay:       tail -f data/relay.log"
echo -e "  • API:         tail -f ../zentalk-api/data/api-server.log"
echo -e "  • MeshStorage: tail -f data/mesh-api.log"
echo ""
echo -e "${CYAN}Useful Commands:${NC}"
echo -e "  • Stop all:   ./stop-all.sh"
echo -e "  • Test flow:  ./test_messaging.sh"
echo -e "  • View logs:  ./view-logs.sh"
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
