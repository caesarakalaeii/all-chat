# Frontend Development - Quick Start

**One-command setup for fast iteration on the frontend.**

## 🚀 30-Second Setup

```bash
# 1. Start services
make frontend-dev

# 2. Seed test data
make frontend-seed

# 3. Verify everything works
make frontend-verify
```

**Done!** Backend is ready.

## 🎯 Start Developing

### Terminal 1: Message Generator
```bash
make frontend-messages
```
This generates realistic chat messages every 3 seconds.

### Terminal 2: Frontend
```bash
cd frontend
npm install  # First time only
npm run dev
```

Visit: **http://localhost:3000**

## 📊 What You Get

- **Test Overlay**: `00000000-0000-0000-0000-000000000002`
- **Test User**: `teststreamer`
- **Chat Sources**: Twitch + YouTube configured
- **Mock Messages**: Continuous stream of test messages
- **WebSocket**: `ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002`

## 🔧 Common Commands

```bash
# View logs
docker-compose -f docker-compose.frontend.yml logs -f

# Restart a service
docker-compose -f docker-compose.frontend.yml restart api-gateway

# Stop everything
make frontend-down

# Complete reset
make frontend-reset
```

## 🧪 Testing Tools

### WebSocket Test (Node.js)
```bash
cd scripts
npm install  # First time only
node test-websocket.js
```

### WebSocket Test (wscat)
```bash
npm install -g wscat
wscat -c ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002
```

### Direct API Test
```bash
# Send a message via the message-processor mock endpoint
curl -X POST http://localhost:8087/internal/mock-messages \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: dev-frontend-key" \
  -d '{
    "overlay_id": "00000000-0000-0000-0000-000000000002",
    "platform": "twitch",
    "username": "APIUser",
    "text": "Hello from API!"
  }'
```

## 🎨 LLM Agent Workflow

**Perfect for iterative frontend development:**

1. **One-time setup** (do once per session):
   ```bash
   make frontend-dev
   make frontend-seed
   ```

2. **Keep running** (in separate terminals):
   ```bash
   make frontend-messages  # Terminal 1
   cd frontend && npm run dev  # Terminal 2
   ```

3. **Iterate**:
   - LLM modifies frontend code
   - Next.js hot-reloads automatically
   - See changes immediately
   - No backend restarts needed

4. **Monitor** (optional):
   ```bash
   docker-compose -f docker-compose.frontend.yml logs -f api-gateway
   ```

## 🔍 Debugging

### Check Service Health
```bash
curl http://localhost:8080/health/live  # API Gateway
curl http://localhost:8082/health/live  # Overlay Manager
curl http://localhost:8087/health/live  # Message Processor
```

### Monitor Redis
```bash
# Watch messages being published
redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"

# View raw stream
redis-cli XREAD COUNT 10 STREAMS chat:raw 0-0
```

### Check Database
```bash
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat

# View overlays
SELECT id, name, is_active FROM overlays;

# View sources
SELECT platform, channel_name, is_active FROM overlay_chat_sources;
```

## ⚡ Performance Tips

### Faster Message Generation
```bash
MESSAGE_INTERVAL=1 ./scripts/generate-test-messages.sh  # 1 msg/second
MESSAGE_INTERVAL=0.5 ./scripts/generate-test-messages.sh  # 2 msg/second
```

### Burst Testing
```bash
MESSAGE_COUNT=100 MESSAGE_INTERVAL=0.1 ./scripts/generate-test-messages.sh
```

### Custom Messages
Edit `scripts/generate-test-messages.sh` and modify:
- `MESSAGES` array - different message texts
- `USERNAMES` array - different usernames
- `PLATFORMS` array - add/remove platforms

## 🛑 Cleanup

```bash
# Stop services (keep data)
make frontend-down

# Stop and remove all data
docker-compose -f docker-compose.frontend.yml down -v

# Remove Docker images (free disk space)
docker-compose -f docker-compose.frontend.yml down --rmi all -v
```

## 📦 What's Running

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Pub/Sub + Streams |
| API Gateway | 8080 | WebSocket + HTTP |
| Overlay Manager | 8082 | CRUD operations |
| Message Processor | 8087 | Message normalization |
| Share Service | 8090 | Shareable overlay links |

**Not running** (to save resources):
- Auth Service
- Emote Service
- Platform Listeners
- Token Refresh Service
- Source Manager

## 💡 Tips

1. **Auto-restart on crashes**: Services use `restart: unless-stopped`
2. **Fast startup**: ~30 seconds from `make frontend-dev` to ready
3. **No secrets needed**: All dev defaults work out-of-the-box
4. **Isolated data**: Uses separate volumes from main docker-compose
5. **Parallel safe**: Can run alongside main stack on different ports

## 📚 Full Documentation

For complete details, see:
- [FRONTEND_DEV_SETUP.md](./FRONTEND_DEV_SETUP.md) - Complete guide
- [GETTING_STARTED.md](./GETTING_STARTED.md) - Full project onboarding

## ❓ Troubleshooting

**Services won't start**
```bash
# Check if ports are in use
lsof -i :8080,8082,8087,5432,6379

# Stop full stack if needed
cd deployments && docker-compose down
```

**No messages appearing**
```bash
# Verify message processor
curl http://localhost:8087/health/live

# Check Redis subscription
redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"
```

**Database errors**
```bash
# Verify PostgreSQL is ready
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -c '\dt'

# Re-seed if needed
make frontend-seed
```
