# All-Chat Deployment Status

**Last Updated**: November 13, 2025
**Status**: ✅ Automated Deployment Pipeline Complete

---

## ✅ What's Complete

### 1. Automated Ansible Vault Setup
- **Location**: `deployments/ansible/`
- **Features**:
  - Reads credentials from `.env` file automatically
  - Generates `secrets.vault.yml` with one command
  - Encrypts using password from `~/.ssh/ansible_vault_pass`
  - **Zero manual editing required!**

**Commands**:
```bash
cd deployments/ansible
./generate-vault-from-env.sh  # Auto-generates from .env
./deploy.sh                     # Deploys to k3d cluster
```

### 2. GitHub Container Registry (GHCR) Integration
- **Workflow**: `.github/workflows/build-and-push.yml`
- **Trigger**: Builds on ANY branch push
- **Tagging Strategy**:
  - `main` → Branch name tag
  - `main-abc1234` → Branch + commit SHA
  - `latest` → Only on default branch (main)

**Images Built**:
- `ghcr.io/caesarakalaeii/allchat-auth-service:main`
- `ghcr.io/caesarakalaeii/allchat-overlay-manager:main`
- `ghcr.io/caesarakalaeii/allchat-emote-service:main`
- `ghcr.io/caesarakalaeii/allchat-api-gateway:main`
- `ghcr.io/caesarakalaeii/allchat-twitch-listener:main`
- `ghcr.io/caesarakalaeii/allchat-youtube-listener:main`
- `ghcr.io/caesarakalaeii/allchat-message-processor:main`
- `ghcr.io/caesarakalaeii/allchat-source-manager:main`

### 3. Kubernetes Deployment
- **Cluster**: k3d (local Kubernetes)
- **Cluster Name**: `allchat`
- **Namespace**: `allchat`
- **Services Deployed**: 10 (8 microservices + PostgreSQL + Redis)

**Current Status**:
- ✅ k3d cluster created
- ✅ PostgreSQL running
- ✅ Redis running
- ⏳ Service pods waiting for GHCR images (being built by GitHub Actions)

### 4. Documentation Created
- `deployments/ansible/QUICKSTART.md` - 3-command deployment guide
- `deployments/ansible/README.md` - Full deployment documentation
- `deployments/ansible/README_VAULT.md` - Ansible Vault guide
- `frontend/README_TESTING.md` - Playwright testing guide
- `CHECKPOINT.md` - Complete project status
- `DEPLOYMENT_STATUS.md` - This file

---

## 🚀 Deployment Workflow

### Step 1: Generate Vault from .env
```bash
cd deployments/ansible
./generate-vault-from-env.sh
```

Output shows all credentials loaded:
```
Twitch OAuth:
  • Client ID: zdqxhcv9n8...
  • Client Secret: 1pqh1bh219...
  • Bot Username: caesarlp
  • Bot OAuth: oauth:z38qsx8...

YouTube:
  • Client ID: ...
  • Client Secret: ...
  • API Key: AIzaSyBs2z...

Security:
  • JWT Secret: CHANGE_ME...
  • Database Password: ...
```

### Step 2: Push Code (Triggers GitHub Actions)
```bash
git add -A
git commit -m "Your changes"
git push origin main
```

GitHub Actions automatically:
- Detects changed services
- Builds Docker images with Go 1.25
- Pushes to GHCR with branch tags
- All 8 services built in parallel

### Step 3: Deploy to Kubernetes
```bash
cd deployments/ansible
./deploy.sh
```

Deployment script:
- Uses vault password from `~/.ssh/ansible_vault_pass`
- Creates k3d cluster if needed
- Deploys PostgreSQL + Redis
- Applies all service manifests
- Waits for deployments to be ready

### Step 4: Verify Deployment
```bash
kubectl get pods -n allchat
kubectl get services -n allchat
kubectl logs -n allchat -l app=api-gateway
```

### Step 5: Access Services
```bash
# Port forward API Gateway
kubectl port-forward -n allchat svc/api-gateway 8080:8080

# Or use the generated script
./port-forward.sh
```

---

## 📊 Current Cluster Status

```
$ kubectl get pods -n allchat

NAME                                 READY   STATUS
postgres-77bc476bc9-68w66            1/1     Running     ✅
redis-74bbdb694c-rjccq               1/1     Running     ✅
api-gateway-*                        0/1     ImagePullBackOff   ⏳
auth-service-*                       0/1     ImagePullBackOff   ⏳
emote-service-*                      0/1     ImagePullBackOff   ⏳
message-processor-*                  0/1     ImagePullBackOff   ⏳
overlay-manager-*                    0/1     ImagePullBackOff   ⏳
source-manager-*                     0/1     ImagePullBackOff   ⏳
twitch-listener-*                    0/1     ImagePullBackOff   ⏳
youtube-listener-*                   0/1     ImagePullBackOff   ⏳
```

**Status**: Waiting for GitHub Actions to build and publish images to GHCR.

**Once images are ready**, pods will automatically pull and start running.

---

## 🔄 What Happens Next

### Automatic Image Pull
Once GitHub Actions completes:
1. Images are available at `ghcr.io/caesarakalaeii/allchat-*:main`
2. Kubernetes will retry image pull (automatic)
3. Pods will transition from `ImagePullBackOff` → `Running`
4. Services become available

### Manual Refresh (if needed)
```bash
# Force pods to restart and pull new images
kubectl rollout restart deployment --all -n allchat

# Watch status
kubectl get pods -n allchat --watch
```

---

## 🧪 Testing

### Backend Services (kubectl)
```bash
# Check all pods
kubectl get pods -n allchat

# Check services
kubectl get services -n allchat

# View logs
kubectl logs -n allchat -l app=api-gateway -f

# Test health endpoints
kubectl port-forward -n allchat svc/api-gateway 8080:8080
curl http://localhost:8080/health
```

### Frontend (Playwright)
```bash
cd frontend
npm install
npx playwright install

# Run all E2E tests
npm run test:e2e

# Interactive UI mode
npm run test:e2e:ui
```

---

## 📝 Credentials in .env

Your `.env` file has:
- ✅ Twitch Client ID & Secret
- ✅ Twitch Bot Username & OAuth Token
- ✅ YouTube API Key
- ⚠️ JWT Secret (currently "CHANGE_ME" - recommend generating: `openssl rand -base64 32`)
- ⚠️ YouTube Client ID & Secret (needed for full YouTube integration)

**To update credentials**:
1. Edit `.env`
2. Run `./generate-vault-from-env.sh`
3. Run `./deploy.sh` to redeploy

---

## 🎯 Service Architecture

| Service | Port | Image | Status |
|---------|------|-------|--------|
| API Gateway | 8080 | ghcr.io/.../api-gateway:main | Building |
| Auth Service | 8081 | ghcr.io/.../auth-service:main | Building |
| Overlay Manager | 8082 | ghcr.io/.../overlay-manager:main | Building |
| Emote Service | 8083 | ghcr.io/.../emote-service:main | Building |
| Twitch Listener | 8085 | ghcr.io/.../twitch-listener:main | Building |
| YouTube Listener | 8086 | ghcr.io/.../youtube-listener:main | Building |
| Message Processor | 8087 | ghcr.io/.../message-processor:main | Building |
| Source Manager | 8088 | ghcr.io/.../source-manager:main | Building |
| PostgreSQL | 5432 | postgres:16-alpine | ✅ Running |
| Redis | 6379 | redis:7-alpine | ✅ Running |

---

## 🔧 Troubleshooting

### Images not pulling?
- Check if GitHub Actions completed: https://github.com/caesarakalaeii/all-chat/actions
- Verify images are public or add imagePullSecrets
- Check image names match in deployments and kustomization

### Pods crashing?
```bash
kubectl describe pod -n allchat <pod-name>
kubectl logs -n allchat <pod-name>
```

### Need to rebuild images?
Push to main branch:
```bash
git add .
git commit -m "your changes"
git push origin main
```

GitHub Actions will automatically rebuild changed services.

---

## 📚 Quick Reference

**Check build status**:
- https://github.com/caesarakalaeii/all-chat/actions

**Monitor image availability**:
```bash
cd deployments/ansible
./wait-for-images.sh
```

**Deploy/Update cluster**:
```bash
cd deployments/ansible
./deploy.sh
```

**Access services**:
```bash
kubectl port-forward -n allchat svc/api-gateway 8080:8080
curl http://localhost:8080/health
```

**View logs**:
```bash
kubectl logs -n allchat -l app=api-gateway --tail=100 -f
```

**Delete cluster**:
```bash
k3d cluster delete allchat
```

---

**Status**: All infrastructure ready, waiting for GitHub Actions builds to complete (~5-10 minutes).
