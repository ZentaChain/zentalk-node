#!/bin/bash

# ZenTalk Stop All Services Script

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
RELAY_PORT=9001
API_PORT=3001
MESH_API_PORT=8080

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Stopping All ZenTalk Services${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

STOPPED_COUNT=0

# Kill relay server
RELAY_PID=$(lsof -ti:$RELAY_PORT 2>/dev/null || true)
if [ ! -z "$RELAY_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping relay server (PID: $RELAY_PID)"
  kill -9 $RELAY_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} Relay server stopped"
  ((STOPPED_COUNT++))
else
  echo -e "  ${YELLOW}⚠${NC} Relay server not running"
fi

# Kill API server
API_PID=$(lsof -ti:$API_PORT 2>/dev/null || true)
if [ ! -z "$API_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping API server (PID: $API_PID)"
  kill -9 $API_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} API server stopped"
  ((STOPPED_COUNT++))
else
  echo -e "  ${YELLOW}⚠${NC} API server not running"
fi

# Kill MeshStorage API
MESH_PID=$(lsof -ti:$MESH_API_PORT 2>/dev/null || true)
if [ ! -z "$MESH_PID" ]; then
  echo -e "  ${CYAN}→${NC} Stopping MeshStorage API (PID: $MESH_PID)"
  kill -9 $MESH_PID 2>/dev/null || true
  echo -e "  ${GREEN}✓${NC} MeshStorage API stopped"
  ((STOPPED_COUNT++))
else
  echo -e "  ${YELLOW}⚠${NC} MeshStorage API not running"
fi

echo ""
if [ $STOPPED_COUNT -eq 0 ]; then
  echo -e "${YELLOW}⚠ No services were running${NC}"
else
  echo -e "${GREEN}✓ Stopped $STOPPED_COUNT service(s)${NC}"
fi
echo ""
