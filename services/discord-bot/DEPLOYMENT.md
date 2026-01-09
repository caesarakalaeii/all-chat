# Discord Bot Deployment Guide

Complete guide for deploying the Discord Quota Monitor bot in production.

## Kubernetes Deployment

### 1. Create Kubernetes Manifests

Create `deployments/k8s/base/discord-bot/` directory:

```bash
mkdir -p deployments/k8s/base/discord-bot
```

### 2. Create Secret

**discord-bot-secret.yaml**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: discord-bot-secrets
  namespace: all-chat
type: Opaque
stringData:
  discord-bot-token: "YOUR_DISCORD_BOT_TOKEN_HERE"
  discord-channel-id: "YOUR_DISCORD_CHANNEL_ID_HERE"
```

Apply the secret:
```bash
kubectl apply -f deployments/k8s/base/discord-bot/discord-bot-secret.yaml
```

### 3. Create Deployment

**deployment.yaml**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: discord-bot
  namespace: all-chat
  labels:
    app: discord-bot
    component: monitoring
spec:
  replicas: 1  # Only run 1 replica to avoid duplicate messages
  selector:
    matchLabels:
      app: discord-bot
  template:
    metadata:
      labels:
        app: discord-bot
    spec:
      containers:
      - name: discord-bot
        image: ghcr.io/your-org/all-chat-discord-bot:latest
        imagePullPolicy: Always
        env:
        - name: DISCORD_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: discord-bot-secrets
              key: discord-bot-token
        - name: DISCORD_CHANNEL_ID
          valueFrom:
            secretKeyRef:
              name: discord-bot-secrets
              key: discord-channel-id
        - name: REDIS_HOST
          value: "redis-service.all-chat.svc.cluster.local"
        - name: REDIS_PORT
          value: "6379"
        - name: YOUTUBE_LISTENER_URL
          value: "http://youtube-listener-service.all-chat.svc.cluster.local:8086"
        - name: STATUS_UPDATE_INTERVAL
          value: "3600000"
        resources:
          requests:
            memory: "128Mi"
            cpu: "50m"
          limits:
            memory: "256Mi"
            cpu: "100m"
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - "ps aux | grep node | grep -v grep"
          initialDelaySeconds: 30
          periodSeconds: 30
          failureThreshold: 3
      restartPolicy: Always
```

### 4. Apply Deployment

```bash
kubectl apply -f deployments/k8s/base/discord-bot/deployment.yaml
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n all-chat -l app=discord-bot

# View logs
kubectl logs -n all-chat -l app=discord-bot -f

# Should see:
# ✅ Connected to Redis
# ✅ Subscribed to quota:alerts channel
# ✅ Discord bot logged in
# ✅ Discord bot ready as QuotaMonitor#1234
```

---

## Docker Compose Deployment

Already configured in `deployments/docker-compose.yml`!

### 1. Set Environment Variables

Add to your `.env`:
```bash
DISCORD_BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN
DISCORD_CHANNEL_ID=YOUR_DISCORD_CHANNEL_ID
STATUS_UPDATE_INTERVAL=3600000
```

### 2. Build and Start

```bash
# Build the image
docker-compose build discord-bot

# Start the bot
docker-compose up -d discord-bot

# View logs
docker-compose logs -f discord-bot
```

---

## Standalone Docker Deployment

### 1. Build Image

```bash
cd services/discord-bot
docker build -t all-chat-discord-bot:latest .
```

### 2. Run Container

```bash
docker run -d \
  --name discord-quota-bot \
  --restart unless-stopped \
  -e DISCORD_BOT_TOKEN="YOUR_TOKEN" \
  -e DISCORD_CHANNEL_ID="YOUR_CHANNEL_ID" \
  -e REDIS_HOST="your-redis-host" \
  -e REDIS_PORT="6379" \
  -e YOUTUBE_LISTENER_URL="http://your-youtube-listener:8086" \
  -e STATUS_UPDATE_INTERVAL="3600000" \
  all-chat-discord-bot:latest
```

---

## Production Best Practices

### Security

1. **Never commit tokens to git**
   - Use Kubernetes secrets or environment variables
   - Rotate bot token periodically

2. **Restrict bot permissions**
   - Only grant "Send Messages" and "Embed Links"
   - Don't enable unnecessary intents

3. **Network isolation**
   - Bot only needs access to Redis and YouTube Listener
   - No external internet access required (except Discord API)

### Reliability

1. **Single replica only**
   - Running multiple replicas will cause duplicate messages
   - Use `replicas: 1` in Kubernetes

2. **Restart policy**
   - Set `restartPolicy: Always` or `restart: unless-stopped`
   - Bot will auto-reconnect on crashes

3. **Health checks**
   - Process-based liveness probe
   - Monitor logs for connection issues

### Monitoring

1. **Check logs regularly**
   ```bash
   # Kubernetes
   kubectl logs -n all-chat -l app=discord-bot --tail=100

   # Docker
   docker logs discord-quota-bot --tail=100
   ```

2. **Key log messages to watch for**
   - ✅ Connected to Redis
   - ✅ Subscribed to quota:alerts
   - ✅ Discord bot ready
   - ⚠️ Failed to fetch quota status
   - ❌ Redis connection error

3. **Set up alerts**
   - Monitor for repeated connection failures
   - Alert if bot is offline for > 5 minutes

### Resource Limits

Recommended limits:
- **Memory**: 128Mi request, 256Mi limit
- **CPU**: 50m request, 100m limit

The bot is very lightweight:
- ~50MB memory usage
- Minimal CPU usage (event-driven)

---

## GitHub Actions CI/CD

### 1. Build and Push Docker Image

**.github/workflows/discord-bot.yml**
```yaml
name: Build Discord Bot

on:
  push:
    branches: [main]
    paths:
      - 'services/discord-bot/**'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v4
        with:
          context: services/discord-bot
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/discord-bot:latest
            ghcr.io/${{ github.repository }}/discord-bot:${{ github.sha }}
```

### 2. Auto-deploy to Kubernetes

Add deployment step:
```yaml
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/discord-bot \
            discord-bot=ghcr.io/${{ github.repository }}/discord-bot:${{ github.sha }} \
            -n all-chat
```

---

## Scaling Considerations

### ⚠️ Do NOT scale horizontally

The Discord bot **should NOT be scaled** to multiple replicas because:
- Each replica would post duplicate messages
- Redis pub/sub delivers to all subscribers
- Multiple bots would spam the Discord channel

### ✅ High availability options

If you need redundancy:

1. **Standby replica**
   - Run a second replica with a different channel
   - Use for failover monitoring

2. **Leader election** (advanced)
   - Implement Redis-based leader election
   - Only leader posts to Discord
   - Followers take over on leader failure

3. **Multiple channels**
   - Different bots for different Discord servers
   - Each bot monitors same Redis but posts to different channels

---

## Troubleshooting Production Issues

### Bot keeps restarting

**Symptom**: Pod/container in crash loop

**Check**:
```bash
kubectl logs -n all-chat -l app=discord-bot --previous
```

**Common causes**:
- Invalid Discord token
- Missing environment variables
- Redis connection failure

### No messages appearing

**Symptom**: Bot is running but no Discord messages

**Check**:
1. Bot has permission to send messages
2. Channel ID is correct
3. Redis pub/sub is working:
   ```bash
   redis-cli SUBSCRIBE quota:alerts
   ```

### Duplicate messages

**Symptom**: Every message appears 2+ times

**Cause**: Multiple replicas running

**Fix**:
```bash
kubectl scale deployment/discord-bot --replicas=1 -n all-chat
```

### High memory usage

**Symptom**: Memory usage > 256MB

**Cause**: Memory leak (unlikely with current code)

**Fix**:
1. Check for memory leaks in logs
2. Restart the bot: `kubectl rollout restart deployment/discord-bot -n all-chat`
3. If persistent, report as bug

---

## Maintenance

### Updating the bot

```bash
# Kubernetes
kubectl set image deployment/discord-bot discord-bot=ghcr.io/your-org/discord-bot:new-tag -n all-chat

# Docker Compose
docker-compose pull discord-bot
docker-compose up -d discord-bot
```

### Changing configuration

```bash
# Update secret
kubectl edit secret discord-bot-secrets -n all-chat

# Restart to pick up changes
kubectl rollout restart deployment/discord-bot -n all-chat
```

### Backup and recovery

The bot is stateless - no data to backup!

To recover:
1. Ensure Discord token is saved securely
2. Redeploy with same configuration
3. Bot will resume operation immediately

---

## Cost Considerations

### Discord API

- **Free tier**: Unlimited messages (within rate limits)
- **Rate limits**:
  - 5 messages per 5 seconds per channel
  - Bot posts max 1 message per hour (periodic) + alerts (infrequent)
  - Well within free tier limits

### Infrastructure

- **Kubernetes**: ~0.05 CPU, ~128MB RAM
- **Docker**: Negligible resources
- **Network**: Minimal (only Discord API + Redis)

**Estimated cost**: < $0.50/month in most cloud providers

---

## Support Channels

- **GitHub Issues**: Report bugs or feature requests
- **Discord**: Join All-Chat community server
- **Logs**: First stop for troubleshooting
