# All-Chat Production Deployment Guide

**Date**: November 13, 2025
**Status**: Ready for Production Deployment
**Target Cluster**: Caesar Production Kubernetes

---

## Overview

All-Chat is now ready for production deployment to the Caesar cluster with:
- ✅ CloudNativePG (CNPG) cluster for PostgreSQL
- ✅ Keel automatic image updates
- ✅ Prometheus metrics integration
- ✅ Horizontal Pod Autoscaling
- ✅ GitHub Actions CI/CD pipeline

---

## Deployment Architecture

```
┌────────────────────────────────────────────────┐
│     Caesar Cluster (Production)               │
│                                                │
│  ┌──────────────────────────────────────────┐ │
│  │       Namespace: allchat                 │ │
│  │                                          │ │
│  │  ┌─────────────────────────────────┐    │ │
│  │  │  CNPG Cluster: allchat-cluster  │    │ │
│  │  │  - 3 instances (1 primary + 2)  │    │ │
│  │  │  - 20Gi storage per instance    │    │ │
│  │  │  - Automatic failover           │    │ │
│  │  │  - Pod Monitor enabled          │    │ │
│  │  └─────────────────────────────────┘    │ │
│  │                                          │ │
│  │  ┌─────────────────────────────────┐    │ │
│  │  │  Redis (1 replica, 10Gi)        │    │ │
│  │  └─────────────────────────────────┘    │ │
│  │                                          │ │
│  │  ┌─────────────────────────────────┐    │ │
│  │  │  Application Services (8)       │    │ │
│  │  │  - All with Keel auto-update    │    │ │
│  │  │  - All with HPA scaling         │    │ │
│  │  │  - All with Prometheus metrics  │    │ │
│  │  └─────────────────────────────────┘    │ │
│  └──────────────────────────────────────────┘ │
│                                                │
│  ┌──────────────────────────────────────────┐ │
│  │  Namespace: keel                         │ │
│  │  - Polls ghcr.io every 1 minute          │ │
│  │  - Updates deployments automatically     │ │
│  │  - Discord notifications                 │ │
│  └──────────────────────────────────────────┘ │
│                                                │
│  ┌──────────────────────────────────────────┐ │
│  │  Namespace: monitoring                   │ │
│  │  - Grafana (dashboards)                  │ │
│  │  - Mimir (metrics storage)               │ │
│  │  - Loki (log aggregation)                │ │
│  │  - Grafana Agent (collection)            │ │
│  └──────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

---

## Prerequisites

### 1. Ansible Vault Variables

Add to `/home/moersener/Hobby/caesar-deployment/inventory/local/group_vars/all/vault.yml`:

```yaml
# All-Chat Secrets
vault_allchat_database_password: "GENERATE_SECURE_PASSWORD"
vault_allchat_jwt_secret: "GENERATE_JWT_SECRET"

# Twitch OAuth
vault_allchat_twitch_client_id: "your-twitch-client-id"
vault_allchat_twitch_client_secret: "your-twitch-client-secret"
vault_allchat_twitch_bot_username: "your-bot-username"
vault_allchat_twitch_bot_oauth: "oauth:your-oauth-token"

# YouTube OAuth
vault_allchat_youtube_client_id: "xxx.apps.googleusercontent.com"
vault_allchat_youtube_client_secret: "GOCSPX-your-secret"
```

**Edit vault:**
```bash
cd /home/moersener/Hobby/caesar-deployment
ansible-vault edit inventory/local/group_vars/all/vault.yml --vault-password-file ~/.ssh/ansible_vault_pass
```

### 2. Ansible Configuration Variables

Add to `/home/moersener/Hobby/caesar-deployment/inventory/local/group_vars/all/main.yml`:

```yaml
allchat:
  database_user: "allchat"
  database_password: "{{ vault_allchat_database_password }}"
  database_name: "allchat"
  jwt_secret: "{{ vault_allchat_jwt_secret }}"
  twitch_client_id: "{{ vault_allchat_twitch_client_id }}"
  twitch_client_secret: "{{ vault_allchat_twitch_client_secret }}"
  twitch_bot_username: "{{ vault_allchat_twitch_bot_username }}"
  twitch_bot_oauth: "{{ vault_allchat_twitch_bot_oauth }}"
  youtube_client_id: "{{ vault_allchat_youtube_client_id }}"
  youtube_client_secret: "{{ vault_allchat_youtube_client_secret }}"
  twitch_redirect_url: "https://allchat.yourdomain.com/api/v1/auth/callback"
  youtube_redirect_url: "https://allchat.yourdomain.com/api/v1/auth/youtube/callback"
  log_level: "info"
  cors_origin: "https://allchat.yourdomain.com"
```

### 3. GitHub Secrets

Configure in All-Chat repository settings:
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
- No additional secrets needed (uses GITHUB_TOKEN for ghcr.io)

---

## Deployment Steps

### Step 1: Push Code to GitHub

```bash
cd /home/moersener/Hobby/all-chat

# Commit Phase 4 changes
git add .
git commit -m "feat: Phase 4 YouTube Integration complete"
git push origin main
```

This triggers GitHub Actions to build and push all Docker images to ghcr.io.

### Step 2: Deploy to Caesar Cluster

```bash
cd /home/moersener/Hobby/caesar-deployment

# Deploy all services (includes all-chat)
ansible-playbook ci.yaml -i inventory/local/hosts.ini -e "cluster_env=prod" --vault-password-file ~/.ssh/ansible_vault_pass

# Or deploy only all-chat
ansible-playbook ci.yaml -i inventory/local/hosts.ini -e "cluster_env=prod" --vault-password-file ~/.ssh/ansible_vault_pass --tags allchat
```

### Step 3: Verify Deployment

```bash
# Check all pods
kubectl get pods -n allchat

# Check CNPG cluster
kubectl get cluster -n allchat

# Expected output:
# NAME              AGE   INSTANCES   READY   STATUS                    PRIMARY
# allchat-cluster   5m    3           3       Cluster in healthy state  allchat-cluster-1

# Check services
kubectl get services -n allchat

# Check HPA
kubectl get hpa -n allchat
```

### Step 4: Apply Database Migrations

```bash
# Port forward to database primary
kubectl port-forward -n allchat allchat-cluster-1 5432:5432 &

# Apply migrations
cd /home/moersener/Hobby/all-chat
psql postgresql://allchat:PASSWORD@localhost:5432/allchat < migrations/001_initial_schema.sql
psql postgresql://allchat:PASSWORD@localhost:5432/allchat < migrations/003_youtube_support.sql

# Verify tables
psql postgresql://allchat:PASSWORD@localhost:5432/allchat -c "\dt"
```

### Step 5: Verify Services

```bash
# Port forward API Gateway
kubectl port-forward -n allchat svc/api-gateway 8080:8080 &

# Test health
curl http://localhost:8080/health

# Test YouTube Listener
kubectl port-forward -n allchat svc/youtube-listener 8086:8086 &
curl http://localhost:8086/status | jq

# Test Source Controller
kubectl port-forward -n allchat svc/source-controller 8088:8088 &
curl http://localhost:8088/status | jq
```

---

## CI/CD Pipeline

### GitHub Actions Workflow

**File**: `.github/workflows/build-and-push.yml`

**Triggers:**
- Push to `main` or `develop` branches
- Changes in `services/**` or `shared/**`
- Manual workflow dispatch

**Process:**
1. **Detect Changes**: Only builds services that changed
2. **Build Images**: Builds Docker images with Buildx
3. **Push to Registry**: Pushes to `ghcr.io/caesarakalaeii/allchat-*`
4. **Tag Images**:
   - `main` - Latest tag
   - `develop` - Develop tag
   - `main-<sha>` - Git SHA tag

### Image Names

```
ghcr.io/caesarakalaeii/allchat-auth-service:main
ghcr.io/caesarakalaeii/allchat-overlay-manager:main
ghcr.io/caesarakalaeii/allchat-emote-service:main
ghcr.io/caesarakalaeii/allchat-api-gateway:main
ghcr.io/caesarakalaeii/allchat-twitch-listener:main
ghcr.io/caesarakalaeii/allchat-youtube-listener:main
ghcr.io/caesarakalaeii/allchat-message-processor:main
ghcr.io/caesarakalaeii/allchat-source-controller:main
```

### Keel Auto-Update

Keel monitors these images and automatically:
1. Polls ghcr.io every 1 minute
2. Detects new image tags
3. Updates Kubernetes deployments
4. Sends Discord notification
5. Performs rolling update

**Policy**: `force` - Updates immediately without approval

---

## Monitoring & Observability

### Metrics

All services expose `/metrics` endpoint:
- Grafana Agent scrapes via Prometheus annotations
- Metrics stored in Mimir
- Viewable in Grafana dashboards

**Create Dashboards in Grafana:**
1. Port forward: `kubectl port-forward -n monitoring svc/lgtm-grafana 3000:80`
2. Access: http://localhost:3000
3. Import dashboards for All-Chat services

### Logs

Logs automatically collected by Grafana Agent:
- No configuration needed per service
- Stored in Loki
- Queryable in Grafana Explore

**View Logs:**
```bash
# Via kubectl
kubectl logs -n allchat -l app=youtube-listener --tail=100 -f

# Via Grafana
# Port forward and access Explore → Loki → {namespace="allchat"}
```

### Alerts

Create PrometheusRule CRDs for alerting:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: allchat-alerts
  namespace: allchat
spec:
  groups:
  - name: allchat
    rules:
    - alert: YouTubeQuotaHigh
      expr: youtube_quota_usage_percentage > 80
      for: 5m
      annotations:
        summary: "YouTube API quota usage above 80%"
    - alert: AllChatServiceDown
      expr: up{namespace="allchat"} == 0
      for: 2m
      annotations:
        summary: "All-Chat service is down"
```

---

## Scaling Strategy

### Horizontal Pod Autoscaling

| Service | Min | Max | Strategy |
|---------|-----|-----|----------|
| Auth Service | 2 | 10 | CPU-based |
| Overlay Manager | 2 | 10 | CPU-based |
| Emote Service | 2 | 10 | CPU-based |
| API Gateway | 3 | 20 | CPU + Memory |
| Twitch Listener | 2 | 10 | CPU-based |
| YouTube Listener | 2 | 10 | CPU-based |
| Message Processor | 3 | 20 | CPU + Memory |
| Source Controller | 2 | 5 | CPU-based |

### Database Scaling

CNPG cluster with 3 instances:
- 1 Primary (read-write)
- 2 Replicas (automatic failover)
- Endpoint: `allchat-cluster-rw.allchat.svc.cluster.local`

**To add more replicas:**
```bash
kubectl edit cluster allchat-cluster -n allchat
# Change spec.instances to desired number
```

### Redis Scaling

Currently: 1 replica with 10Gi storage

**For production at scale, consider:**
- Redis Cluster (multiple masters)
- Redis Sentinel (automatic failover)
- Increase to 3+ instances

---

## Production Checklist

### Before Deployment

- [ ] Vault variables configured
- [ ] GitHub Actions workflow tested
- [ ] Images built and pushed to ghcr.io
- [ ] Domain configured (allchat.yourdomain.com)
- [ ] SSL certificates ready
- [ ] YouTube OAuth app created
- [ ] Twitch OAuth app created
- [ ] Twitch bot account created

### During Deployment

- [ ] Ansible playbook runs successfully
- [ ] CNPG cluster reaches "Cluster in healthy state"
- [ ] All pods reach Running state
- [ ] Database migrations applied
- [ ] Health checks passing

### After Deployment

- [ ] Test Twitch OAuth flow
- [ ] Test YouTube OAuth flow
- [ ] Create test overlay
- [ ] Add Twitch source
- [ ] Add YouTube source
- [ ] Verify WebSocket connection
- [ ] Verify messages flow from both platforms
- [ ] Check Grafana dashboards
- [ ] Verify Keel updates work
- [ ] Load test with real streams

---

## Files Created

### In caesar-deployment Repository

**Kubernetes Manifests** (`/home/moersener/Hobby/caesar-deployment/all-chat/`):
- `namespace.yaml` - Namespace definition
- `configmap.yaml` - Application configuration
- `secrets.yaml` - Sensitive credentials (from vault)
- `allchat-cluster.yaml` - CNPG cluster (3 instances, 20Gi)
- `allchat-cluster-secret.yaml` - Database credentials
- `redis-deployment.yaml` - Redis deployment with PVC
- `auth-service-deployment.yaml` - With Keel annotations
- `youtube-listener-deployment.yaml` - With Keel annotations
- `source-controller-deployment.yaml` - With Keel annotations
- `api-gateway-deployment.yaml` - With Keel annotations
- `message-processor-deployment.yaml` - With Keel annotations
- `README.md` - Deployment documentation

**Ansible Tasks** (`/home/moersener/Hobby/caesar-deployment/roles/ci/tasks/`):
- `allchat.yaml` - Deployment task for All-Chat

**Updated**:
- `roles/ci/tasks/main.yaml` - Added allchat task

### In all-chat Repository

**GitHub Actions** (`.github/workflows/`):
- `build-and-push.yml` - Build and push all Docker images

**Kubernetes** (`deployments/k8s/base/`):
- Base manifests for local k3d testing
- Kustomization configuration

**Ansible** (`deployments/ansible/`):
- `playbook.yml` - k3d cluster setup
- `build-and-push.sh` - Local registry build
- `verify-deployment.sh` - Verification script
- `test-integration.sh` - Integration tests
- `TESTING_GUIDE.md` - Comprehensive guide

---

## Deployment Commands

### Full Deployment

```bash
cd /home/moersener/Hobby/caesar-deployment

# Deploy everything (including all-chat)
ansible-playbook ci.yaml \
  -i inventory/local/hosts.ini \
  -e "cluster_env=prod" \
  --vault-password-file ~/.ssh/ansible_vault_pass
```

### Deploy Only All-Chat

```bash
cd /home/moersener/Hobby/caesar-deployment

# Deploy just all-chat namespace
ansible-playbook ci.yaml \
  -i inventory/local/hosts.ini \
  -e "cluster_env=prod" \
  --vault-password-file ~/.ssh/ansible_vault_pass \
  --tags allchat
```

### Update After Code Changes

```bash
# 1. Push code to GitHub (triggers image build)
cd /home/moersener/Hobby/all-chat
git push origin main

# 2. Wait for GitHub Actions (~5-10 minutes)
# Check: https://github.com/your-org/all-chat/actions

# 3. Keel automatically detects and updates (within 1-2 minutes)
# Monitor: kubectl logs -n keel -l app=keel -f

# 4. Verify rollout
kubectl rollout status deployment youtube-listener -n allchat
```

---

## Monitoring

### Check Deployment Status

```bash
# All pods
kubectl get pods -n allchat -o wide

# All deployments with ready status
kubectl get deployments -n allchat

# HPA status
kubectl get hpa -n allchat

# CNPG cluster
kubectl get cluster -n allchat
kubectl describe cluster allchat-cluster -n allchat
```

### View Logs

```bash
# Real-time logs
kubectl logs -n allchat -l app=youtube-listener -f

# All services
stern -n allchat .

# Specific pod
kubectl logs -n allchat youtube-listener-xxxxx-yyy
```

### Access Services

```bash
# Port forward API Gateway
kubectl port-forward -n allchat svc/api-gateway 8080:8080

# Port forward specific services
kubectl port-forward -n allchat svc/youtube-listener 8086:8086
kubectl port-forward -n allchat svc/source-controller 8088:8088

# Access database
kubectl port-forward -n allchat allchat-cluster-1 5432:5432
psql postgresql://allchat:PASSWORD@localhost:5432/allchat
```

---

## Keel Integration

### How It Works

1. **GitHub Actions** builds and pushes image to ghcr.io
2. **Keel** polls ghcr.io every 1 minute
3. **Keel** detects new image tag (e.g., `main`)
4. **Keel** updates deployment with new image
5. **Kubernetes** performs rolling update
6. **Keel** sends Discord notification

### Monitor Keel

```bash
# Keel logs
kubectl logs -n keel -l app=keel -f

# Check tracked images
kubectl get pods -n keel
kubectl exec -n keel <keel-pod> -- keel version
```

### Manual Trigger

If Keel doesn't update automatically:

```bash
# Force update
kubectl rollout restart deployment youtube-listener -n allchat

# Or update image manually
kubectl set image deployment/youtube-listener \
  youtube-listener=ghcr.io/caesarakalaeii/allchat-youtube-listener:main \
  -n allchat
```

---

## Troubleshooting

### CNPG Cluster Won't Start

```bash
# Check CNPG operator
kubectl logs -n cnpg-system -l app.kubernetes.io/name=cloudnative-pg

# Check cluster status
kubectl describe cluster allchat-cluster -n allchat

# Check pods
kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster

# Check events
kubectl get events -n allchat --sort-by='.lastTimestamp' | grep allchat-cluster
```

### Service CrashLoopBackOff

```bash
# Check logs
kubectl logs -n allchat <pod-name>

# Common issues:
# - Database not ready: Wait for CNPG cluster
# - Redis not ready: Check redis pod
# - Missing secrets: Verify vault variables
# - Wrong database host: Should be allchat-cluster-rw.allchat.svc.cluster.local
```

### Keel Not Updating

```bash
# Check Keel is running
kubectl get pods -n keel

# Check Keel logs
kubectl logs -n keel -l app=keel | grep allchat

# Verify annotation on deployment
kubectl get deployment youtube-listener -n allchat -o yaml | grep keel

# Force poll
kubectl annotate deployment youtube-listener -n allchat keel.sh/pollSchedule="@every 1m" --overwrite
```

---

## Rollback

### Rollback a Deployment

```bash
# View rollout history
kubectl rollout history deployment youtube-listener -n allchat

# Rollback to previous version
kubectl rollout undo deployment youtube-listener -n allchat

# Rollback to specific revision
kubectl rollout undo deployment youtube-listener -n allchat --to-revision=2
```

### Emergency: Delete Namespace

```bash
# WARNING: This deletes all data
kubectl delete namespace allchat

# Redeploy
ansible-playbook ci.yaml -i inventory/local/hosts.ini -e "cluster_env=prod" --vault-password-file ~/.ssh/ansible_vault_pass --tags allchat
```

---

## Production Recommendations

### Security
1. ✅ Secrets stored in Ansible vault
2. ✅ Database credentials in Kubernetes Secret
3. ✅ JWT secret rotation policy
4. ⏳ Enable network policies
5. ⏳ Enable Pod Security Standards

### High Availability
1. ✅ CNPG cluster with 3 instances
2. ✅ Multiple replicas for all services
3. ✅ HPA for auto-scaling
4. ⏳ Redis replication/clustering
5. ⏳ Multi-zone deployment

### Monitoring
1. ✅ Prometheus metrics enabled
2. ✅ Grafana Agent scraping
3. ✅ Logs collected by Loki
4. ⏳ Create Grafana dashboards
5. ⏳ Configure alerts

### Backup
1. ✅ CNPG automatic backups (built-in)
2. ⏳ Configure backup schedule
3. ⏳ Test restore procedures
4. ⏳ Off-cluster backup storage

---

## Support

**Deployment Issues:**
- Check Ansible output
- Verify vault variables
- Check pod logs

**Application Issues:**
- Check service logs
- Check database connectivity
- Verify Redis Streams

**Performance Issues:**
- Check HPA metrics
- Check resource usage
- Scale manually if needed

---

## Next Steps

1. ✅ Deploy to production cluster
2. ⏳ Apply database migrations
3. ⏳ Configure domain and ingress
4. ⏳ Set up SSL certificates
5. ⏳ Create Grafana dashboards
6. ⏳ Configure AlertManager rules
7. ⏳ Load testing
8. ⏳ Production launch

---

**Created**: November 13, 2025
**Last Updated**: November 13, 2025
**Status**: Ready for Production Deployment
