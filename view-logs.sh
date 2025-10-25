#!/bin/bash

# ZenTalk Log Viewer Script
# View logs from all services in real-time

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

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  ZenTalk Live Logs Viewer${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${CYAN}Press Ctrl+C to exit${NC}"
echo ""

# Check if logs exist
LOGS_EXIST=0
LOG_FILES=()

if [ -f "data/relay.log" ]; then
  LOGS_EXIST=1
  LOG_FILES+=("data/relay.log")
fi

if [ -f "../zentalk-api/data/api-server.log" ]; then
  LOGS_EXIST=1
  LOG_FILES+=("../zentalk-api/data/api-server.log")
fi

if [ -f "data/mesh-api.log" ]; then
  LOGS_EXIST=1
  LOG_FILES+=("data/mesh-api.log")
fi

if [ $LOGS_EXIST -eq 0 ]; then
  echo -e "${YELLOW}⚠ No log files found. Start the services first:${NC}"
  echo -e "  ./clean-restart.sh"
  echo ""
  exit 1
fi

# Follow all logs with prefixes
tail -f "${LOG_FILES[@]}" 2>/dev/null | \
  while IFS= read -r line; do
    # Add colored prefixes based on log content
    if [[ "$line" == *"relay"* ]] || [[ "$line" == *"Relay"* ]]; then
      echo -e "${CYAN}[RELAY]${NC} $line"
    elif [[ "$line" == *"API"* ]] || [[ "$line" == *"api"* ]]; then
      echo -e "${GREEN}[API]${NC} $line"
    elif [[ "$line" == *"mesh"* ]] || [[ "$line" == *"Mesh"* ]] || [[ "$line" == *"shard"* ]]; then
      echo -e "${YELLOW}[MESH]${NC} $line"
    else
      echo "$line"
    fi
  done
