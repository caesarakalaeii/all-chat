# TikTok Listener - CI/CD Integration

**Date**: November 15, 2025
**Status**: ✅ Complete

---

## Summary

Successfully integrated the TikTok listener service into the existing GitHub Actions CI/CD pipeline and Makefile build system.

## Changes Made

### 1. GitHub Actions Workflow (`.github/workflows/build-and-push.yml`)

**Added to `detect-changes` job outputs:**
```yaml
tiktok-listener: ${{ steps.changes.outputs.tiktok-listener }}
```

**Added to path filters:**
```yaml
tiktok-listener:
  - 'services/tiktok-listener/**'
```

**Added to build matrix:**
```yaml
- name: tiktok-listener
  changed: ${{ needs.detect-changes.outputs.tiktok-listener }}
  path: services/tiktok-listener
```

### 2. Makefile Updates

**Added to help menu:**
```makefile
make build-tiktok        - Build tiktok-listener (Node.js)
```

**Added to `deps` target:**
```makefile
cd services/tiktok-listener && npm install
```

**Added to `build` target:**
```makefile
@$(MAKE) build-tiktok
```

**New `build-tiktok` target:**
```makefile
build-tiktok:
	@echo "Building tiktok-listener..."
	cd services/tiktok-listener && npm install && npm run build
```

### 3. Dockerfile Updates

Updated `services/tiktok-listener/Dockerfile` to support building from repository root (matching the pattern used by GitHub Actions):

**Before:**
```dockerfile
COPY package*.json ./
COPY tsconfig.json ./
COPY src/ ./src/
```

**After:**
```dockerfile
COPY services/tiktok-listener/package*.json ./
COPY services/tiktok-listener/tsconfig.json ./
COPY services/tiktok-listener/src/ ./src/
```

---

## How It Works

### GitHub Actions

When code is pushed to any branch:

1. **Change Detection**: Detects if `services/tiktok-listener/**` files changed
2. **Conditional Build**: Only builds if tiktok-listener files changed (or shared files changed for Go services)
3. **Docker Build**: Uses Docker Buildx with root context
4. **Image Push**: Pushes to GitHub Container Registry (ghcr.io)

**Image naming:**
```
ghcr.io/caesarakalaeii/allchat-tiktok-listener:latest
ghcr.io/caesarakalaeii/allchat-tiktok-listener:main-<sha>
ghcr.io/caesarakalaeii/allchat-tiktok-listener:<branch>
```

### Makefile

**Build locally:**
```bash
# Build just TikTok listener
make build-tiktok

# Build all services (including TikTok)
make build

# Install dependencies
make deps
```

**Output:**
- TypeScript compiled to JavaScript in `services/tiktok-listener/dist/`
- Docker image can be built with: `docker build -f services/tiktok-listener/Dockerfile .`

---

## Testing the CI/CD

### Local Testing

```bash
# Test Dockerfile build from root
docker build -f services/tiktok-listener/Dockerfile -t tiktok-listener:test .

# Test Makefile
make build-tiktok

# Verify output
ls -la services/tiktok-listener/dist/
```

### GitHub Actions Testing

1. **Push to any branch** with changes to `services/tiktok-listener/`
2. **Check Actions tab** in GitHub repository
3. **Verify build succeeds** for tiktok-listener job
4. **Check GHCR** for pushed image

**View workflow run:**
```
https://github.com/caesarakalaeii/all-chat/actions
```

**View published images:**
```
https://github.com/caesarakalaeii?tab=packages
```

---

## Image Usage

### Pull from Registry

```bash
# Pull latest from main branch
docker pull ghcr.io/caesarakalaeii/allchat-tiktok-listener:latest

# Pull specific branch
docker pull ghcr.io/caesarakalaeii/allchat-tiktok-listener:main

# Pull specific commit
docker pull ghcr.io/caesarakalaeii/allchat-tiktok-listener:main-abc1234
```

### Use in Kubernetes

Update `deployments/k8s/base/tiktok-listener/deployment.yaml`:

```yaml
spec:
  containers:
  - name: tiktok-listener
    image: ghcr.io/caesarakalaeii/allchat-tiktok-listener:latest
    imagePullPolicy: Always  # Or IfNotPresent for tagged versions
```

### Use in Docker Compose

```yaml
services:
  tiktok-listener:
    image: ghcr.io/caesarakalaeii/allchat-tiktok-listener:latest
    environment:
      - REDIS_HOST=redis
      - DATABASE_HOST=postgres
    ports:
      - "8089:8089"
```

---

## Differences from Go Services

### Node.js vs Go

| Aspect | Go Services | TikTok Listener (Node.js) |
|--------|-------------|---------------------------|
| **Language** | Go 1.23+ | Node.js 20 + TypeScript |
| **Package Manager** | go mod | npm |
| **Build Output** | Single binary in `bin/` | Compiled JS in `dist/` |
| **Base Image** | golang:1.23-alpine | node:20-alpine |
| **Dependencies** | Go modules (shared/) | NPM packages |
| **Build Time** | ~30-60s | ~20-40s |
| **Image Size** | ~20-50MB | ~150-200MB |

### Why Node.js?

The TikTok listener uses Node.js because:
1. **Unofficial Library**: `tiktok-live-connector` is only available as an NPM package
2. **No Go Port**: No mature Go implementation of TikTok WebSocket protocol
3. **Temporary Solution**: Will be replaced when TikTok releases official API
4. **Works Well**: Node.js handles WebSocket connections efficiently

---

## CI/CD Workflow Diagram

```
┌─────────────────────────────────────────────────┐
│  Developer pushes code                          │
│  - services/tiktok-listener/src/index.ts        │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│  GitHub Actions: detect-changes                 │
│  - Uses dorny/paths-filter@v3                   │
│  - Checks if tiktok-listener/** changed         │
└────────────────┬────────────────────────────────┘
                 │
                 ▼ (if changed)
┌─────────────────────────────────────────────────┐
│  GitHub Actions: build-and-push                 │
│  1. Checkout repository                         │
│  2. Set up Docker Buildx                        │
│  3. Login to GHCR                               │
│  4. Extract metadata (tags)                     │
│  5. Build Docker image (context: root)          │
│  6. Push to ghcr.io                             │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│  Image Available in GHCR                        │
│  ghcr.io/caesarakalaeii/allchat-tiktok-listener │
│  - :latest (main branch)                        │
│  - :main-abc1234 (commit SHA)                   │
│  - :feature-branch (other branches)             │
└─────────────────────────────────────────────────┘
```

---

## Troubleshooting

### Build Fails: "npm: not found"

The GitHub Actions workflow uses a container that has Docker but not Node.js. This is okay because Node.js is installed inside the Docker build (multi-stage build).

**Solution**: No action needed, this is expected behavior.

### Build Fails: "Cannot find module"

Check that all COPY paths in Dockerfile start with `services/tiktok-listener/`:

```dockerfile
# Correct
COPY services/tiktok-listener/package*.json ./

# Incorrect
COPY package*.json ./
```

### Local Build Works, CI Fails

Ensure you're testing from the repository root:

```bash
# Test from root (matching CI)
cd /path/to/all-chat
docker build -f services/tiktok-listener/Dockerfile .

# NOT from service directory
cd services/tiktok-listener
docker build -f Dockerfile .  # This won't match CI
```

### Image Pull Permission Denied

GitHub Container Registry requires authentication:

```bash
# Login with GitHub token
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# For Kubernetes, create image pull secret
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=USERNAME \
  --docker-password=$GITHUB_TOKEN
```

---

## Future Improvements

### Potential Enhancements

1. **Multi-arch Builds**: Add ARM64 support for Apple Silicon
   ```yaml
   platforms: linux/amd64,linux/arm64
   ```

2. **Separate Test Job**: Add Node.js testing step
   ```yaml
   - name: Run tests
     run: |
       cd services/tiktok-listener
       npm install
       npm test
   ```

3. **Dependency Caching**: Cache node_modules for faster builds
   ```yaml
   - uses: actions/cache@v3
     with:
       path: services/tiktok-listener/node_modules
       key: ${{ runner.os }}-node-${{ hashFiles('**/package-lock.json') }}
   ```

4. **Vulnerability Scanning**: Add Trivy or Snyk scanning
   ```yaml
   - name: Run Trivy vulnerability scanner
     uses: aquasecurity/trivy-action@master
   ```

### When Official API Arrives

Once TikTok releases an official Live Chat API:

1. **Rewrite in Go**: Match other listener services
2. **Remove Node.js dependencies**: Simplify build process
3. **Add to test matrix**: Include in Go test suite
4. **Update CI/CD**: Remove Node.js-specific handling

---

## Verification Checklist

- [x] TikTok listener added to GitHub Actions detect-changes
- [x] TikTok listener added to build matrix
- [x] Dockerfile updated for root context
- [x] Makefile targets created
- [x] Build tested locally
- [x] Documentation updated
- [ ] First CI build completed successfully
- [ ] Image pulled and tested
- [ ] Kubernetes deployment updated with new image

---

## Resources

- **GitHub Actions Workflow**: `.github/workflows/build-and-push.yml`
- **Makefile**: `Makefile`
- **Dockerfile**: `services/tiktok-listener/Dockerfile`
- **Package Config**: `services/tiktok-listener/package.json`
- **Service Code**: `services/tiktok-listener/src/index.ts`

---

**Status**: Ready for deployment
**Next**: Push changes and verify CI build succeeds
