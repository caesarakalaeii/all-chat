# All-Chat: Exhaustive Testing Guide

**Version**: 1.0
**Date**: November 13, 2025
**Status**: Complete Testing Suite for Phase 4
**Estimated Time**: 2-4 hours for full test suite

---

## Table of Contents

1. [Prerequisites & Setup](#prerequisites--setup)
2. [Environment Configuration](#environment-configuration)
3. [Phase 1: Unit Tests](#phase-1-unit-tests)
4. [Phase 2: Integration Tests](#phase-2-integration-tests)
5. [Phase 3: Service Tests](#phase-3-service-tests)
6. [Phase 4: End-to-End Tests](#phase-4-end-to-end-tests)
7. [Phase 5: Multi-Platform Tests](#phase-5-multi-platform-tests)
8. [Phase 6: Load Tests](#phase-6-load-tests)
9. [Phase 7: Kubernetes Tests](#phase-7-kubernetes-tests)
10. [Verification Checklist](#verification-checklist)
11. [Troubleshooting](#troubleshooting)

---

## Prerequisites & Setup

### Required Software

```bash
# Check versions
go version                    # Requires 1.25+
docker --version              # Requires 20.10+
docker-compose --version      # Requires 2.0+
psql --version                # Requires 14+
redis-cli --version           # Requires 6+
curl --version                # Any recent version
jq --version                  # For JSON parsing
websocat --version            # For WebSocket testing (optional)

# Install missing tools
# Arch Linux
sudo pacman -S go docker docker-compose postgresql redis jq

# Ubuntu/Debian
sudo apt-get install golang docker.io docker-compose postgresql-client redis-tools jq

# WebSocket testing tool
cargo install websocat
# Or
wget -qO- https://github.com/vi/websocat/releases/latest/download/websocat.x86_64-unknown-linux-musl > /usr/local/bin/websocat
chmod +x /usr/local/bin/websocat
```

### Clone and Setup

```bash
# Clone repository
cd /home/moersener/Hobby/all-chat

# Verify structure
ls -la services/
ls -la migrations/

# Expected services:
# auth-service, overlay-manager, emote-service, api-gateway,
# twitch-listener, youtube-listener, message-processor, source-manager
```

---

## Environment Configuration

### Step 1: Copy Environment Template

```bash
cd /home/moersener/Hobby/all-chat
cp .env.example deployments/.env
```

### Step 2: Fill in Required Variables

Edit `deployments/.env` with your credentials:

```bash
# Open in editor
nano deployments/.env
# or
vim deployments/.env
```

**Required Variables:**

```bash
# ==============================================
# Twitch OAuth (REQUIRED)
# Get from: https://dev.twitch.tv/console/apps
# ==============================================
TWITCH_CLIENT_ID=your_actual_client_id_here
TWITCH_CLIENT_SECRET=your_actual_client_secret_here

# ==============================================
# Twitch Bot Account (REQUIRED for Twitch Listener)
# Get OAuth token from: https://twitchapps.com/tmi/
# ==============================================
TWITCH_BOT_USERNAME=your_bot_username
TWITCH_BOT_OAUTH=oauth:your_actual_oauth_token_here

# ==============================================
# YouTube OAuth (REQUIRED for Phase 4)
# Get from: https://console.cloud.google.com/
# 1. Create project
# 2. Enable YouTube Data API v3
# 3. Create OAuth 2.0 credentials
# ==============================================
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-your_actual_secret_here

# ==============================================
# JWT Secret (REQUIRED)
# Generate with: openssl rand -base64 32
# ==============================================
JWT_SECRET=your_generated_jwt_secret_here

# ==============================================
# Database (defaults OK for local testing)
# ==============================================
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# ==============================================
# Redis (defaults OK for local testing)
# ==============================================
REDIS_HOST=localhost
REDIS_PORT=6379

# ==============================================
# Frontend URL (used to build OAuth redirect URIs)
# ==============================================
FRONTEND_URL=http://localhost:3000

# ==============================================
# Application Settings
# ==============================================
LOG_LEVEL=info
CORS_ORIGIN=http://localhost:3000
```

### Step 3: Verify Environment File

```bash
# Check file exists
cat deployments/.env

# Verify no placeholder values remain
grep -E "(your_|xxx|GOCSPX-your)" deployments/.env

# If any placeholders remain, fill them in before proceeding
```

### Step 4: Set Up OAuth Applications

#### Twitch OAuth Setup

1. Go to https://dev.twitch.tv/console/apps
2. Click "Register Your Application"
3. Name: "All-Chat Local Dev"
4. OAuth Redirect URLs: `<FRONTEND_URL>/api/v1/auth/twitch/callback` (e.g., `http://localhost:8080/api/v1/auth/twitch/callback` for local Docker)
5. Category: Chat Bot
6. Copy Client ID and Client Secret to `.env`

#### Twitch Bot OAuth Token

1. Go to https://twitchapps.com/tmi/
2. Click "Connect"
3. Authorize the application
4. Copy the OAuth token (format: `oauth:abc123...`)
5. Add to `.env` as `TWITCH_BOT_OAUTH`

#### YouTube OAuth Setup

1. Go to https://console.cloud.google.com/
2. Create new project: "all-chat-dev"
3. Enable YouTube Data API v3:
   - APIs & Services → Library
   - Search "YouTube Data API v3"
   - Click Enable
4. Create OAuth 2.0 Credentials:
   - APIs & Services → Credentials
   - Create Credentials → OAuth client ID
   - Application type: Web application
   - Authorized redirect URIs: `http://localhost:8080/api/v1/auth/youtube/callback`
5. Copy Client ID and Client Secret to `.env`

---

## Phase 1: Unit Tests

### Test 1.1: Shared Packages

```bash
# Test shared packages (foundation)
cd /home/moersener/Hobby/all-chat

# Database utilities
cd shared/database && go test -v ./...

# Logger
cd ../logger && go test -v ./...

# Auth utilities
cd ../auth && go test -v ./...

# Redis client
cd ../redis && go test -v ./...

# Middleware
cd ../middleware && go test -v ./...

# All shared packages
cd .. && go test -v ./...
```

**Expected**: All shared package tests pass

### Test 1.2: Auth Service

```bash
cd /home/moersener/Hobby/all-chat/services/auth-service

# Run all tests
go test -v ./...

# Run with coverage
go test -v -cover ./...

# Run short tests only (no database)
go test -v -short ./...

# Test specific packages
go test -v ./adapters/api/...
go test -v ./adapters/repository/...
go test -v ./core/services/...
```

**Expected**: 48 tests passing, ~85% coverage

### Test 1.3: Overlay Manager

```bash
cd /home/moersener/Hobby/all-chat/services/overlay-manager

# Run all tests
go test -v ./...

# With coverage
go test -v -cover ./...
```

**Expected**: 48 tests passing, ~82% coverage

### Test 1.4: Emote Service

```bash
cd /home/moersener/Hobby/all-chat/services/emote-service

# Run all tests
go test -v ./...

# With coverage
go test -v -cover ./...
```

**Expected**: 62 tests passing, 81.8% coverage

### Test 1.5: API Gateway

```bash
cd /home/moersener/Hobby/all-chat/services/api-gateway

# Run all tests
go test -v ./...

# With coverage
go test -v -cover ./...
```

**Expected**: 17 tests passing, 90.9% coverage

### Test 1.6: Twitch Listener

```bash
cd /home/moersener/Hobby/all-chat/services/twitch-listener

# Run all tests
go test -v ./...

# Short tests (no Redis)
go test -v -short ./...

# With coverage
go test -v -cover ./...
```

**Expected**: 22 tests passing, ~95% coverage

### Test 1.7: Message Processor

```bash
cd /home/moersener/Hobby/all-chat/services/message-processor

# Run all tests
go test -v ./...

# Test normalizers specifically
go test -v ./normalizer/...

# With coverage
go test -v -cover ./...
```

**Expected**: 8+ tests passing (now includes YouTube normalizer tests), 100% normalizer coverage

### Test 1.8: YouTube Listener (NEW)

```bash
cd /home/moersener/Hobby/all-chat/services/youtube-listener

# Run all tests
go test -v ./...

# Test specific packages
go test -v ./oauth/...
go test -v ./api/...
go test -v ./quota/...

# With coverage
go test -v -cover ./...
```

**Expected**: Tests for OAuth, parser, and quota tracker passing

### Test 1.9: Source Manager (NEW)

```bash
cd /home/moersener/Hobby/all-chat/services/source-manager

# Run all tests
go test -v ./...

# Test leader election specifically
go test -v ./election/...

# With coverage
go test -v -cover ./...
```

**Expected**: Leader election tests passing with miniredis

### Test 1.10: Run All Unit Tests

```bash
cd /home/moersener/Hobby/all-chat

# Run all tests across all services
go test ./... -v

# Run with coverage report
go test ./... -cover

# Generate HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
# Open coverage.html in browser

# Run short tests (no integration)
go test ./... -short
```

**Expected**: 200+ tests passing across all services

---

## Phase 2: Integration Tests

### Test 2.1: Start Infrastructure

```bash
cd /home/moersener/Hobby/all-chat/deployments

# Start PostgreSQL and Redis
docker-compose up -d postgres redis

# Wait for healthy status
docker-compose ps

# Verify PostgreSQL
docker-compose exec postgres pg_isready -U allchat
# Expected: "postgres:5432 - accepting connections"

# Verify Redis
docker-compose exec redis redis-cli ping
# Expected: "PONG"
```

### Test 2.2: Apply Database Migrations

```bash
cd /home/moersener/Hobby/all-chat

# Connect to PostgreSQL
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat

# Inside psql:
# Check if tables exist
\dt

# If empty, apply migrations
\q

# Apply migrations
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/001_initial_schema.sql

# Apply YouTube support
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/003_youtube_support.sql

# Verify tables
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c "\dt"

# Expected tables:
# users
# overlays
# overlay_configs
# overlay_chat_sources
# youtube_oauth_tokens
# youtube_quota_usage
# supported_platforms
```

### Test 2.3: Initialize Redis Consumer Group

```bash
# Connect to Redis
redis-cli -h localhost

# Create consumer group for message-processor
XGROUP CREATE chat:raw message-processor $ MKSTREAM

# Verify group created
XINFO GROUPS chat:raw

# Expected: Consumer group "message-processor" exists

# Exit
exit
```

### Test 2.4: Test Database Connectivity

```bash
# Test connection from each service
cd /home/moersener/Hobby/all-chat

# Auth Service
cd services/auth-service
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Quick connection test (won't run service, just check DB)
go run ./cmd &
PID=$!
sleep 3
kill $PID 2>/dev/null

# Check logs for "Connected to PostgreSQL"
```

---

## Phase 3: Service Tests

### Test 3.1: Auth Service

**Start Service:**
```bash
cd /home/moersener/Hobby/all-chat/services/auth-service

# Set environment variables
export PORT=8081
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379
export TWITCH_CLIENT_ID=$(grep TWITCH_CLIENT_ID ../../deployments/.env | cut -d'=' -f2)
export TWITCH_CLIENT_SECRET=$(grep TWITCH_CLIENT_SECRET ../../deployments/.env | cut -d'=' -f2)
export FRONTEND_URL=http://localhost:3000
export JWT_SECRET=$(grep JWT_SECRET ../../deployments/.env | cut -d'=' -f2)

# Run service
go run ./cmd
```

**Test Endpoints (in another terminal):**
```bash
# Health check
curl http://localhost:8081/health/live
# Expected: {"status":"alive"}

curl http://localhost:8081/health/ready
# Expected: {"status":"ready"}

# Login endpoint
curl http://localhost:8081/twitch/login
# Expected: Redirect to Twitch OAuth

# Test with invalid token
curl -H "Authorization: Bearer invalid-token" http://localhost:8081/auth/me
# Expected: 401 Unauthorized
```

**Stop Service:** `Ctrl+C`

### Test 3.2: Overlay Manager

```bash
cd /home/moersener/Hobby/all-chat/services/overlay-manager

# Set environment
export PORT=8082
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=$(grep JWT_SECRET ../../deployments/.env | cut -d'=' -f2)

# Run service
go run ./cmd
```

**Test Endpoints:**
```bash
# Health check
curl http://localhost:8082/health/live
# Expected: {"status":"alive"}

# List overlays (requires valid JWT)
curl -H "Authorization: Bearer <your-jwt>" http://localhost:8082/overlays
# Expected: {"overlays":[...]}
```

**Stop Service:** `Ctrl+C`

### Test 3.3: Emote Service

```bash
cd /home/moersener/Hobby/all-chat/services/emote-service

# Set environment
export PORT=8083
export LOG_LEVEL=debug
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Run service
go run ./cmd
```

**Test Endpoints:**
```bash
# Health check
curl http://localhost:8083/health/live
# Expected: {"status":"alive"}

# Fetch emotes for a channel
curl http://localhost:8083/emotes/channel/xqc
# Expected: JSON with emotes from 7TV, BTTV, FFZ

# Check Redis caching
redis-cli -h localhost KEYS "emotes:*"
# Expected: Keys like "emotes:7tv:xqc", "emotes:bttv:xqc"

# Second request should be faster (cached)
time curl http://localhost:8083/emotes/channel/xqc > /dev/null
```

**Stop Service:** `Ctrl+C`

### Test 3.4: Twitch Listener

```bash
cd /home/moersener/Hobby/all-chat/services/twitch-listener

# Set environment
export PORT=8085
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379
export TWITCH_BOT_USERNAME=$(grep TWITCH_BOT_USERNAME ../../deployments/.env | cut -d'=' -f2)
export TWITCH_BOT_OAUTH=$(grep TWITCH_BOT_OAUTH ../../deployments/.env | cut -d'=' -f2)

# Run service
go run ./cmd
```

**Test Functionality:**
```bash
# Check health
curl http://localhost:8085/health/live
# Expected: {"status":"alive"}

# Check status (shows connected channels)
curl http://localhost:8085/status | jq
# Expected: {"status":"ok","irc":{"connected":true,"channels":[...]}}

# Add a test overlay with Twitch source
# (Use overlay manager to create overlay and add source)
# Twitch Listener should auto-join the channel within 30 seconds

# Check Redis Streams for messages
redis-cli -h localhost
> XLEN chat:raw
# Should increase as messages come in

> XREAD COUNT 10 STREAMS chat:raw 0
# Should show recent Twitch messages
```

**Stop Service:** `Ctrl+C`

### Test 3.5: YouTube Listener (NEW)

```bash
cd /home/moersener/Hobby/all-chat/services/youtube-listener

# Set environment
export PORT=8086
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379
export YOUTUBE_CLIENT_ID=$(grep YOUTUBE_CLIENT_ID ../../deployments/.env | cut -d'=' -f2)
export YOUTUBE_CLIENT_SECRET=$(grep YOUTUBE_CLIENT_SECRET ../../deployments/.env | cut -d'=' -f2)
export FRONTEND_URL=http://localhost:3000
export POLLING_INTERVAL_MS=2000
export QUOTA_LIMIT_DAILY=10000

# Run service
go run ./cmd
```

**Test Functionality:**
```bash
# Check health
curl http://localhost:8086/health/live
# Expected: {"status":"alive"}

curl http://localhost:8086/health/ready
# Expected: {"status":"ready"}

# Check status
curl http://localhost:8086/status | jq
# Expected:
# {
#   "status": "running",
#   "streams": {
#     "active_count": 0,
#     "streams": []
#   },
#   "quota": {
#     "used": 0,
#     "remaining": 10000,
#     "percentage": 0
#   }
# }

# Note: No streams will be active until:
# 1. User completes YouTube OAuth
# 2. Overlay with YouTube source is created
# 3. YouTube channel is actually streaming live
```

**Stop Service:** `Ctrl+C`

### Test 3.6: Source Manager (NEW)

```bash
cd /home/moersener/Hobby/all-chat/services/source-manager

# Set environment
export PORT=8088
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Run service
go run ./cmd
```

**Test Functionality:**
```bash
# Check health
curl http://localhost:8088/health/live
# Expected: {"status":"alive"}

# Check status
curl http://localhost:8088/status | jq
# Expected:
# {
#   "status": "running",
#   "instance_id": "uuid-here",
#   "registry": {
#     "total_sources": 0,
#     "platform_counts": {}
#   },
#   "leadership": {
#     "total_streams": 0,
#     "leader_count": 0,
#     "streams": []
#   }
# }

# Test leadership claim
curl -X POST http://localhost:8088/leadership/claim \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","stream_id":"test123"}' | jq

# Expected: {"acquired":true,"instance_id":"...","platform":"youtube","stream_id":"test123"}

# Verify in Redis
redis-cli -h localhost GET leader:youtube:test123
# Expected: instance ID

# Test heartbeat/renew
curl -X POST http://localhost:8088/leadership/renew \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","stream_id":"test123"}' | jq

# Expected: {"renewed":true,...}

# Release leadership
curl -X POST http://localhost:8088/leadership/release \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","stream_id":"test123"}' | jq

# Expected: {"released":true,...}
```

**Stop Service:** `Ctrl+C`

### Test 3.7: Message Processor

```bash
cd /home/moersener/Hobby/all-chat/services/message-processor

# Set environment
export PORT=8087
export LOG_LEVEL=debug
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=allchat
export DATABASE_PASSWORD=allchat_dev_password
export DATABASE_NAME=allchat
export REDIS_HOST=localhost
export REDIS_PORT=6379
export EMOTE_SERVICE_URL=http://localhost:8083

# Run service
go run ./cmd
```

**Test Functionality:**
```bash
# Check health
curl http://localhost:8087/health/live
# Expected: {"status":"alive"}

# Check status
curl http://localhost:8087/status | jq
# Expected: {"status":"ok","consumer":{"pending_messages":0}}

# Monitor logs for message processing
# Should see "Message processor started"
# Will process messages from chat:raw stream
```

**Stop Service:** `Ctrl+C`

---

## Phase 4: End-to-End Tests

### Test 4.1: Start All Services with Docker Compose

```bash
cd /home/moersener/Hobby/all-chat/deployments

# Start all services
docker-compose up -d

# Check all services are running
docker-compose ps

# Expected: All services "Up" and healthy

# View logs
docker-compose logs -f

# Or specific service
docker-compose logs -f youtube-listener
docker-compose logs -f source-manager
```

### Test 4.2: Verify All Health Checks

```bash
# Create verification script
cat > /tmp/test-health.sh << 'EOF'
#!/bin/bash

services=(
  "8080:API Gateway"
  "8081:Auth Service"
  "8082:Overlay Manager"
  "8083:Emote Service"
  "8085:Twitch Listener"
  "8086:YouTube Listener"
  "8087:Message Processor"
  "8088:Source Manager"
)

echo "========================================="
echo "Health Check Verification"
echo "========================================="

for svc in "${services[@]}"; do
  port=$(echo "$svc" | cut -d':' -f1)
  name=$(echo "$svc" | cut -d':' -f2)

  if curl -s -f "http://localhost:$port/health/live" > /dev/null; then
    echo "✓ $name ($port)"
  else
    echo "✗ $name ($port) - FAILED"
  fi
done
EOF

chmod +x /tmp/test-health.sh
/tmp/test-health.sh
```

**Expected**: All services show ✓

### Test 4.3: Twitch OAuth Flow Test

```bash
# Get login URL
curl http://localhost:8080/api/v1/auth/twitch/login

# Expected: Redirect URL to Twitch OAuth
# Copy the URL and open in browser

# Complete OAuth flow in browser
# You'll be redirected back to: http://localhost:8080/api/v1/auth/twitch/callback?code=...

# The callback should return a JWT token
# Copy the token for next tests

TOKEN="paste-jwt-token-here"

# Test token validity
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/auth/me | jq

# Expected:
# {
#   "id": "uuid",
#   "twitch_id": "123456",
#   "username": "your_username",
#   "display_name": "Your Display Name",
#   ...
# }
```

### Test 4.4: Create Overlay with Twitch Source

```bash
TOKEN="your-jwt-token-from-above"

# Create overlay
OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Overlay 1","description":"Testing Twitch"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

echo "Overlay ID: $OVERLAY_ID"

# Verify overlay created
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID | jq

# Add Twitch source (e.g., xqc channel)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"twitch","channel_id":"xqc"}' \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources | jq

# Expected: Source created successfully

# List sources
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources | jq

# Expected: Array with one Twitch source
```

**Wait 30 seconds for Twitch Listener to sync and JOIN channel**

```bash
# Check Twitch Listener status
curl http://localhost:8085/status | jq '.irc.channels'

# Expected: Array includes "xqc"
```

### Test 4.5: WebSocket Connection Test

```bash
TOKEN="your-jwt-token"
OVERLAY_ID="your-overlay-id"

# Connect to WebSocket
websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN"

# Expected: Connection established
# Wait for messages to appear

# Go to https://www.twitch.tv/xqc and send a chat message
# Expected: Message appears in WebSocket within 500ms
# Format:
# {
#   "type": "chat_message",
#   "data": {
#     "platform": "twitch",
#     "user": {...},
#     "message": {...}
#   }
# }
```

### Test 4.6: Verify Message Flow (Twitch)

```bash
# Terminal 1: Monitor Redis Streams
redis-cli -h localhost
> XREAD COUNT 10 STREAMS chat:raw $

# Terminal 2: Monitor Redis Pub/Sub
redis-cli -h localhost
> SUBSCRIBE overlay:*

# Terminal 3: Send Twitch message
# Go to twitch.tv/xqc and type in chat

# Expected flow:
# 1. Message appears in Terminal 1 (Redis Streams)
# 2. Message appears in Terminal 2 (Redis Pub/Sub)
# 3. Message appears in WebSocket connection

# Verify message fields:
# - platform: "twitch"
# - user.username: your twitch username
# - message.text: your message
# - message.emotes: array (if you used emotes)
```

### Test 4.7: YouTube OAuth Flow Test (Manual)

**Note**: This requires a YouTube channel that is currently live streaming.

```bash
# YouTube OAuth flow must be handled by Auth Service
# For now, manually insert a test token into database

# Get a test OAuth token from YouTube OAuth Playground:
# https://developers.google.com/oauthplayground/
# Scopes: https://www.googleapis.com/auth/youtube.readonly

# Insert into database
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat << EOF
INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, token_type, expiry)
SELECT
  u.id,
  'UCxxxxxx',  -- Replace with actual YouTube channel ID
  'ya29.your-access-token',  -- Replace with actual token
  'your-refresh-token',      -- Replace with actual refresh token
  'Bearer',
  NOW() + INTERVAL '1 hour'
FROM users u
WHERE u.username = 'your-twitch-username'
LIMIT 1;
EOF

# Verify inserted
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c \
  "SELECT channel_id, expiry FROM youtube_oauth_tokens;"
```

### Test 4.8: Create Overlay with YouTube Source

```bash
TOKEN="your-jwt-token"

# Create YouTube overlay
YOUTUBE_OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"YouTube Test Overlay","description":"Testing YouTube"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

echo "YouTube Overlay ID: $YOUTUBE_OVERLAY_ID"

# Add YouTube source (must match OAuth token channel)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","channel_id":"UCxxxxxx"}' \
  http://localhost:8080/api/v1/overlays/$YOUTUBE_OVERLAY_ID/sources | jq

# Expected: Source created

# Wait 30 seconds for YouTube Listener to sync
sleep 30

# Check YouTube Listener status
curl http://localhost:8086/status | jq

# Expected: If channel is live, streams.active_count > 0
# If not live: active_count = 0
```

### Test 4.9: Verify Message Flow (YouTube)

**Prerequisites**: YouTube channel must be live streaming

```bash
# Monitor YouTube Listener logs
docker-compose logs -f youtube-listener

# Monitor Redis Streams
redis-cli -h localhost
> XREAD COUNT 10 STREAMS chat:raw $

# Connect WebSocket
websocat "ws://localhost:8080/ws/overlay/$YOUTUBE_OVERLAY_ID?token=$TOKEN"

# Send a message in YouTube live chat

# Expected flow:
# 1. YouTube Listener polls and detects message
# 2. Message published to Redis Streams (platform="youtube")
# 3. Message Processor normalizes with YouTube normalizer
# 4. Message enriched with emotes
# 5. Message published to Pub/Sub
# 6. WebSocket receives message

# Verify message fields:
# - platform: "youtube"
# - user.badges: ["member", "moderator", etc.]
# - metadata.super_chat_amount: 0 (or amount if Super Chat)
# - user.color: "" (YouTube doesn't have colors)
```

---

## Phase 5: Multi-Platform Tests

### Test 5.1: Create Multi-Platform Overlay

```bash
TOKEN="your-jwt-token"

# Create overlay for both platforms
MULTI_OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Multi-Platform Overlay","description":"Twitch + YouTube"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

echo "Multi-Platform Overlay ID: $MULTI_OVERLAY_ID"

# Add Twitch source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"twitch","channel_id":"xqc"}' \
  http://localhost:8080/api/v1/overlays/$MULTI_OVERLAY_ID/sources | jq

# Add YouTube source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","channel_id":"UCxxxxxx"}' \
  http://localhost:8080/api/v1/overlays/$MULTI_OVERLAY_ID/sources | jq

# List all sources
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/overlays/$MULTI_OVERLAY_ID/sources | jq

# Expected: 2 sources (one Twitch, one YouTube)
```

### Test 5.2: Multi-Platform WebSocket Test

```bash
TOKEN="your-jwt-token"
MULTI_OVERLAY_ID="your-multi-platform-overlay-id"

# Connect to WebSocket
websocat "ws://localhost:8080/ws/overlay/$MULTI_OVERLAY_ID?token=$TOKEN"

# Expected: Receive messages from BOTH platforms
# - Twitch messages: platform="twitch", low latency (<500ms)
# - YouTube messages: platform="youtube", higher latency (~2-5s)
# - Messages interleaved by timestamp
# - Both platforms' emotes enriched
```

### Test 5.3: Verify Platform-Specific Features

**Twitch Message Structure:**
```json
{
  "type": "chat_message",
  "data": {
    "platform": "twitch",
    "user": {
      "color": "#FF0000",
      "badges": ["subscriber", "moderator"]
    },
    "metadata": {
      "is_subscriber": true,
      "is_moderator": false,
      "bits": 0
    }
  }
}
```

**YouTube Message Structure:**
```json
{
  "type": "chat_message",
  "data": {
    "platform": "youtube",
    "user": {
      "color": "",
      "badges": ["member", "moderator"],
      "avatar_url": "https://..."
    },
    "metadata": {
      "is_sponsor": true,
      "super_chat_amount": 0
    }
  }
}
```

### Test 5.4: Leader Election Test (Multiple Instances)

**Terminal 1:**
```bash
cd /home/moersener/Hobby/all-chat/services/source-manager
export PORT=8088 DATABASE_HOST=localhost REDIS_HOST=localhost
go run ./cmd
```

**Terminal 2:**
```bash
cd /home/moersener/Hobby/all-chat/services/source-manager
export PORT=8089 DATABASE_HOST=localhost REDIS_HOST=localhost
go run ./cmd
```

**Terminal 3:**
```bash
# Check both instances
curl http://localhost:8088/status | jq '.instance_id'
curl http://localhost:8089/status | jq '.instance_id'

# Should show different instance IDs

# Create a YouTube source (requires live stream)
# Only ONE instance should become leader

curl http://localhost:8088/leadership | jq
curl http://localhost:8089/leadership | jq

# Expected: Same stream shows same leader_id in both responses
# Only one instance is leader (is_leader: true)

# Kill leader instance (Ctrl+C in that terminal)
# Wait 15 seconds for lock to expire

# Check leadership again
curl http://localhost:8088/leadership | jq  # or 8089 if that one is alive

# Expected: Remaining instance takes over leadership
```

**Stop Services:** `Ctrl+C` in both terminals

---

## Phase 6: Load Tests

### Test 6.1: Multiple Overlays (Twitch)

```bash
TOKEN="your-jwt-token"

# Create 10 overlays with Twitch sources
for i in {1..10}; do
  OVERLAY_ID=$(curl -s -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Load Test Overlay $i\"}" \
    http://localhost:8080/api/v1/overlays | jq -r '.id')

  curl -s -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"platform":"twitch","channel_id":"xqc"}' \
    http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources > /dev/null

  echo "Created overlay $i: $OVERLAY_ID"
done

# Check Twitch Listener status
curl http://localhost:8085/status | jq '.irc.channels'

# Expected: Still only one "xqc" entry (channel reuse)

# Check Message Processor routes correctly
# All 10 overlays should receive the same messages
```

### Test 6.2: Redis Streams Performance

```bash
# Monitor stream length
watch -n 1 'redis-cli -h localhost XLEN chat:raw'

# Monitor consumer group lag
watch -n 1 'redis-cli -h localhost XPENDING chat:raw message-processor'

# Expected:
# - Stream length grows as messages come in
# - Pending count stays low (<100) if processor is keeping up
# - If pending grows, processor is falling behind
```

### Test 6.3: Message Throughput Test

```bash
# Generate test messages (simulate high volume)
# Create a test script

cat > /tmp/test-throughput.sh << 'EOF'
#!/bin/bash

# Publish test messages directly to Redis Streams
for i in {1..1000}; do
  redis-cli -h localhost XADD chat:raw "*" \
    message_id "test-$i" \
    platform "twitch" \
    channel_id "test-channel" \
    user_id "12345" \
    username "testuser" \
    text "Test message $i" \
    timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    data "{\"message_id\":\"test-$i\"}"
done

echo "Published 1000 test messages"
EOF

chmod +x /tmp/test-throughput.sh

# Run throughput test
time /tmp/test-throughput.sh

# Monitor processing
watch -n 1 'curl -s http://localhost:8087/status | jq ".consumer.pending_messages"'

# Expected: Message Processor handles 1000+ messages per second
# Pending count should decrease quickly
```

### Test 6.4: WebSocket Load Test (Multiple Connections)

```bash
# Install connection testing tool
npm install -g wscat  # or use websocat

TOKEN="your-jwt-token"
OVERLAY_ID="your-overlay-id"

# Create test script for multiple connections
cat > /tmp/test-websocket-load.sh << 'EOF'
#!/bin/bash

TOKEN=$1
OVERLAY_ID=$2
CONNECTIONS=${3:-10}

for i in $(seq 1 $CONNECTIONS); do
  websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN" > /tmp/ws-$i.log 2>&1 &
  echo "Started connection $i (PID: $!)"
done

echo "Started $CONNECTIONS WebSocket connections"
echo "Logs in /tmp/ws-*.log"
echo "Kill with: pkill websocat"
EOF

chmod +x /tmp/test-websocket-load.sh

# Start 10 connections
/tmp/test-websocket-load.sh "$TOKEN" "$OVERLAY_ID" 10

# Send messages in Twitch chat
# Expected: All connections receive the same messages

# View connection logs
tail -f /tmp/ws-*.log

# Cleanup
pkill websocat
```

### Test 6.5: YouTube Quota Tracking Test

```bash
# Monitor quota usage
watch -n 5 'curl -s http://localhost:8086/status | jq ".quota"'

# Expected output:
# {
#   "used": 100,        # Increases with each poll (5 units per poll)
#   "remaining": 9900,
#   "percentage": 1.0
# }

# Check YouTube Listener logs for quota warnings
docker-compose logs youtube-listener | grep -i quota

# Expected: Warnings when usage reaches 80%, errors at 90%
```

---

## Phase 7: Kubernetes Tests

### Test 7.1: k3d Cluster Setup

```bash
cd /home/moersener/Hobby/all-chat/deployments/ansible

# Run Ansible playbook
ansible-playbook -i inventory.yml playbook.yml

# Expected: Cluster created successfully
# Verify
kubectl get nodes

# Expected: 1 server + 2 agents (3 nodes total)
```

### Test 7.2: Build and Push Images

```bash
cd /home/moersener/Hobby/all-chat/deployments/ansible

# Build all images
./build-and-push.sh

# Verify images in registry
curl http://localhost:5000/v2/_catalog | jq

# Expected: List of all allchat services
```

### Test 7.3: Deploy to k3d

```bash
# Apply manifests
kubectl apply -f ../k8s/base/ -n allchat --recursive

# Wait for deployments
kubectl wait --for=condition=Available deployment --all -n allchat --timeout=600s

# Check pods
kubectl get pods -n allchat

# Expected: All pods Running (1/1)
```

### Test 7.4: Run Verification Script

```bash
cd /home/moersener/Hobby/all-chat/deployments/ansible

# Run verification
./verify-deployment.sh

# Expected: All checks pass
```

### Test 7.5: Port Forward and Test

```bash
# Start port forwarding
cd /home/moersener/Hobby/all-chat/deployments/ansible
./port-forward.sh &

# Wait a moment
sleep 5

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8086/status | jq
curl http://localhost:8088/status | jq

# Expected: All responding correctly
```

### Test 7.6: Test in Kubernetes

```bash
# Run integration tests
cd /home/moersener/Hobby/all-chat/deployments/ansible
./test-integration.sh

# Expected: All integration tests pass
```

### Test 7.7: Test Scaling

```bash
# Scale YouTube Listener
kubectl scale deployment youtube-listener -n allchat --replicas=3

# Wait for pods
kubectl wait --for=condition=Ready pod -l app=youtube-listener -n allchat --timeout=120s

# Check HPA
kubectl get hpa -n allchat

# Scale Source Manager
kubectl scale deployment source-manager -n allchat --replicas=3

# Check leader election distribution
curl http://localhost:8088/status | jq '.leadership'

# Expected: Different instances can be leaders for different streams

# Scale back
kubectl scale deployment youtube-listener -n allchat --replicas=1
kubectl scale deployment source-manager -n allchat --replicas=1
```

### Test 7.8: Test CNPG Database (if using CNPG)

```bash
# Check cluster status
kubectl get cluster -n allchat

# Expected: STATUS = "Cluster in healthy state"

# Test connection
kubectl port-forward -n allchat allchat-cluster-1 5432:5432 &

PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c "\dt"

# Expected: All tables present
```

---

## Verification Checklist

### Infrastructure Verification

- [ ] PostgreSQL running and accepting connections
- [ ] Redis running and responding to PING
- [ ] All migrations applied successfully
- [ ] Redis consumer group created (`chat:raw` → `message-processor`)
- [ ] All tables exist in database
- [ ] Supported platforms data loaded

### Service Health Verification

- [ ] Auth Service: `/health/live` returns 200
- [ ] Auth Service: `/health/ready` returns 200
- [ ] Overlay Manager: `/health/live` returns 200
- [ ] Overlay Manager: `/health/ready` returns 200
- [ ] Emote Service: `/health/live` returns 200
- [ ] Emote Service: `/health/ready` returns 200
- [ ] API Gateway: `/health` returns 200
- [ ] Twitch Listener: `/health/live` returns 200
- [ ] Twitch Listener: `/health/ready` returns 200
- [ ] YouTube Listener: `/health/live` returns 200
- [ ] YouTube Listener: `/health/ready` returns 200
- [ ] Message Processor: `/health/live` returns 200
- [ ] Message Processor: `/health/ready` returns 200
- [ ] Source Manager: `/health/live` returns 200
- [ ] Source Manager: `/health/ready` returns 200

### Functional Verification

#### Twitch Flow
- [ ] Twitch OAuth login works
- [ ] JWT token generated successfully
- [ ] Can create overlay
- [ ] Can add Twitch source to overlay
- [ ] Twitch Listener JOINs channel (check `/status`)
- [ ] Messages published to Redis Streams
- [ ] Message Processor normalizes messages
- [ ] Messages enriched with emotes
- [ ] Messages published to Pub/Sub
- [ ] WebSocket receives messages
- [ ] Latency < 500ms (Twitch IRC → Browser)

#### YouTube Flow
- [ ] YouTube OAuth completed (token in database)
- [ ] Can add YouTube source to overlay
- [ ] YouTube Listener detects live stream
- [ ] YouTube Listener starts polling
- [ ] Messages published to Redis Streams
- [ ] Message Processor normalizes with YouTube normalizer
- [ ] YouTube badges extracted correctly
- [ ] Super Chat metadata handled
- [ ] Messages published to Pub/Sub
- [ ] WebSocket receives messages
- [ ] Latency < 5s (API poll interval dependent)

#### Multi-Platform Flow
- [ ] Can create overlay with both Twitch and YouTube sources
- [ ] WebSocket receives messages from both platforms
- [ ] Messages correctly tagged with platform
- [ ] Messages interleaved by timestamp
- [ ] Both platforms' emotes enriched
- [ ] No message loss or duplication

#### Leader Election
- [ ] Source Manager assigns leadership
- [ ] Only one YouTube Listener polls each stream
- [ ] Leadership renewed via heartbeat
- [ ] Failover works when leader dies
- [ ] Multiple instances don't duplicate polling

#### Quota Tracking
- [ ] Quota usage increments with each poll
- [ ] Quota percentage calculated correctly
- [ ] Warning logged at 80% usage
- [ ] Error logged at 90% usage
- [ ] Daily quota resets at midnight

---

## Troubleshooting

### Issue: Service Won't Start

**Symptoms**: Service crashes immediately

**Diagnosis:**
```bash
# Check logs
docker-compose logs <service-name>

# Common errors:
# - "Failed to connect to database" → Check PostgreSQL is running
# - "Failed to connect to Redis" → Check Redis is running
# - "Missing environment variable" → Check .env file
# - "Invalid OAuth credentials" → Check Twitch/YouTube credentials
```

**Solutions:**
- Verify `.env` file has all required variables
- Check database and Redis are running: `docker-compose ps`
- Apply migrations if needed
- Verify OAuth credentials are correct

### Issue: Twitch Listener Not Joining Channels

**Symptoms**: `/status` shows empty channels array

**Diagnosis:**
```bash
# Check database for active sources
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c \
  "SELECT * FROM overlay_chat_sources WHERE platform='twitch';"

# Check Twitch Listener logs
docker-compose logs twitch-listener | grep -i join
```

**Solutions:**
- Ensure overlay has `is_active=true`
- Ensure overlay_chat_source exists for Twitch
- Wait 30 seconds for sync cycle
- Restart Twitch Listener: `docker-compose restart twitch-listener`

### Issue: YouTube Listener Not Polling

**Symptoms**: No active streams in `/status`

**Diagnosis:**
```bash
# Check if OAuth token exists
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c \
  "SELECT channel_id, expiry FROM youtube_oauth_tokens;"

# Check if channel is actually live
# Use YouTube Studio or check manually

# Check YouTube Listener logs
docker-compose logs youtube-listener | grep -i "live stream"
```

**Solutions:**
- Ensure YouTube OAuth token is in database
- Ensure YouTube channel is actually live streaming
- Check token hasn't expired
- Verify YouTube API credentials are correct
- Check quota hasn't been exceeded

### Issue: WebSocket Not Receiving Messages

**Symptoms**: WebSocket connects but no messages appear

**Diagnosis:**
```bash
# Check if messages are in Redis Streams
redis-cli -h localhost XLEN chat:raw
# Should be > 0 if messages are flowing

# Check if consumer group is consuming
redis-cli -h localhost XPENDING chat:raw message-processor

# Check Message Processor logs
docker-compose logs message-processor | grep -i "published"

# Check Redis Pub/Sub
redis-cli -h localhost
> SUBSCRIBE overlay:your-overlay-id
# Send a test message and see if it appears
```

**Solutions:**
- Verify overlay ID is correct
- Check JWT token is valid
- Ensure overlay has active sources
- Check Message Processor is running
- Verify Redis Pub/Sub is working

### Issue: Leader Election Not Working

**Symptoms**: Multiple YouTube Listeners polling same stream

**Diagnosis:**
```bash
# Check Redis locks
redis-cli -h localhost KEYS "leader:*"

# Check each lock value
redis-cli -h localhost GET leader:youtube:stream123

# Check TTL
redis-cli -h localhost TTL leader:youtube:stream123

# Check Source Manager logs
docker-compose logs source-manager | grep -i leader
```

**Solutions:**
- Ensure Source Manager is running
- Verify Redis connection is stable
- Check lock TTL is being renewed
- Restart Source Manager if locks are stale

### Issue: High Quota Usage

**Symptoms**: Quota at 80%+ rapidly

**Diagnosis:**
```bash
# Check current usage
curl http://localhost:8086/status | jq '.quota'

# Check number of active streams
curl http://localhost:8086/status | jq '.streams.active_count'

# Calculate: active_streams * polls_per_hour * 5 units
# Example: 10 streams * (3600/2) * 5 = 9,000 units/hour
```

**Solutions:**
- Reduce number of YouTube sources
- Increase polling interval (adjust in code)
- Request quota increase from Google
- Use multiple API projects to distribute quota

### Issue: Tests Failing

**Symptoms**: `go test` shows failures

**Common Test Failures:**

1. **Database connection errors in tests:**
   ```bash
   # Skip integration tests
   go test -short ./...
   ```

2. **Redis connection errors in tests:**
   ```bash
   # Tests use miniredis (mock), should not need real Redis
   # If failing, check test imports miniredis correctly
   ```

3. **Import errors:**
   ```bash
   # Run go mod tidy
   cd services/youtube-listener
   go mod tidy

   cd ../source-manager
   go mod tidy
   ```

---

## Advanced Testing Scenarios

### Scenario A: Failover Testing

**Test Twitch Listener Reconnection:**
```bash
# Start Twitch Listener
docker-compose up -d twitch-listener

# Monitor logs
docker-compose logs -f twitch-listener

# Simulate IRC disconnect
docker-compose exec twitch-listener killall -USR1 <process-name>
# Or restart: docker-compose restart twitch-listener

# Expected: Automatic reconnection, re-JOIN all channels
```

**Test Message Processor Failover:**
```bash
# Stop Message Processor
docker-compose stop message-processor

# Send messages (will queue in Redis Streams)
# Send 10+ messages in Twitch chat

# Check stream backlog
redis-cli -h localhost XLEN chat:raw
# Should show queued messages

# Restart Message Processor
docker-compose start message-processor

# Expected: Processes backlog, messages caught up
```

### Scenario B: Database Failover (CNPG)

**Only applicable in Kubernetes with CNPG:**

```bash
# Delete primary pod
kubectl delete pod allchat-cluster-1 -n allchat

# Watch failover
kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster -w

# Expected:
# - New primary elected
# - Services reconnect automatically
# - No data loss
```

### Scenario C: Redis Persistence Test

```bash
# Publish messages
redis-cli -h localhost XADD chat:raw "*" message_id "test" platform "twitch" text "test"

# Restart Redis
docker-compose restart redis

# Wait for Redis to come back
sleep 5

# Check message still exists
redis-cli -h localhost XLEN chat:raw

# Expected: Messages persisted (appendonly enabled)
```

### Scenario D: Stress Test

**Simulate Heavy Load:**

```bash
# Start all services
docker-compose up -d

# Create 50 overlays with different Twitch channels
TOKEN="your-jwt-token"

for i in {1..50}; do
  CHANNEL="channel$i"
  OVERLAY_ID=$(curl -s -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Stress Test $i\"}" \
    http://localhost:8080/api/v1/overlays | jq -r '.id')

  curl -s -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"platform\":\"twitch\",\"channel_id\":\"$CHANNEL\"}" \
    http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources > /dev/null
done

# Check Twitch Listener
curl http://localhost:8085/status | jq '.irc.channels | length'
# Expected: 50 channels

# Monitor resource usage
docker stats

# Expected:
# - Message Processor: CPU < 50%, Memory < 500MB
# - Twitch Listener: CPU < 30%, Memory < 200MB
# - Other services: Minimal resource usage
```

---

## Testing Checklist Summary

### Pre-Testing Setup
- [ ] All prerequisites installed
- [ ] `.env` file created and filled with real credentials
- [ ] Twitch OAuth app created
- [ ] YouTube OAuth app created
- [ ] Twitch bot account OAuth token obtained

### Unit Tests
- [ ] All shared package tests pass
- [ ] Auth Service tests pass (48 tests)
- [ ] Overlay Manager tests pass (48 tests)
- [ ] Emote Service tests pass (62 tests)
- [ ] API Gateway tests pass (17 tests)
- [ ] Twitch Listener tests pass (22 tests)
- [ ] Message Processor tests pass (8+ tests)
- [ ] YouTube Listener tests pass
- [ ] Source Manager tests pass

### Integration Tests
- [ ] PostgreSQL accessible
- [ ] Redis accessible
- [ ] Migrations applied successfully
- [ ] Redis consumer group created
- [ ] All services can connect to infrastructure

### Service Tests
- [ ] All 8 services start successfully
- [ ] All health checks pass
- [ ] Twitch OAuth flow works
- [ ] Overlay CRUD operations work
- [ ] Twitch source can be added
- [ ] YouTube source can be added
- [ ] Emote caching works

### End-to-End Tests
- [ ] Twitch messages flow to WebSocket
- [ ] YouTube messages flow to WebSocket (if live stream available)
- [ ] Multi-platform overlay works
- [ ] Messages from both platforms appear correctly
- [ ] Emote enrichment works for both platforms
- [ ] Latency acceptable (Twitch <500ms, YouTube <5s)

### Advanced Tests
- [ ] Leader election prevents duplicate polling
- [ ] Quota tracking increments correctly
- [ ] Failover scenarios work
- [ ] Load test with 10+ overlays passes
- [ ] Multiple WebSocket connections work
- [ ] Redis persistence works

### Kubernetes Tests (Optional)
- [ ] k3d cluster created successfully
- [ ] All images built and pushed
- [ ] All pods running in k3d
- [ ] Port forwarding works
- [ ] Integration tests pass in k3d
- [ ] Scaling works
- [ ] CNPG cluster healthy (if applicable)

---

## Expected Test Results

### Unit Tests
- **Total Tests**: 205+ (existing) + 43 (new) = **248+ tests**
- **Coverage**: ~88% overall
- **Time**: 2-5 minutes
- **Status**: ✅ All passing

### Integration Tests
- **Database Connection**: ✅ Connected
- **Redis Connection**: ✅ Connected
- **Migrations**: ✅ Applied
- **Time**: 1-2 minutes

### Service Tests
- **All Services**: ✅ Started
- **Health Checks**: ✅ 8/8 passing
- **Time**: 5-10 minutes

### End-to-End Tests
- **Twitch Flow**: ✅ Working
- **YouTube Flow**: ✅ Working (with live stream)
- **Multi-Platform**: ✅ Working
- **Latency**: ✅ Acceptable
- **Time**: 10-20 minutes

### Load Tests
- **10 Overlays**: ✅ Handles well
- **50 Overlays**: ✅ Handles with increased resources
- **1000 Messages/s**: ✅ Processes without lag
- **Time**: 10-15 minutes

### Kubernetes Tests
- **Cluster Setup**: ✅ Working
- **All Pods**: ✅ Running
- **Scaling**: ✅ Working
- **Time**: 15-20 minutes

---

## Quick Test Script

Save this as `/tmp/quick-test-allchat.sh`:

```bash
#!/bin/bash
set -e

echo "========================================="
echo "All-Chat Quick Test Suite"
echo "========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0

cd /home/moersener/Hobby/all-chat

echo "1. Running unit tests..."
if go test -short ./... > /tmp/test-output.log 2>&1; then
  echo -e "${GREEN}✓${NC} Unit tests passed"
else
  echo -e "${RED}✗${NC} Unit tests failed (see /tmp/test-output.log)"
  FAILED=1
fi

echo ""
echo "2. Starting infrastructure..."
cd deployments
docker-compose up -d postgres redis
sleep 10

echo ""
echo "3. Checking infrastructure..."
if docker-compose exec postgres pg_isready -U allchat > /dev/null 2>&1; then
  echo -e "${GREEN}✓${NC} PostgreSQL ready"
else
  echo -e "${RED}✗${NC} PostgreSQL not ready"
  FAILED=1
fi

if docker-compose exec redis redis-cli ping > /dev/null 2>&1; then
  echo -e "${GREEN}✓${NC} Redis ready"
else
  echo -e "${RED}✗${NC} Redis not ready"
  FAILED=1
fi

echo ""
echo "4. Starting all services..."
docker-compose up -d
sleep 20

echo ""
echo "5. Checking health endpoints..."
services=(
  "8080:API Gateway"
  "8081:Auth Service"
  "8082:Overlay Manager"
  "8083:Emote Service"
  "8085:Twitch Listener"
  "8086:YouTube Listener"
  "8087:Message Processor"
  "8088:Source Manager"
)

for svc in "${services[@]}"; do
  port=$(echo "$svc" | cut -d':' -f1)
  name=$(echo "$svc" | cut -d':' -f2)

  if curl -s -f "http://localhost:$port/health/live" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} $name"
  else
    echo -e "${RED}✗${NC} $name - FAILED"
    FAILED=1
  fi
done

echo ""
echo "========================================="
if [ $FAILED -eq 0 ]; then
  echo -e "${GREEN}All tests passed!${NC}"
  echo ""
  echo "Next steps:"
  echo "  - Test Twitch OAuth: curl http://localhost:8080/api/v1/auth/twitch/login"
  echo "  - Create overlay: See docs/TESTING_COMPREHENSIVE.md"
  echo "  - Test WebSocket: websocat ws://localhost:8080/ws/overlay/{id}?token={jwt}"
else
  echo -e "${RED}Some tests failed${NC}"
  echo ""
  echo "Check logs: docker-compose logs"
  echo "See /tmp/test-output.log for test failures"
fi
echo "========================================="
```

**Usage:**
```bash
chmod +x /tmp/quick-test-allchat.sh
/tmp/quick-test-allchat.sh
```

---

## Complete Test Sequence

### Full Test Run (Complete Coverage)

```bash
# 1. Unit Tests (5 minutes)
cd /home/moersener/Hobby/all-chat
go test ./... -v -cover

# 2. Start Infrastructure (2 minutes)
cd deployments
docker-compose up -d postgres redis
sleep 10

# 3. Apply Migrations (1 minute)
cd ..
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/001_initial_schema.sql
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/003_youtube_support.sql

# 4. Create Consumer Group (30 seconds)
redis-cli -h localhost XGROUP CREATE chat:raw message-processor $ MKSTREAM

# 5. Start All Services (3 minutes)
cd deployments
docker-compose up -d
sleep 30

# 6. Verify Health (1 minute)
/tmp/quick-test-allchat.sh

# 7. Test Twitch Flow (5 minutes)
# - Complete OAuth
# - Create overlay
# - Add Twitch source
# - Connect WebSocket
# - Send message in Twitch chat
# - Verify message appears

# 8. Test YouTube Flow (10 minutes)
# - Set up OAuth token
# - Create overlay
# - Add YouTube source
# - Wait for stream detection
# - Send message in YouTube chat
# - Verify message appears

# 9. Test Multi-Platform (5 minutes)
# - Create overlay with both sources
# - Verify messages from both platforms

# 10. Load Test (10 minutes)
# - Create 10+ overlays
# - Monitor resource usage
# - Verify performance

# Total Time: ~45 minutes
```

---

## Post-Testing Cleanup

```bash
# Stop all services
cd /home/moersener/Hobby/all-chat/deployments
docker-compose down

# Remove volumes (optional - deletes data)
docker-compose down -v

# Clean Redis data
redis-cli -h localhost FLUSHALL

# Clean database
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat << EOF
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
EOF

# Remove test logs
rm /tmp/test-*.log
rm /tmp/ws-*.log
```

---

## Next Steps After Testing

Once all tests pass:

1. **Document Results**: Note any issues found
2. **Performance Tuning**: Optimize any slow components
3. **Security Review**: Check for vulnerabilities
4. **Production Deployment**: Use Ansible playbook for Caesar cluster
5. **Monitoring Setup**: Configure Grafana dashboards
6. **User Acceptance Testing**: Get feedback from real users

---

**Last Updated**: November 13, 2025
**Status**: Ready for comprehensive testing
**Estimated Time**: 2-4 hours for complete test suite
