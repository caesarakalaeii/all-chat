# Frontend Development Setup

Quick setup guide for frontend development with minimal backend services.

## 🚀 Quick Start

```bash
# 1. Start backend services
docker-compose -f docker-compose.frontend.yml up -d

# 2. Wait for services to be healthy (about 30 seconds)
docker-compose -f docker-compose.frontend.yml ps

# 3. Seed test data
./scripts/seed-test-data.sh

# 4. Start message generator (in separate terminal)
./scripts/generate-test-messages.sh

# 5. Start frontend (in separate terminal)
cd frontend
npm install
npm run dev
```

Visit: http://localhost:3000

## 📦 What's Included

**Minimal Backend Services:**
- ✅ PostgreSQL (database)
- ✅ Redis (pub/sub and streams)
- ✅ API Gateway (WebSocket + HTTP)
- ✅ Overlay Manager (CRUD operations)
- ✅ Message Processor (message normalization)
- ✅ Share Service (shareable overlay links, `:8090`)

**Not Included (for faster startup):**
- ❌ Auth Service (auth disabled)
- ❌ Emote Service (emotes disabled)
- ❌ Platform Listeners (Twitch, YouTube, etc.)
- ❌ Token Refresh Service
- ❌ Source Manager

## 🧪 Test Data

The seed script creates:

| Resource | ID | Details |
|----------|----|---------|
| **User** | `00000000-0000-0000-0000-000000000001` | Username: `teststreamer` |
| **Overlay** | `00000000-0000-0000-0000-000000000002` | Name: "Frontend Test Overlay" |
| **Twitch Source** | `00000000-0000-0000-0000-000000000003` | Channel: `teststreamer` |
| **YouTube Source** | `00000000-0000-0000-0000-000000000004` | Channel: `UCtest12345` |

### WebSocket URL

```
ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002
```

## 🔧 Configuration

### Environment Variables

No secrets required! All services use development defaults:

```bash
# Database
DB_PASSWORD=dev_password_123
# JWT key chain (shared/auth) — dev values are hardcoded in
# docker-compose.frontend.yml (JWT_SECRET, JWT_SECRET_V1, SERVICE_JWT_SECRET_V1;
# secrets must be at least 32 bytes)

# API Keys
MESSAGE_PROCESSOR_API_KEY=dev-frontend-key

# CORS
CORS_ORIGIN=http://localhost:3000  # wildcard is rejected with cookie auth
```

### Message Generator Options

```bash
# Generate 10 messages then stop
MESSAGE_COUNT=10 ./scripts/generate-test-messages.sh

# Generate messages every 5 seconds
MESSAGE_INTERVAL=5 ./scripts/generate-test-messages.sh

# Use custom overlay ID
TEST_OVERLAY_ID=your-overlay-id ./scripts/generate-test-messages.sh
```

## 📊 Monitoring

### View Logs

```bash
# All services
docker-compose -f docker-compose.frontend.yml logs -f

# Specific service
docker-compose -f docker-compose.frontend.yml logs -f api-gateway
docker-compose -f docker-compose.frontend.yml logs -f message-processor
```

### Check Health

```bash
# API Gateway
curl http://localhost:8080/health/live

# Overlay Manager
curl http://localhost:8082/health/live

# Message Processor
curl http://localhost:8087/health/live
```

### Redis Pub/Sub

```bash
# Monitor messages on the overlay channel
redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"
```

### Database Access

```bash
# Connect to PostgreSQL
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat

# View overlays
SELECT id, name, is_active FROM overlays;

# View chat sources
SELECT id, overlay_id, platform, channel_name FROM overlay_chat_sources;
```

## 🔄 Reset Everything

```bash
# Stop and remove all containers + volumes
docker-compose -f docker-compose.frontend.yml down -v

# Start fresh
docker-compose -f docker-compose.frontend.yml up -d
./scripts/seed-test-data.sh
```

## 🐛 Troubleshooting

### Services won't start

```bash
# Check if ports are in use
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis
lsof -i :8080  # API Gateway
lsof -i :8082  # Overlay Manager
lsof -i :8087  # Message Processor

# Stop full stack if running
cd deployments && docker-compose down
```

### Messages not appearing

1. **Check message processor is running:**
   ```bash
   curl http://localhost:8087/health/live
   ```

2. **Verify Redis pub/sub:**
   ```bash
   redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"
   ```

3. **Check WebSocket connection in browser console:**
   ```javascript
   const ws = new WebSocket('ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002');
   ws.onmessage = (event) => console.log('Message:', event.data);
   ```

### Database connection errors

```bash
# Wait for PostgreSQL to be ready
docker-compose -f docker-compose.frontend.yml logs postgres

# Manually test connection
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -c '\dt'
```

## 🎯 LLM Agent Workflow

For iterative frontend development with LLM agents:

1. **Start services once:**
   ```bash
   docker-compose -f docker-compose.frontend.yml up -d
   ./scripts/seed-test-data.sh
   ```

2. **Keep message generator running:**
   ```bash
   ./scripts/generate-test-messages.sh
   ```

3. **Iterate on frontend:**
   - LLM agent modifies frontend code
   - Frontend auto-reloads (Next.js hot reload)
   - View changes in browser
   - Repeat

4. **No backend restarts needed** unless:
   - Changing API Gateway code
   - Modifying database schema
   - Updating message processor logic

## 📝 API Endpoints

### Overlay Manager (`:8082`)

The overlay-manager mounts routes at the root path (no `/api/overlays` prefix). Most endpoints require a JWT (`middleware.JWTAuthWithRevocation`). For dev, the easier path is to call through the API gateway (`/api/v1/overlays/...` on port 8080) or hit the public read-only endpoints directly:

```bash
# Public overlay config (no auth)
curl http://localhost:8082/public/00000000-0000-0000-0000-000000000002/config

# Protected (requires JWT in Authorization: Bearer ...)
# GET /:id              -> get overlay
# GET /:id/sources      -> list chat sources
# POST /:id/mock-messages -> send mock message via overlay-manager (also JWT-protected)
```

### Message Processor (`:8087`)

```bash
# Send mock message via the message-processor internal endpoint
curl -X POST http://localhost:8087/internal/mock-messages \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: dev-frontend-key" \
  -d '{
    "overlay_id": "00000000-0000-0000-0000-000000000002",
    "platform": "twitch",
    "username": "TestUser",
    "text": "Hello from API!"
  }'
```

### API Gateway (`:8080`)

```bash
# WebSocket (use browser or wscat)
wscat -c ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002

# Health check
curl http://localhost:8080/health/live
```

## 🎨 Frontend Testing Tips

1. **Test different message volumes:**
   ```bash
   MESSAGE_INTERVAL=0.5 ./scripts/generate-test-messages.sh  # Fast
   MESSAGE_INTERVAL=10 ./scripts/generate-test-messages.sh   # Slow
   ```

2. **Test reconnection:**
   ```bash
   # Restart API Gateway mid-stream
   docker-compose -f docker-compose.frontend.yml restart api-gateway
   ```

3. **Test with multiple overlays:**
   - Edit `seed-test-data.sh` to create more overlays
   - Run multiple message generators with different `TEST_OVERLAY_ID`

4. **Inspect message flow:**
   ```bash
   # Watch Redis streams
   redis-cli XREAD COUNT 10 STREAMS chat:raw 0-0

   # Monitor pub/sub
   redis-cli SUBSCRIBE "overlay:*"
   ```

## 🚧 Limitations

This setup is **for frontend development only**:

- ⚠️ No authentication (auth service disabled)
- ⚠️ No emotes (emote service disabled)
- ⚠️ No real platform connections
- ⚠️ Simple mock data only
- ⚠️ Not suitable for production

For full testing, use the main docker-compose setup:
```bash
cd deployments
docker-compose up
```
