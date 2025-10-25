#!/bin/bash

# ZenTalk Services Status Script

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
echo -e "${BLUE}  ZenTalk Services Status${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

RUNNING_COUNT=0

# Check Relay Server
echo -e "${CYAN}Relay Server (Port $RELAY_PORT):${NC}"
RELAY_PID=$(lsof -ti:$RELAY_PORT 2>/dev/null || true)
if [ ! -z "$RELAY_PID" ]; then
  echo -e "  ${GREEN}✓ Running${NC} (PID: $RELAY_PID)"
  # Get memory usage
  MEM=$(ps -o rss= -p $RELAY_PID 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
  echo -e "  Memory: $MEM"
  ((RUNNING_COUNT++))
else
  echo -e "  ${RED}✗ Not running${NC}"
fi
echo ""

# Check API Server
echo -e "${CYAN}API Server (Port $API_PORT):${NC}"
API_PID=$(lsof -ti:$API_PORT 2>/dev/null || true)
if [ ! -z "$API_PID" ]; then
  echo -e "  ${GREEN}✓ Running${NC} (PID: $API_PID)"
  # Get memory usage
  MEM=$(ps -o rss= -p $API_PID 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
  echo -e "  Memory: $MEM"
  # Check if endpoint is responding
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$API_PORT/health 2>/dev/null || echo "000")
  if [ "$HTTP_STATUS" == "200" ]; then
    echo -e "  Health: ${GREEN}✓ OK${NC}"
  else
    echo -e "  Health: ${YELLOW}⚠ Unreachable${NC}"
  fi
  ((RUNNING_COUNT++))
else
  echo -e "  ${RED}✗ Not running${NC}"
fi
echo ""

# Check MeshStorage API
echo -e "${CYAN}MeshStorage API (Port $MESH_API_PORT):${NC}"
MESH_PID=$(lsof -ti:$MESH_API_PORT 2>/dev/null || true)
if [ ! -z "$MESH_PID" ]; then
  echo -e "  ${GREEN}✓ Running${NC} (PID: $MESH_PID)"
  # Get memory usage
  MEM=$(ps -o rss= -p $MESH_PID 2>/dev/null | awk '{printf "%.1f MB", $1/1024}')
  echo -e "  Memory: $MEM"
  # Check if endpoint is responding
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$MESH_API_PORT/health 2>/dev/null || echo "000")
  if [ "$HTTP_STATUS" == "200" ]; then
    echo -e "  Health: ${GREEN}✓ OK${NC}"
  else
    echo -e "  Health: ${YELLOW}⚠ Unreachable${NC}"
  fi
  ((RUNNING_COUNT++))
else
  echo -e "  ${RED}✗ Not running${NC}"
fi
echo ""

# Databases
echo -e "${CYAN}Databases:${NC}"

# Node databases
if [ -f "data/relay-9001-queue.db" ]; then
  DB_SIZE=$(du -h data/relay-9001-queue.db 2>/dev/null | cut -f1)
  echo -e "  relay-9001-queue.db: ${GREEN}✓${NC} ($DB_SIZE)"
else
  echo -e "  relay-9001-queue.db: ${YELLOW}⚠ Not found${NC}"
fi

if [ -f "mesh-data-clean/chunks.db" ]; then
  DB_SIZE=$(du -h mesh-data-clean/chunks.db 2>/dev/null | cut -f1)
  echo -e "  mesh chunks.db: ${GREEN}✓${NC} ($DB_SIZE)"
else
  echo -e "  mesh chunks.db: ${YELLOW}⚠ Not found${NC}"
fi

# API databases (in zentalk-api folder)
if [ -f "../zentalk-api/data/messages.db" ]; then
  DB_SIZE=$(du -h ../zentalk-api/data/messages.db 2>/dev/null | cut -f1)
  echo -e "  messages.db (API): ${GREEN}✓${NC} ($DB_SIZE)"
else
  echo -e "  messages.db (API): ${YELLOW}⚠ Not found${NC}"
fi
echo ""

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
if [ $RUNNING_COUNT -eq 3 ]; then
  echo -e "${GREEN}✓ All services running ($RUNNING_COUNT/3)${NC}"
elif [ $RUNNING_COUNT -eq 0 ]; then
  echo -e "${RED}✗ No services running${NC}"
  echo -e "\n${CYAN}Start services with:${NC} ./clean-restart.sh"
else
  echo -e "${YELLOW}⚠ Partial services running ($RUNNING_COUNT/3)${NC}"
  echo -e "\n${CYAN}Restart all services with:${NC} ./clean-restart.sh"
fi
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
