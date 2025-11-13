# All-Chat Kubernetes Testing Guide

Complete guide for testing All-Chat on a local k3d cluster.

## Quick Start

```bash
# 1. Set up cluster
ansible-playbook -i inventory.yml playbook.yml

# 2. Build and push images
./build-and-push.sh

# 3. Verify deployment
./verify-deployment.sh

# 4. Start port forwarding
./port-forward.sh &

# 5. Run integration tests
./test-integration.sh
```

---

## Detailed Setup

### Step 1: Install Prerequisites

```bash
# Arch Linux
sudo pacman -S ansible docker kubectl

# Ubuntu/Debian
sudo apt-get install ansible docker.io
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install Ansible Kubernetes collection
ansible-galaxy collection install kubernetes.core
pip3 install kubernetes openshift

# Start Docker
sudo systemctl start docker
sudo usermod -aG docker $USER  # Add yourself to docker group
newgrp docker  # Activate group without logout
```

### Step 2: Run Ansible Playbook

```bash
cd /home/moersener/Hobby/all-chat/deployments/ansible

# Run playbook
ansible-playbook -i inventory.yml playbook.yml

# What it does:
# - Installs k3d (if needed)
# - Creates k3d cluster "allchat"
# - Creates local Docker registry (localhost:5000)
# - Creates namespace "allchat"
# - Deploys PostgreSQL and Redis
# - Creates ConfigMaps and Secrets
# - Displays cluster info
```

**Expected output:**
```
PLAY [Set up k3d cluster for All-Chat] *************************************

TASK [Check if k3d is installed] *******************************************
ok: [localhost]

TASK [Create k3d cluster] **************************************************
changed: [localhost]

TASK [Wait for cluster to be ready] ****************************************
ok: [localhost]

...

TASK [Display next steps] **************************************************
ok: [localhost] => {
    "msg": [
        "k3d cluster 'allchat' is ready!",
        ...
    ]
}
```

### Step 3: Build Docker Images

```bash
# Build all services and push to local registry
./build-and-push.sh

# This builds:
# - auth-service
# - overlay-manager
# - emote-service
# - api-gateway
# - twitch-listener
# - youtube-listener (NEW)
# - message-processor
# - source-controller (NEW)
```

**Expected output:**
```
Building: youtube-listener
...
✅ youtube-listener complete

Building: source-controller
...
✅ source-controller complete

All images built and pushed successfully!
```

### Step 4: Deploy Application Services

```bash
# Apply all Kubernetes manifests
kubectl apply -f ../k8s/base/ -n allchat --recursive

# Wait for deployments
kubectl wait --for=condition=Available deployment --all -n allchat --timeout=600s

# Check status
kubectl get pods -n allchat
```

**Expected pods:**
- postgres-xxxxx-yyy (1/1 Running)
- redis-xxxxx-yyy (1/1 Running)
- auth-service-xxxxx-yyy (1/1 Running)
- overlay-manager-xxxxx-yyy (1/1 Running)
- emote-service-xxxxx-yyy (1/1 Running)
- api-gateway-xxxxx-yyy (1/1 Running)
- twitch-listener-xxxxx-yyy (1/1 Running)
- youtube-listener-xxxxx-yyy (1/1 Running)
- message-processor-xxxxx-yyy (1/1 Running)
- source-controller-xxxxx-yyy (1/1 Running)

### Step 5: Verify Deployment

```bash
./verify-deployment.sh
```

This script checks:
- ✅ Prerequisites installed
- ✅ Cluster exists
- ✅ Namespace exists
- ✅ ConfigMap and Secret exist
- ✅ All pods running
- ✅ All services exist
- ✅ Deployments ready

---

## Testing Scenarios

### Scenario 1: YouTube Listener Health

```bash
# Port forward YouTube Listener
kubectl port-forward -n allchat svc/youtube-listener 8086:8086 &

# Test health
curl http://localhost:8086/health/live
# Expected: {"status":"alive"}

curl http://localhost:8086/health/ready
# Expected: {"status":"ready"}

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
```

### Scenario 2: Source Controller Leader Election

```bash
# Port forward Source Controller
kubectl port-forward -n allchat svc/source-controller 8088:8088 &

# Check status
curl http://localhost:8088/status | jq

# Scale to 3 replicas
kubectl scale deployment source-controller -n allchat --replicas=3

# Wait for pods
kubectl wait --for=condition=Ready pod -l app=source-controller -n allchat --timeout=120s

# Check instance IDs
kubectl get pods -n allchat -l app=source-controller -o wide

# Check leadership status (all instances should have unique IDs)
curl http://localhost:8088/status | jq '.instance_id'

# Scale back
kubectl scale deployment source-controller -n allchat --replicas=1
```

### Scenario 3: PostgreSQL Data Persistence

```bash
# Port forward PostgreSQL
kubectl port-forward -n allchat svc/postgres 5432:5432 &

# Connect with psql
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat

# Check tables
\dt

# Expected tables:
# - users
# - overlays
# - overlay_configs
# - overlay_chat_sources
# - youtube_oauth_tokens (from migration 003)
# - youtube_quota_usage (from migration 003)
# - supported_platforms (from migration 003)

# Check supported platforms
SELECT platform, display_name, is_enabled FROM supported_platforms;

# Expected:
# twitch  | Twitch  | t
# youtube | YouTube | t
# kick    | Kick    | f
# tiktok  | TikTok  | f
```

### Scenario 4: Redis Streams

```bash
# Port forward Redis
kubectl port-forward -n allchat svc/redis 6379:6379 &

# Connect with redis-cli
redis-cli -h localhost

# Check Streams
> XINFO GROUPS chat:raw
# Expected: Consumer group "message-processor" exists

> XLEN chat:raw
# Expected: 0 (no messages yet)

# Check Pub/Sub channels
> PUBSUB CHANNELS overlay:*
# Expected: Empty (no active overlays yet)
```

### Scenario 5: Service Communication

```bash
# Check if services can communicate
kubectl exec -it -n allchat deployment/youtube-listener -- /bin/sh

# From inside pod:
wget -qO- http://postgres:5432
# Expected: PostgreSQL connection attempt

wget -qO- http://redis:6379
# Expected: Redis connection attempt

wget -qO- http://source-controller:8088/health/live
# Expected: {"status":"alive"}

exit
```

### Scenario 6: Message Processor Platform Routing

```bash
# Check Message Processor logs
kubectl logs -n allchat -l app=message-processor --tail=100

# Expected log lines:
# "Loaded normalizers" (should mention both twitch and youtube)
# "Message processor started"

# Check if both normalizers are loaded
kubectl logs -n allchat -l app=message-processor | grep -i normalizer
```

---

## Load Testing

### Scale Services

```bash
# Scale YouTube Listener to handle more streams
kubectl scale deployment youtube-listener -n allchat --replicas=3

# Scale Source Controller for redundancy
kubectl scale deployment source-controller -n allchat --replicas=2

# Scale Message Processor for higher throughput
kubectl scale deployment message-processor -n allchat --replicas=5

# Verify
kubectl get pods -n allchat -l component=listener
kubectl get pods -n allchat -l component=controller
```

### Monitor Resource Usage

```bash
# Install metrics-server (if not already)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Wait for metrics to be available
sleep 30

# Check resource usage
kubectl top pods -n allchat
kubectl top nodes

# Check HPA status
kubectl get hpa -n allchat
```

---

## Debugging

### View Logs

```bash
# All logs from a service
kubectl logs -n allchat -l app=youtube-listener --tail=100 -f

# All containers in a pod
kubectl logs -n allchat youtube-listener-xxxxx-yyy --all-containers=true

# Previous crashed container
kubectl logs -n allchat youtube-listener-xxxxx-yyy --previous

# Save logs to file
kubectl logs -n allchat -l app=youtube-listener --tail=1000 > youtube-listener.log
```

### Describe Resources

```bash
# Describe pod (shows events, status, conditions)
kubectl describe pod -n allchat youtube-listener-xxxxx-yyy

# Describe deployment
kubectl describe deployment -n allchat youtube-listener

# Get events
kubectl get events -n allchat --sort-by='.lastTimestamp' | tail -20
```

### Exec into Pods

```bash
# YouTube Listener (Alpine-based, use sh)
kubectl exec -it -n allchat deployment/youtube-listener -- /bin/sh

# Check environment
env | grep -E '(DATABASE|REDIS|YOUTUBE)'

# Test connectivity
wget -qO- http://postgres:5432  # Should show PostgreSQL response
wget -qO- http://redis:6379     # Should show Redis response

exit
```

### Check ConfigMaps and Secrets

```bash
# View ConfigMap
kubectl get configmap allchat-config -n allchat -o yaml

# View Secret (base64 encoded)
kubectl get secret allchat-secrets -n allchat -o yaml

# Decode a secret value
kubectl get secret allchat-secrets -n allchat -o jsonpath='{.data.JWT_SECRET}' | base64 -d
```

### Restart Services

```bash
# Restart a deployment (rolling restart)
kubectl rollout restart deployment youtube-listener -n allchat

# Restart all deployments
kubectl rollout restart deployment -n allchat --all

# Check rollout status
kubectl rollout status deployment youtube-listener -n allchat
```

---

## Cleanup

### Delete Namespace Only

```bash
kubectl delete namespace allchat
```

### Delete Entire Cluster

```bash
k3d cluster delete allchat
```

### Delete and Recreate

```bash
k3d cluster delete allchat
ansible-playbook -i inventory.yml playbook.yml
./build-and-push.sh
kubectl apply -f ../k8s/base/ -n allchat --recursive
```

---

## Common Issues

### Issue: Pods in CrashLoopBackOff

**Cause**: Usually database or Redis connection issues

**Solution:**
```bash
# Check pod logs
kubectl logs -n allchat <pod-name>

# Check if postgres/redis are ready
kubectl get pods -n allchat -l app=postgres
kubectl get pods -n allchat -l app=redis

# Restart the failing pod
kubectl delete pod -n allchat <pod-name>
```

### Issue: ImagePullBackOff

**Cause**: Image not in local registry

**Solution:**
```bash
# Check registry contents
curl http://localhost:5000/v2/_catalog

# Rebuild and push
./build-and-push.sh

# Restart deployment
kubectl rollout restart deployment <service-name> -n allchat
```

### Issue: Port Forward Fails

**Cause**: Port already in use

**Solution:**
```bash
# Check what's using the port
lsof -i :8080

# Kill the process
kill <PID>

# Or use a different local port
kubectl port-forward -n allchat svc/api-gateway 9080:8080
```

### Issue: Database Migration Not Applied

**Cause**: Migrations not run on init

**Solution:**
```bash
# Port forward PostgreSQL
kubectl port-forward -n allchat svc/postgres 5432:5432 &

# Apply migrations manually
cd ../..
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat < migrations/001_initial_schema.sql
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat < migrations/003_youtube_support.sql

# Verify
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat -c "\dt"
```

---

## Performance Testing

### Simulate YouTube API Load

```bash
# Create mock overlays with YouTube sources
for i in {1..50}; do
  curl -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Test Overlay $i\"}" \
    http://localhost:8080/api/v1/overlays
done

# Monitor YouTube Listener
kubectl logs -n allchat -l app=youtube-listener -f

# Monitor quota usage
watch -n 5 'curl -s http://localhost:8086/status | jq ".quota"'
```

### Simulate High Message Volume

```bash
# Scale Message Processor
kubectl scale deployment message-processor -n allchat --replicas=5

# Monitor Redis Streams backlog
redis-cli -h localhost XINFO GROUPS chat:raw
redis-cli -h localhost XPENDING chat:raw message-processor

# Monitor processing rate
kubectl logs -n allchat -l app=message-processor -f | grep "Published"
```

---

## CI/CD Integration

This setup can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up k3d
        run: |
          cd deployments/ansible
          ansible-playbook -i inventory.yml playbook.yml

      - name: Build and deploy
        run: |
          cd deployments/ansible
          ./build-and-push.sh
          kubectl apply -f ../k8s/base/ -n allchat --recursive

      - name: Run tests
        run: |
          cd deployments/ansible
          ./verify-deployment.sh
          ./test-integration.sh

      - name: Cleanup
        if: always()
        run: k3d cluster delete allchat
```

---

## Next Steps

After successful deployment:

1. **Write Tests**: Add unit and integration tests for Phase 4 services
2. **Manual Testing**: Test YouTube OAuth flow with real credentials
3. **Load Testing**: Simulate 50+ YouTube streams
4. **Frontend**: Build UI for overlay management
5. **Production**: Deploy to real Kubernetes cluster (GKE, EKS, AKS)

---

## Resources

- **k3d Documentation**: https://k3d.io/
- **Kubernetes Documentation**: https://kubernetes.io/docs/
- **Ansible Kubernetes Module**: https://docs.ansible.com/ansible/latest/collections/kubernetes/core/
- **YouTube Data API**: https://developers.google.com/youtube/v3

---

## Support

For issues:
1. Check `kubectl get events -n allchat --sort-by='.lastTimestamp'`
2. Check pod logs: `kubectl logs -n allchat <pod-name>`
3. Check CHECKPOINT.md for known issues
4. Open issue on repository

**Last Updated**: November 13, 2025
