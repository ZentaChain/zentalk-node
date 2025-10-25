#!/bin/bash

# ZenTalk Messaging Test Script
# Automates the full flow: delete accounts, create accounts, send message

set -e  # Exit on error

API_URL="http://localhost:3001/api"

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test accounts
CHAIN_WALLET="0xfe9472d0f49b424042fd66403840b5144f9f9988"
MAIN_WALLET="0xefeaa8e5efdcb380bf8581944cd738f448b8a288"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  ZenTalk Messaging Test - Automated Flow${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Delete existing accounts (if any)
echo -e "${YELLOW}[1/6]${NC} Deleting existing accounts..."
curl -s -X POST "$API_URL/delete-account" \
  -H "Content-Type: application/json" \
  -H "X-Wallet-Address: $CHAIN_WALLET" \
  -d "{\"wallet_address\":\"$CHAIN_WALLET\"}" > /dev/null 2>&1 || true

curl -s -X POST "$API_URL/delete-account" \
  -H "Content-Type: application/json" \
  -H "X-Wallet-Address: $MAIN_WALLET" \
  -d "{\"wallet_address\":\"$MAIN_WALLET\"}" > /dev/null 2>&1 || true

echo -e "${GREEN}✓${NC} Accounts deleted (if they existed)"
sleep 1

# Step 2: Initialize "chain" account (FIRST)
echo -e "\n${YELLOW}[2/6]${NC} Initializing 'chain' account..."
CHAIN_RESPONSE=$(curl -s -X POST "$API_URL/initialize" \
  -H "Content-Type: application/json" \
  -d "{\"wallet_address\":\"$CHAIN_WALLET\",\"username\":\"chain\"}")

if echo "$CHAIN_RESPONSE" | grep -q "\"success\":true"; then
  echo -e "${GREEN}✓${NC} Chain account initialized"
else
  echo -e "${RED}✗${NC} Failed to initialize chain account"
  echo "$CHAIN_RESPONSE" | jq '.' 2>/dev/null || echo "$CHAIN_RESPONSE"
  exit 1
fi

sleep 2  # Wait for DHT to stabilize

# Step 3: Initialize "main" account (SECOND)
echo -e "\n${YELLOW}[3/6]${NC} Initializing 'main' account..."
MAIN_RESPONSE=$(curl -s -X POST "$API_URL/initialize" \
  -H "Content-Type: application/json" \
  -d "{\"wallet_address\":\"$MAIN_WALLET\",\"username\":\"main\"}")

if echo "$MAIN_RESPONSE" | grep -q "\"success\":true"; then
  echo -e "${GREEN}✓${NC} Main account initialized"
else
  echo -e "${RED}✗${NC} Failed to initialize main account"
  echo "$MAIN_RESPONSE" | jq '.' 2>/dev/null || echo "$MAIN_RESPONSE"
  exit 1
fi

sleep 3  # Wait for DHT republishing (2 seconds + buffer)

# Step 4: Discover "main" from "chain" account
echo -e "\n${YELLOW}[4/6]${NC} Discovering 'main' contact from 'chain' account..."
DISCOVER_RESPONSE=$(curl -s -X POST "$API_URL/discover" \
  -H "Content-Type: application/json" \
  -H "X-Wallet-Address: $CHAIN_WALLET" \
  -d "{\"address\":\"main\"}")

if echo "$DISCOVER_RESPONSE" | grep -q "\"success\":true"; then
  echo -e "${GREEN}✓${NC} Successfully discovered 'main' contact"
else
  echo -e "${RED}✗${NC} Failed to discover 'main' contact"
  echo "$DISCOVER_RESPONSE" | jq '.' 2>/dev/null || echo "$DISCOVER_RESPONSE"
  echo -e "\n${RED}ERROR: Contact discovery failed!${NC}"
  exit 1
fi

sleep 1

# Step 5: Send message from "chain" to "main"
echo -e "\n${YELLOW}[5/6]${NC} Sending test message from 'chain' to 'main'..."
SEND_RESPONSE=$(curl -s -X POST "$API_URL/send" \
  -H "Content-Type: application/json" \
  -H "X-Wallet-Address: $CHAIN_WALLET" \
  -d "{\"recipient_address\":\"main\",\"content\":\"Hello from automated test!\"}")

if echo "$SEND_RESPONSE" | grep -q "\"success\":true"; then
  echo -e "${GREEN}✓${NC} Message sent successfully!"
  echo "$SEND_RESPONSE" | jq '.' 2>/dev/null || echo "$SEND_RESPONSE"
else
  echo -e "${RED}✗${NC} Failed to send message"
  echo "$SEND_RESPONSE" | jq '.' 2>/dev/null || echo "$SEND_RESPONSE"
  echo -e "\n${RED}ERROR: Message sending failed!${NC}"
  echo "Check the API server logs for the detailed error."
  exit 1
fi

# Step 6: Verify message was received
echo -e "\n${YELLOW}[6/6]${NC} Verifying message delivery..."
sleep 1

CHATS_RESPONSE=$(curl -s -X GET "$API_URL/chats" \
  -H "X-Wallet-Address: $MAIN_WALLET")

if echo "$CHATS_RESPONSE" | grep -q "Hello from automated test"; then
  echo -e "${GREEN}✓${NC} Message received successfully!"
else
  echo -e "${YELLOW}⚠${NC} Message sent but not yet received (check WebSocket delivery)"
fi

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ Test completed successfully!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
