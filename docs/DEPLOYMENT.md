# All-Chat Deployment Guide

This guide covers deploying your own instance of All-Chat. If you just want to use the service, visit **[allch.at](https://allch.at)** instead.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Local Development Setup](#local-development-setup)
- [Docker Compose Deployment](#docker-compose-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Environment Configuration](#environment-configuration)
- [Database Migrations](#database-migrations)
- [Monitoring & Health Checks](#monitoring--health-checks)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go 1.25+** - Backend services
- **Docker & Docker Compose** - Containerization
- **PostgreSQL 16** - Database (or use Docker)
- **Redis 7** - Cache and messaging (or use Docker)
- **Node.js 22+** - Frontend (React + Next.js)
- **kubectl** - For Kubernetes deployments (optional)

### Required API Credentials

#### Twitch Developer Account

1. Go to https://dev.twitch.tv/console/apps
2. Create a new application
3. Set OAuth Redirect URL to: `<FRONTEND_URL>/api/v1/auth/twitch/callback` (e.g., `http://localhost:8080/api/v1/auth/twitch/callback` for Docker Compose)
4. Copy **Client ID** and **Client Secret**
5. Get IRC OAuth token from https://twitchapps.com/tmi/

#### YouTube API (Optional)

1. Go to https://console.developers.google.com/
2. Create a new project
3. Enable YouTube Data API v3
4. Create credentials (OAuth 2.0 Client ID and API Key)
5. Copy credentials

## Local Development Setup

### 1. Clone Repository

```bash
git clone https://github.com/caesar/all-chat.git
cd all-chat
```

### 2. Configure Environment

```bash
# Copy environment template
cp .env.example .env

# Edit with your credentials
nano .env
```

See [Environment Configuration](#environment-configuration) for all variables.

### 3. Install Dependencies

```bash
# Backend dependencies
make deps

# Frontend dependencies
cd frontend
npm install
cd ..
```

### 4. Start Infrastructure

```bash
# Start PostgreSQL and Redis with Docker
docker-compose up postgres redis -d

# Or start all services (recommended)
make docker-up
```

### 5. Run Migrations

```bash
make migrate
```

### 6. Build Services

```bash
# Build all services
make build

# Or build individually
make build-api-gateway
make build-auth
make build-overlay
# ... etc
```

### 7. Run Services

```bash
# Run all services (each in a separate terminal)
make run-gateway      # Terminal 1
make run-auth         # Terminal 2
make run-overlay      # Terminal 3
make run-emote        # Terminal 4
make run-chat         # Terminal 5
# ... etc

# Or use Docker Compose (easier)
make docker-up
```

### 8. Start Frontend (Development)

```bash
cd frontend
npm run dev
```

Visit http://localhost:3000

## Docker Compose Deployment

The easiest way to deploy All-Chat for production or development.

### Quick Start

```bash
# Start all services
make docker-up

# View logs
make docker-logs

# Restart services
make docker-restart

# Stop services
make docker-down
```

### Services Available

- **API Gateway**: http://localhost:8080
- **Twitch Listener**: http://localhost:8085
- **YouTube Listener**: http://localhost:8086
- **Message Processor**: http://localhost:8087
- **Source Manager**: http://localhost:8088
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **Frontend**: http://localhost:3000 (if configured)

### Custom Configuration

Edit `docker-compose.yml` to customize:
- Port mappings
- Resource limits
- Volume mounts
- Environment variables

## Kubernetes Deployment

For production deployments with high availability and scalability.

### 1. Create Namespace

```bash
kubectl apply -f deployments/k8s/namespace.yaml
```

### 2. Create Secrets

```bash
kubectl create secret generic app-secrets \
  --from-literal=database-password=YOUR_DB_PASSWORD \
  --from-literal=jwt-secret=YOUR_JWT_SECRET \
  --from-literal=twitch-client-id=YOUR_TWITCH_CLIENT_ID \
  --from-literal=twitch-client-secret=YOUR_TWITCH_CLIENT_SECRET \
  --from-literal=twitch-bot-username=YOUR_BOT_USERNAME \
  --from-literal=twitch-bot-oauth=YOUR_BOT_OAUTH \
  --from-literal=youtube-api-key=YOUR_YT_API_KEY \
  --from-literal=youtube-client-id=YOUR_YT_CLIENT_ID \
  --from-literal=youtube-client-secret=YOUR_YT_CLIENT_SECRET \
  -n all-chat
```

### 3. Apply ConfigMaps

```bash
kubectl apply -f deployments/k8s/configmaps/
```

### 4. Deploy Infrastructure

```bash
# PostgreSQL
kubectl apply -f deployments/k8s/postgres/

# Redis
kubectl apply -f deployments/k8s/redis/
```

### 5. Deploy Services

```bash
kubectl apply -f deployments/k8s/api-gateway/
kubectl apply -f deployments/k8s/twitch-listener/
kubectl apply -f deployments/k8s/youtube-listener/
kubectl apply -f deployments/k8s/message-processor/
kubectl apply -f deployments/k8s/source-manager/
```

### 6. Deploy Ingress

```bash
kubectl apply -f deployments/k8s/ingress/
```

### 7. Verify Deployment

```bash
# Check pod status
kubectl get pods -n all-chat

# Check services
kubectl get svc -n all-chat

# View logs
kubectl logs -f deployment/api-gateway -n all-chat
```

## Environment Configuration

### Required Variables

```bash
# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=your_secure_password
DATABASE_NAME=allchat

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=  # Optional

# JWT Secret (generate with: openssl rand -base64 32)
JWT_SECRET=your_jwt_secret_key_at_least_32_chars

# Twitch Bot (for IRC connection)
TWITCH_BOT_USERNAME=your_bot_username
TWITCH_BOT_OAUTH=oauth:your_bot_oauth_token

# Twitch OAuth (for user authentication)
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret

# YouTube API (optional)
YOUTUBE_API_KEY=your_youtube_api_key
YOUTUBE_CLIENT_ID=your_youtube_client_id
YOUTUBE_CLIENT_SECRET=your_youtube_client_secret

# Internal service auth
MESSAGE_PROCESSOR_API_KEY=generate_a_strong_shared_key
```

### Optional Variables

```bash
# Service Ports
API_GATEWAY_PORT=8080
TWITCH_LISTENER_PORT=8085
YOUTUBE_LISTENER_PORT=8086
MESSAGE_PROCESSOR_PORT=8087
MESSAGE_PROCESSOR_URL=http://localhost:8087
SOURCE_MANAGER_PORT=8088

# Frontend
NEXT_PUBLIC_API_URL=http://localhost:8080
FRONTEND_URL=https://your-domain.example  # Used to auto-generate OAuth redirect URLs

# Logging
LOG_LEVEL=info  # debug, info, warn, error

# Performance
MAX_MESSAGES_PER_OVERLAY=50
MESSAGE_DURATION_SECONDS=15
```

## Database Migrations

### Run Migrations

```bash
# Up (apply migrations)
make migrate

# Down (rollback)
make migrate-down

# Create new migration
make migrate-create NAME=add_new_table
```

### Manual Migration

```bash
psql postgresql://allchat:password@localhost:5432/allchat < migrations/001_initial_schema.sql
```

## Monitoring & Health Checks

### Health Check Endpoints

All services expose:

```bash
# Liveness probe (always returns 200 OK)
curl http://localhost:8080/health/live

# Readiness probe (checks DB + Redis)
curl http://localhost:8080/health/ready
```

### Kubernetes Probes

Services are configured with:
- **Liveness Probe**: Restarts container if unhealthy
- **Readiness Probe**: Removes from load balancer if not ready

### Logging

All services use structured logging (Zap):

```bash
# View service logs
kubectl logs -f deployment/api-gateway -n all-chat

# Docker Compose logs
docker-compose logs -f api-gateway

# Local development
# Logs output to stdout
```

### Metrics (TODO)

Prometheus metrics endpoint:
```bash
curl http://localhost:8080/metrics
```

## Service Architecture

```
┌─────────────────┐
│   API Gateway   │ ← Entry point, WebSocket server
│   (Port 8080)   │
└────────┬────────┘
         │
         ├─────────────────┬─────────────────┬──────────────────┐
         │                 │                 │                  │
┌────────▼────────┐ ┌──────▼──────┐ ┌───────▼────────┐ ┌──────▼──────────┐
│ Twitch Listener │ │   YouTube   │ │    Message     │ │     Source      │
│  (Port 8085)    │ │   Listener  │ │   Processor    │ │    Manager      │
│                 │ │ (Port 8086) │ │ (Port 8087)    │ │  (Port 8088)    │
└────────┬────────┘ └──────┬──────┘ └───────┬────────┘ └──────┬──────────┘
         │                 │                 │                  │
         └─────────────────┴─────────────────┴──────────────────┘
                                    │
                     ┌──────────────┴──────────────┐
                     │                             │
              ┌──────▼──────┐            ┌────────▼────────┐
              │  PostgreSQL │            │      Redis      │
              │             │            │  (Pub/Sub +     │
              │             │            │   Streams)      │
              └─────────────┘            └─────────────────┘
```

## Troubleshooting

### Services Not Starting

**Check dependencies:**
```bash
# Verify PostgreSQL
psql postgresql://allchat:password@localhost:5432/allchat

# Verify Redis
redis-cli -h localhost ping
```

**Check logs:**
```bash
# Docker Compose
docker-compose logs -f service-name

# Kubernetes
kubectl logs -f deployment/service-name -n all-chat
```

### Database Connection Errors

```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# Test connection
psql postgresql://allchat:password@localhost:5432/allchat -c "SELECT 1"

# Run migrations
make migrate
```

### Redis Connection Errors

```bash
# Check if Redis is running
docker-compose ps redis

# Test connection
redis-cli -h localhost ping

# Check Redis logs
docker-compose logs redis
```

### Twitch IRC Connection Failing

1. Verify bot OAuth token is valid: https://twitchapps.com/tmi/
2. Ensure `TWITCH_BOT_USERNAME` matches the token's account
3. Check token has `chat:read` scope
4. Test with: `curl -H "Authorization: OAuth YOUR_TOKEN" https://id.twitch.tv/oauth2/validate`

### YouTube API Quota Exceeded

YouTube API has a default quota of 10,000 units/day:
- Each live chat message fetch costs ~5 units
- Monitor usage in Google Cloud Console
- Request quota increase if needed

### WebSocket Connection Failing

```bash
# Test WebSocket connection
wscat -c ws://localhost:8080/ws/overlay/YOUR_OVERLAY_ID

# Check CORS settings
# Ensure frontend URL is allowed in API Gateway CORS config
```

### Build Errors

```bash
# Clean build cache
make clean

# Update dependencies
make deps

# Rebuild everything
make build
```

## Production Checklist

- [ ] Use strong JWT secret (32+ characters)
- [ ] Enable HTTPS/TLS (configure Ingress or reverse proxy)
- [ ] Set secure database passwords
- [ ] Configure CORS for your domain only
- [ ] Enable rate limiting
- [ ] Set up monitoring (Prometheus + Grafana)
- [ ] Configure log aggregation (ELK, Loki, etc.)
- [ ] Set up backups (PostgreSQL + Redis)
- [ ] Configure resource limits (CPU, memory)
- [ ] Set up auto-scaling (HPA in Kubernetes)
- [ ] Configure secrets management (Vault, Sealed Secrets)
- [ ] Set up CI/CD pipeline
- [ ] Configure domain and DNS
- [ ] Set up SSL certificates (Let's Encrypt)

## Scaling Considerations

### Horizontal Scaling

Services that can scale horizontally:
- ✅ API Gateway (multiple instances behind load balancer)
- ✅ Message Processor (consumer groups in Redis Streams)
- ⚠️ Twitch Listener (single instance per channel recommended)
- ⚠️ YouTube Listener (leader election required, only one active)
- ✅ Source Manager (stateless, multiple instances OK)

### Vertical Scaling

Increase resources for:
- PostgreSQL (more RAM for larger databases)
- Redis (more RAM for message buffers)
- Message Processor (more CPU for emote enrichment)

### Database Optimization

- Add indexes on frequently queried columns
- Partition large tables by time
- Archive old messages
- Use read replicas for heavy read workloads

## Additional Resources

- [CLAUDE.md](../CLAUDE.md) - Developer documentation
- [GETTING_STARTED.md](./GETTING_STARTED.md) - Navigation guide
- [API Documentation](./API.md) - API reference (TODO)
- [Architecture Docs](./architecture/) - Detailed architecture
- [Kubernetes Configs](../deployments/k8s/) - K8s manifests

## Support

For deployment issues:
1. Check logs first
2. Search existing GitHub issues
3. Open a new issue with:
   - Deployment method (Docker/K8s)
   - Error messages
   - Environment details
   - Steps to reproduce
