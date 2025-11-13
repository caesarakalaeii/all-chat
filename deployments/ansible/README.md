# All-Chat Kubernetes Deployment with k3d

This directory contains Ansible playbooks to set up a local k3d Kubernetes cluster for All-Chat development and testing.

## Prerequisites

### System Requirements
- Linux (tested on Arch Linux)
- Docker installed and running
- Python 3.8+
- Ansible 2.9+

### Install Dependencies

```bash
# Install Ansible
sudo pacman -S ansible  # Arch Linux
# or
sudo apt-get install ansible  # Ubuntu/Debian

# Install Ansible Kubernetes module
ansible-galaxy collection install kubernetes.core

# Install Python dependencies
pip3 install kubernetes openshift
```

## Quick Start

### 1. Run the Playbook

```bash
cd deployments/ansible
ansible-playbook -i inventory.yml playbook.yml
```

This will:
1. Install k3d and kubectl (if not already installed)
2. Create a k3d cluster named "allchat"
3. Set up local Docker registry (localhost:5000)
4. Create allchat namespace
5. Deploy PostgreSQL and Redis
6. Create ConfigMaps and Secrets
7. Display cluster information

### 2. Build and Push Docker Images

```bash
# From repository root
cd deployments/ansible
./build-and-push.sh
```

This script:
- Builds all service Docker images
- Tags them for the local registry
- Pushes to localhost:5000

### 3. Deploy Application Services

```bash
kubectl apply -f ../k8s/base/ -n allchat --recursive
```

### 4. Verify Deployment

```bash
# Check all pods
kubectl get pods -n allchat

# Check all services
kubectl get services -n allchat

# Check deployments
kubectl get deployments -n allchat

# View logs
kubectl logs -n allchat -l app=youtube-listener --tail=50
kubectl logs -n allchat -l app=source-controller --tail=50
```

### 5. Port Forward for Local Access

```bash
# Run the generated port-forward script
./port-forward.sh

# Or manually:
kubectl port-forward -n allchat svc/api-gateway 8080:8080 &
kubectl port-forward -n allchat svc/postgres 5432:5432 &
kubectl port-forward -n allchat svc/redis 6379:6379 &
```

### 6. Test the Deployment

```bash
# Health checks
curl http://localhost:8080/health
curl http://localhost:8086/health/live  # YouTube Listener
curl http://localhost:8088/health/live  # Source Controller

# Status endpoints
curl http://localhost:8086/status | jq
curl http://localhost:8088/status | jq
```

## Cluster Management

### View Cluster Info

```bash
# Get cluster info
kubectl cluster-info

# Get nodes
kubectl get nodes

# Get all resources in allchat namespace
kubectl get all -n allchat
```

### Scale Services

```bash
# Scale YouTube Listener
kubectl scale deployment youtube-listener -n allchat --replicas=3

# Scale Source Controller
kubectl scale deployment source-controller -n allchat --replicas=2

# View HPAs
kubectl get hpa -n allchat
```

### View Logs

```bash
# All logs from a service
kubectl logs -n allchat -l app=youtube-listener --tail=100 -f

# Specific pod
kubectl logs -n allchat youtube-listener-xxxxx-yyy -f

# All pods in namespace
kubectl logs -n allchat --all-containers=true --tail=50
```

### Debugging

```bash
# Exec into a pod
kubectl exec -it -n allchat youtube-listener-xxxxx-yyy -- /bin/sh

# Describe a pod
kubectl describe pod -n allchat youtube-listener-xxxxx-yyy

# Get events
kubectl get events -n allchat --sort-by='.lastTimestamp'

# Check resource usage
kubectl top pods -n allchat
kubectl top nodes
```

## Testing Scenarios

### Test 1: YouTube Listener Health

```bash
# Port forward
kubectl port-forward -n allchat svc/youtube-listener 8086:8086 &

# Check health
curl http://localhost:8086/health/live
curl http://localhost:8086/health/ready
curl http://localhost:8086/status | jq

# Expected: status="running", quota tracking visible
```

### Test 2: Source Controller Leader Election

```bash
# Port forward
kubectl port-forward -n allchat svc/source-controller 8088:8088 &

# Check status
curl http://localhost:8088/status | jq

# Scale to 3 replicas
kubectl scale deployment source-controller -n allchat --replicas=3

# Wait for pods to start
kubectl wait --for=condition=Ready pod -l app=source-controller -n allchat --timeout=120s

# Check leadership (should see multiple instances, one leader per stream)
curl http://localhost:8088/leadership | jq

# Expected: Only one leader per stream, different instances can be leaders for different streams
```

### Test 3: Multi-Platform Message Flow

```bash
# Port forward API Gateway
kubectl port-forward -n allchat svc/api-gateway 8080:8080 &

# Create overlay with both platforms
TOKEN="your-jwt-token"
OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Multi-Platform K8s Test"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

# Add Twitch source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"twitch","channel_id":"xqc"}' \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources

# Add YouTube source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","channel_id":"UCxxxxxx"}' \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources

# Connect WebSocket
websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN"

# Expected: Messages from both Twitch and YouTube
```

### Test 4: Redis Streams and Pub/Sub

```bash
# Port forward Redis
kubectl port-forward -n allchat svc/redis 6379:6379 &

# Connect to Redis
redis-cli

# Check Streams
> XINFO GROUPS chat:raw
> XLEN chat:raw
> XREAD COUNT 10 STREAMS chat:raw 0

# Check Pub/Sub
> PUBSUB CHANNELS overlay:*
> SUBSCRIBE overlay:{overlay-id}
```

## Cleanup

### Delete Everything

```bash
# Delete cluster
k3d cluster delete allchat

# Or just delete the namespace
kubectl delete namespace allchat
```

### Restart Fresh

```bash
# Delete and recreate
k3d cluster delete allchat
ansible-playbook -i inventory.yml playbook.yml
```

## Troubleshooting

### Cluster Won't Start

```bash
# Check Docker is running
docker ps

# Check k3d version
k3d version

# Delete and recreate
k3d cluster delete allchat
k3d cluster create allchat --agents 2
```

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n allchat

# Describe failing pod
kubectl describe pod -n allchat <pod-name>

# Check logs
kubectl logs -n allchat <pod-name>

# Check events
kubectl get events -n allchat --sort-by='.lastTimestamp'
```

### Image Pull Errors

```bash
# Check if registry is running
docker ps | grep registry

# Rebuild and push images
cd deployments/ansible
./build-and-push.sh
```

### Database Connection Issues

```bash
# Check PostgreSQL pod
kubectl get pod -n allchat -l app=postgres

# Check PostgreSQL logs
kubectl logs -n allchat -l app=postgres

# Verify secret exists
kubectl get secret -n allchat allchat-secrets -o yaml

# Test connection from a pod
kubectl run -it --rm psql-test --image=postgres:16-alpine -n allchat -- \
  psql postgresql://allchat:allchat_dev_password@postgres:5432/allchat
```

### Redis Connection Issues

```bash
# Check Redis pod
kubectl get pod -n allchat -l app=redis

# Check Redis logs
kubectl logs -n allchat -l app=redis

# Test connection
kubectl run -it --rm redis-test --image=redis:7-alpine -n allchat -- \
  redis-cli -h redis ping
```

## Advanced Usage

### Update Secrets

```bash
# Edit secrets
kubectl edit secret allchat-secrets -n allchat

# Or delete and recreate
kubectl delete secret allchat-secrets -n allchat
kubectl create secret generic allchat-secrets -n allchat \
  --from-literal=YOUTUBE_CLIENT_ID=new-client-id \
  --from-literal=YOUTUBE_CLIENT_SECRET=new-secret

# Restart pods to pick up new secrets
kubectl rollout restart deployment youtube-listener -n allchat
```

### Update ConfigMap

```bash
# Edit ConfigMap
kubectl edit configmap allchat-config -n allchat

# Restart pods
kubectl rollout restart deployment -n allchat --all
```

### View Resource Usage

```bash
# Install metrics-server (if not already)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# View resource usage
kubectl top pods -n allchat
kubectl top nodes
```

## Architecture in Kubernetes

```
┌─────────────────────────────────────────┐
│         Namespace: allchat              │
│                                         │
│  ┌──────────────┐   ┌──────────────┐   │
│  │  PostgreSQL  │   │    Redis     │   │
│  │  (StatefulSet)│   │(StatefulSet) │   │
│  └──────┬───────┘   └──────┬───────┘   │
│         │                  │            │
│         └────────┬─────────┘            │
│                  │                      │
│    ┌─────────────┼──────────────┐       │
│    │             │              │       │
│    ▼             ▼              ▼       │
│  ┌────┐      ┌────┐        ┌────┐      │
│  │Auth│      │Over│        │Emote│     │
│  │Svc │      │lay │        │ Svc │     │
│  └────┘      └────┘        └────┘      │
│                  │                      │
│         ┌────────┴────────┐             │
│         │                 │             │
│         ▼                 ▼             │
│    ┌────────┐        ┌────────┐        │
│    │ Twitch │        │YouTube │        │
│    │Listener│        │Listener│        │
│    └───┬────┘        └───┬────┘        │
│        └────────┬─────────┘             │
│                 ▼                       │
│           ┌──────────┐                  │
│           │ Message  │                  │
│           │Processor │                  │
│           └─────┬────┘                  │
│                 ▼                       │
│           ┌──────────┐                  │
│           │   API    │                  │
│           │ Gateway  │                  │
│           └──────────┘                  │
└─────────────────────────────────────────┘
```

## Files

- `playbook.yml` - Main Ansible playbook
- `inventory.yml` - Ansible inventory (localhost)
- `build-and-push.sh` - Build and push Docker images to local registry
- `port-forward.sh` - Generated script for port forwarding
- `README.md` - This file

## Support

For issues with:
- **k3d**: https://k3d.io/
- **Kubernetes**: https://kubernetes.io/docs/
- **Ansible**: https://docs.ansible.com/

## License

See repository root LICENSE file.
