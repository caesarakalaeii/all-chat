# All-Chat: Kubernetes Deployment Architecture

**Version:** 1.0
**Last Updated:** 2025-11-11
**Related Docs**: [Approved Architecture](./APPROVED_ARCHITECTURE.md), [Scaling & Performance](./SCALING_PERFORMANCE.md)

---

## Table of Contents

1. [Introduction](#introduction)
2. [Cluster Architecture](#cluster-architecture)
3. [Namespace Strategy](#namespace-strategy)
4. [Deployment Specifications](#deployment-specifications)
5. [Service Networking](#service-networking)
6. [Ingress & Load Balancing](#ingress--load-balancing)
7. [ConfigMaps & Secrets](#configmaps--secrets)
8. [Persistent Storage](#persistent-storage)
9. [Resource Quotas & Limits](#resource-quotas--limits)
10. [Health Checks & Probes](#health-checks--probes)
11. [Horizontal Pod Autoscaling](#horizontal-pod-autoscaling)
12. [Rolling Updates & Rollbacks](#rolling-updates--rollbacks)
13. [Monitoring & Logging](#monitoring--logging)
14. [Deployment Commands](#deployment-commands)

---

## Introduction

All-Chat is designed as a **Kubernetes-native** application, leveraging Kubernetes features for:
- **Auto-scaling**: HPA based on CPU/memory/custom metrics
- **Self-healing**: Automatic pod restarts and health checks
- **Rolling updates**: Zero-downtime deployments
- **Service discovery**: Internal DNS for service-to-service communication
- **Configuration management**: ConfigMaps and Secrets
- **Observability**: Prometheus metrics and structured logging

### Deployment Context

**Phase 1 - Early Stages (Current)**:
- **Platform**: Hetzner VPS with Kubernetes
- **Storage**: Hetzner Cloud Volumes (hcloud-csi driver)
- **Backups**: Local volume snapshots (7-day retention)
- **Cost**: ~€50-100/month

**Phase 2+ - Production Scale**:
- **Storage**: Migrate to Hetzner Object Storage or S3-compatible
- **Backups**: Object storage with 30-day retention + PITR
- **Multi-region**: Consider geographic distribution

This document provides complete Kubernetes deployment specifications for both phases.

---

## Cluster Architecture

### Logical Architecture

```mermaid
graph TB
    subgraph "External Traffic"
        INTERNET[Internet]
        LB[Cloud Load Balancer<br/>AWS ALB / GCP LB / Azure LB]
    end

    subgraph "Kubernetes Cluster - all-chat namespace"
        subgraph "Ingress Layer"
            INGRESS[Ingress Controller<br/>NGINX / Traefik]
        end

        subgraph "Application Layer"
            GW_SVC[api-gateway-service<br/>ClusterIP]
            AUTH_SVC[auth-service<br/>ClusterIP]
            OVM_SVC[overlay-service<br/>ClusterIP]
            EMOTE_SVC[emote-service<br/>ClusterIP]
        end

        subgraph "Control Plane"
            SC_SVC[source-manager-service<br/>ClusterIP]
        end

        subgraph "Data Layer"
            TWL_SVC[twitch-listener-service<br/>Headless]
            YTL_SVC[youtube-listener-service<br/>Headless]
            MP_SVC[message-processor-service<br/>Headless]
        end

        subgraph "Infrastructure"
            PG_SVC[postgresql-service<br/>StatefulSet]
            RD_SVC[redis-service<br/>StatefulSet]
        end
    end

    INTERNET --> LB
    LB --> INGRESS
    INGRESS --> GW_SVC
    GW_SVC --> AUTH_SVC
    GW_SVC --> OVM_SVC
    GW_SVC --> EMOTE_SVC

    SC_SVC --> RD_SVC
    SC_SVC --> PG_SVC

    TWL_SVC --> RD_SVC
    YTL_SVC --> RD_SVC
    MP_SVC --> RD_SVC
    MP_SVC --> EMOTE_SVC

    style GW_SVC fill:#ffe1f5
    style INGRESS fill:#e1f5ff
```

### Node Groups (Recommended)

| Node Group | Purpose | Instance Type | Min | Max | Taints/Labels |
|------------|---------|---------------|-----|-----|---------------|
| **core** | Application services | t3.large (2 vCPU, 8GB) | 2 | 10 | `workload=application` |
| **data** | PostgreSQL, Redis | r5.large (2 vCPU, 16GB) | 2 | 5 | `workload=stateful` |
| **listeners** | Platform listeners | t3.medium (2 vCPU, 4GB) | 2 | 10 | `workload=listeners` |

---

## Namespace Strategy

### Namespace: `all-chat`

All All-Chat services run in the `all-chat` namespace for isolation and resource management.

```yaml
# deployments/k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: all-chat
  labels:
    app: all-chat
    environment: production
```

### Resource Quota

```yaml
# deployments/k8s/resource-quota.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: all-chat-quota
  namespace: all-chat
spec:
  hard:
    requests.cpu: "50"          # Total CPU requests
    requests.memory: "100Gi"    # Total memory requests
    limits.cpu: "100"           # Total CPU limits
    limits.memory: "200Gi"      # Total memory limits
    pods: "100"                 # Max pods
    services: "20"              # Max services
    persistentvolumeclaims: "10"
```

### Network Policy (Optional)

```yaml
# deployments/k8s/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: all-chat-network-policy
  namespace: all-chat
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: all-chat
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              name: all-chat
    - to:  # Allow external API calls
        - podSelector: {}
      ports:
        - protocol: TCP
          port: 443
        - protocol: TCP
          port: 80
```

---

## Deployment Specifications

### API Gateway

**File**: `deployments/k8s/api-gateway/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
  namespace: all-chat
  labels:
    app: api-gateway
    version: v1
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: api-gateway
  template:
    metadata:
      labels:
        app: api-gateway
        version: v1
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - api-gateway
                topologyKey: kubernetes.io/hostname
      containers:
        - name: api-gateway
          image: allchat/api-gateway:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          env:
            - name: PORT
              value: "8080"
            - name: LOG_LEVEL
              value: "info"
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
            - name: REDIS_PORT
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_port
            - name: AUTH_SERVICE_URL
              value: "http://auth-service:8081"
            - name: OVERLAY_SERVICE_URL
              value: "http://overlay-service:8082"
            - name: EMOTE_SERVICE_URL
              value: "http://emote-service:8083"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: jwt_secret
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 3
            failureThreshold: 2
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 15"]
---
apiVersion: v1
kind: Service
metadata:
  name: api-gateway
  namespace: all-chat
  labels:
    app: api-gateway
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: 8080
      protocol: TCP
  selector:
    app: api-gateway
```

---

### Auth Service

**File**: `deployments/k8s/auth-service/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  namespace: all-chat
spec:
  replicas: 2
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
    spec:
      containers:
        - name: auth-service
          image: allchat/auth-service:latest
          ports:
            - containerPort: 8081
          env:
            - name: PORT
              value: "8081"
            - name: DATABASE_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: database_host
            - name: DATABASE_PORT
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: database_port
            - name: DATABASE_NAME
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: database_name
            - name: DATABASE_USER
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: database_user
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: database_password
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
            - name: TWITCH_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: twitch_client_id
            - name: TWITCH_CLIENT_SECRET
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: twitch_client_secret
            - name: TWITCH_REDIRECT_URL
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: twitch_redirect_url
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: jwt_secret
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "256Mi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8081
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: auth-service
  namespace: all-chat
spec:
  type: ClusterIP
  ports:
    - port: 8081
      targetPort: 8081
  selector:
    app: auth-service
```

---

### Source Manager (Leader Election)

**File**: `deployments/k8s/source-manager/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: source-manager
  namespace: all-chat
spec:
  replicas: 2  # Only one will be leader
  selector:
    matchLabels:
      app: source-manager
  template:
    metadata:
      labels:
        app: source-manager
    spec:
      containers:
        - name: source-manager
          image: allchat/source-manager:latest
          ports:
            - containerPort: 8084
          env:
            - name: PORT
              value: "8084"
            - name: INSTANCE_ID
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name  # Unique pod name
            - name: DATABASE_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: database_host
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
            - name: LEADER_ELECTION_KEY
              value: "leader:source-manager"
            - name: LEADER_TTL
              value: "30"  # seconds
            - name: POLL_INTERVAL
              value: "10"  # seconds
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "200m"
              memory: "256Mi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8084
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8084
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: source-manager
  namespace: all-chat
spec:
  type: ClusterIP
  ports:
    - port: 8084
      targetPort: 8084
  selector:
    app: source-manager
```

---

### Twitch Listener

**File**: `deployments/k8s/twitch-listener/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: twitch-listener
  namespace: all-chat
spec:
  replicas: 2
  selector:
    matchLabels:
      app: twitch-listener
  template:
    metadata:
      labels:
        app: twitch-listener
        workload: listeners
    spec:
      nodeSelector:
        workload: listeners  # Schedule on listener-specific nodes
      containers:
        - name: twitch-listener
          image: allchat/twitch-listener:latest
          ports:
            - containerPort: 8085
          env:
            - name: PORT
              value: "8085"
            - name: INSTANCE_ID
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
            - name: TWITCH_BOT_USERNAME
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: twitch_bot_username
            - name: TWITCH_BOT_OAUTH
              valueFrom:
                secretKeyRef:
                  name: all-chat-secrets
                  key: twitch_bot_oauth
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8085
            initialDelaySeconds: 15
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8085
            initialDelaySeconds: 10
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: twitch-listener
  namespace: all-chat
spec:
  clusterIP: None  # Headless service
  ports:
    - port: 8085
      targetPort: 8085
  selector:
    app: twitch-listener
```

---

### Message Processor

**File**: `deployments/k8s/message-processor/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: message-processor
  namespace: all-chat
spec:
  replicas: 3  # Consumer group members
  selector:
    matchLabels:
      app: message-processor
  template:
    metadata:
      labels:
        app: message-processor
    spec:
      containers:
        - name: message-processor
          image: allchat/message-processor:latest
          ports:
            - containerPort: 8087
          env:
            - name: PORT
              value: "8087"
            - name: INSTANCE_ID
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
            - name: EMOTE_SERVICE_URL
              value: "http://emote-service:8083"
            - name: CONSUMER_GROUP
              value: "processor-group"
            - name: BATCH_SIZE
              value: "10"
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "1Gi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8087
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8087
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: message-processor
  namespace: all-chat
spec:
  clusterIP: None  # Headless service
  ports:
    - port: 8087
      targetPort: 8087
  selector:
    app: message-processor
```

---

## Service Networking

### Service Types

| Service | Type | Port | Purpose |
|---------|------|------|---------|
| `api-gateway` | ClusterIP | 8080 | Internal HTTP routing, exposed via Ingress |
| `auth-service` | ClusterIP | 8081 | Internal service-to-service communication |
| `overlay-service` | ClusterIP | 8082 | Internal service-to-service communication |
| `emote-service` | ClusterIP | 8083 | Internal service-to-service communication |
| `source-manager` | ClusterIP | 8084 | Health checks only |
| `twitch-listener` | Headless (ClusterIP: None) | 8085 | Individual pod targeting |
| `youtube-listener` | Headless | 8086 | Individual pod targeting |
| `message-processor` | Headless | 8087 | Consumer group members |
| `postgresql` | ClusterIP | 5432 | Database access |
| `redis` | ClusterIP | 6379 | Cache & messaging |

### DNS Resolution

All services are discoverable via Kubernetes DNS:

```
<service-name>.<namespace>.svc.cluster.local
```

Examples:
- `auth-service.all-chat.svc.cluster.local` → Resolves to ClusterIP
- `twitch-listener.all-chat.svc.cluster.local` → Resolves to individual pod IPs (headless)

---

## Ingress & Load Balancing

### Ingress Configuration

**File**: `deployments/k8s/ingress.yaml`

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: all-chat-ingress
  namespace: all-chat
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/websocket-services: "api-gateway"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - api.allchat.io
        - overlays.allchat.io
      secretName: allchat-tls
  rules:
    - host: api.allchat.io
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api-gateway
                port:
                  number: 8080
    - host: overlays.allchat.io
      http:
        paths:
          - path: /ws
            pathType: Prefix
            backend:
              service:
                name: api-gateway
                port:
                  number: 8080
```

### WebSocket Support

Ingress NGINX requires special annotations for WebSocket:
- `nginx.ingress.kubernetes.io/websocket-services`: List of services supporting WebSocket
- `nginx.ingress.kubernetes.io/proxy-read-timeout`: Keep connection alive (3600s = 1 hour)
- `nginx.ingress.kubernetes.io/proxy-send-timeout`: Send timeout

---

## ConfigMaps & Secrets

### ConfigMap: `all-chat-config`

**File**: `deployments/k8s/configmaps/all-chat-config.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: all-chat-config
  namespace: all-chat
data:
  # Database
  database_host: "postgresql.all-chat.svc.cluster.local"
  database_port: "5432"
  database_name: "allchat"

  # Redis
  redis_host: "redis.all-chat.svc.cluster.local"
  redis_port: "6379"

  # Service URLs
  auth_service_url: "http://auth-service:8081"
  overlay_service_url: "http://overlay-service:8082"
  emote_service_url: "http://emote-service:8083"

  # Twitch OAuth
  twitch_redirect_url: "https://api.allchat.io/api/v1/auth/callback"

  # Logging
  log_level: "info"
```

### Secret: `all-chat-secrets`

**File**: Create manually or via CI/CD

```bash
kubectl create secret generic all-chat-secrets \
  --namespace=all-chat \
  --from-literal=jwt_secret='your-secret-key' \
  --from-literal=database_user='allchat' \
  --from-literal=database_password='secure-password' \
  --from-literal=twitch_client_id='your-client-id' \
  --from-literal=twitch_client_secret='your-client-secret' \
  --from-literal=twitch_bot_username='your-bot-username' \
  --from-literal=twitch_bot_oauth='oauth:your-token'
```

**IMPORTANT**: Never commit secrets to Git. Use:
- Sealed Secrets (bitnami-labs/sealed-secrets)
- External Secrets Operator
- Vault integration
- Cloud-provider secret managers (AWS Secrets Manager, GCP Secret Manager)

---

## Persistent Storage

### CloudNativePG Cluster

**File**: `deployments/k8s/postgresql/cnpg-cluster.yaml`

All-Chat uses CloudNativePG operator for PostgreSQL instead of manual StatefulSets, providing:
- Automated failover (~30 seconds)
- Point-in-time recovery (PITR)
- Automated backups to S3/compatible storage
- Built-in connection pooling (PgBouncer)
- Read replica support

#### Prerequisites

```bash
# Install CloudNativePG operator
kubectl apply -f \
  https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml

# Verify operator is running
kubectl get deployment -n cnpg-system cnpg-controller-manager
```

#### CNPG Cluster Configuration

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-pg
  namespace: all-chat
spec:
  instances: 3  # 1 primary + 2 replicas

  # PostgreSQL configuration
  postgresql:
    parameters:
      max_connections: "200"
      shared_buffers: "256MB"
      effective_cache_size: "1GB"
      maintenance_work_mem: "64MB"
      checkpoint_completion_target: "0.9"
      wal_buffers: "16MB"
      default_statistics_target: "100"
      random_page_cost: "1.1"
      effective_io_concurrency: "200"
      work_mem: "2621kB"
      min_wal_size: "1GB"
      max_wal_size: "4GB"

  # Bootstrap from scratch (or restore from backup)
  bootstrap:
    initdb:
      database: allchat
      owner: allchat
      secret:
        name: allchat-pg-credentials

  # Storage configuration (Hetzner Cloud Volumes)
  storage:
    storageClass: hcloud-volumes  # Hetzner CSI driver
    size: 50Gi

  # Resource limits
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2000m"
      memory: "4Gi"

  # Automated backup to local volume (Phase 1 - VPS)
  # NOTE: For production, migrate to S3/object storage
  backup:
    volumeSnapshot:
      className: hcloud-volumes
    retentionPolicy: "7d"  # Keep 7 days locally due to space constraints

  # For production with object storage, uncomment:
  # backup:
  #   barmanObjectStore:
  #     destinationPath: s3://allchat-backups/postgresql
  #     endpointURL: https://fsn1.your-region.hetznerobjects.com
  #     s3Credentials:
  #       accessKeyId:
  #         name: s3-credentials
  #         key: ACCESS_KEY_ID
  #       secretAccessKey:
  #         name: s3-credentials
  #         key: SECRET_ACCESS_KEY
  #     wal:
  #       compression: gzip
  #     data:
  #       compression: gzip
  #   retentionPolicy: "30d"

  # Connection pooling (optional but recommended)
  connectionPooler:
    enabled: true
    type: pgbouncer
    instances: 3
    pgbouncer:
      poolMode: transaction
      parameters:
        max_client_conn: "1000"
        default_pool_size: "25"

  # Monitoring
  monitoring:
    enablePodMonitor: true

  # Node affinity (spread across nodes)
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                cnpg.io/cluster: allchat-pg
            topologyKey: kubernetes.io/hostname

---
# Database credentials secret
apiVersion: v1
kind: Secret
metadata:
  name: allchat-pg-credentials
  namespace: all-chat
type: kubernetes.io/basic-auth
stringData:
  username: allchat
  password: CHANGE_ME_SECURE_PASSWORD

# NOTE: S3 credentials not needed for Phase 1 (local backups only)
# For production migration to object storage, create:
# ---
# apiVersion: v1
# kind: Secret
# metadata:
#   name: s3-credentials
#   namespace: all-chat
# type: Opaque
# stringData:
#   ACCESS_KEY_ID: YOUR_S3_ACCESS_KEY
#   SECRET_ACCESS_KEY: YOUR_S3_SECRET_KEY
```

#### Connecting Applications to CNPG

Applications should use the following connection strings:

```bash
# Primary (read-write)
postgresql://allchat:password@allchat-pg-rw.all-chat.svc.cluster.local:5432/allchat

# Read replicas (read-only)
postgresql://allchat:password@allchat-pg-ro.all-chat.svc.cluster.local:5432/allchat

# Connection pooler (if enabled)
postgresql://allchat:password@allchat-pg-pooler-rw.all-chat.svc.cluster.local:5432/allchat
```

**CNPG automatically creates these services:**
- `allchat-pg-rw`: Primary instance (read-write)
- `allchat-pg-ro`: Read replicas (read-only, load balanced)
- `allchat-pg-r`: All instances (for admin tasks)
- `allchat-pg-pooler-rw`: PgBouncer pooler (if enabled)

### Redis StatefulSet with AOF Persistence

**File**: `deployments/k8s/redis/statefulset.yaml`

**Important**: Redis is configured with AOF (Append-Only File) persistence to prevent data loss on crashes. This provides ~1-second RPO (Recovery Point Objective).

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: all-chat
spec:
  serviceName: redis
  replicas: 1  # Single instance for Phase 1, migrate to Redis Cluster in Phase 5
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      nodeSelector:
        workload: stateful
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          command:
            - redis-server
          args:
            # Memory management
            - --maxmemory
            - "768mb"  # Leave headroom for AOF rewrite
            - --maxmemory-policy
            - allkeys-lru

            # AOF persistence (CRITICAL for durability)
            - --appendonly
            - "yes"
            - --appendfsync
            - everysec  # Fsync every second (balance performance/durability)
            - --auto-aof-rewrite-percentage
            - "100"
            - --auto-aof-rewrite-min-size
            - 64mb

            # Performance tuning
            - --tcp-backlog
            - "511"
            - --timeout
            - "300"
            - --tcp-keepalive
            - "60"

            # Logging
            - --loglevel
            - notice

            # Save snapshots (backup to AOF)
            - --save
            - "900 1"  # After 900s if ≥1 key changed
            - --save
            - "300 10"  # After 300s if ≥10 keys changed
            - --save
            - "60 10000"  # After 60s if ≥10000 keys changed

          volumeMounts:
            - name: redis-storage
              mountPath: /data
          resources:
            requests:
              cpu: "100m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "1Gi"
          livenessProbe:
            tcpSocket:
              port: 6379
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command:
                - redis-cli
                - ping
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: redis-storage
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: hcloud-volumes  # Hetzner Cloud Volumes
        resources:
          requests:
            storage: 20Gi  # Increased for AOF + RDB files
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: all-chat
spec:
  type: ClusterIP
  ports:
    - port: 6379
      targetPort: 6379
  selector:
    app: redis
```

---

## Resource Quotas & Limits

### Resource Requests vs Limits

| Resource Type | Requests (Guaranteed) | Limits (Maximum) | Purpose |
|---------------|----------------------|------------------|---------|
| **CPU** | Pod scheduling decision | Throttled if exceeded | CPU time slicing |
| **Memory** | Pod scheduling decision | **OOM kill** if exceeded | Memory allocation |

### Best Practices

1. **Always set requests**: Ensures pod gets minimum resources
2. **Set limits conservatively**: Prevents resource hogging
3. **Request = Limit for critical services**: Guaranteed QoS (PostgreSQL, Redis)
4. **Request < Limit for burstable**: Allows scaling up (API Gateway)

---

## Health Checks & Probes

### Liveness Probe

**Purpose**: Determine if the container is alive. If fails → restart container.

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 10  # Wait before first check
  periodSeconds: 10        # Check every 10 seconds
  timeoutSeconds: 5        # Timeout for request
  failureThreshold: 3      # Restart after 3 failures
```

**Implementation** (`/health/live`):
```go
func (h *HealthHandler) HandleLiveness(c *gin.Context) {
    c.JSON(200, gin.H{"status": "alive"})
}
```

### Readiness Probe

**Purpose**: Determine if the container is ready to serve traffic. If fails → remove from service endpoints.

```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

**Implementation** (`/health/ready`):
```go
func (h *HealthHandler) HandleReadiness(c *gin.Context) {
    // Check database connection
    if err := h.db.Ping(c.Request.Context()); err != nil {
        c.JSON(503, gin.H{"status": "not ready", "error": "database unreachable"})
        return
    }

    // Check Redis connection
    if err := h.redis.Ping(c.Request.Context()).Err(); err != nil {
        c.JSON(503, gin.H{"status": "not ready", "error": "redis unreachable"})
        return
    }

    c.JSON(200, gin.H{"status": "ready"})
}
```

---

## Horizontal Pod Autoscaling

### HPA: API Gateway

**File**: `deployments/k8s/api-gateway/hpa.yaml`

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-gateway-hpa
  namespace: all-chat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70  # Scale up if CPU > 70%
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80  # Scale up if memory > 80%
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
        - type: Percent
          value: 50  # Scale up by 50% of current replicas
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300  # Wait 5 minutes before scaling down
      policies:
        - type: Pods
          value: 1  # Scale down 1 pod at a time
          periodSeconds: 60
```

### HPA: Message Processor

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: message-processor-hpa
  namespace: all-chat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: message-processor
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
```

---

## Rolling Updates & Rollbacks

### Rolling Update Strategy

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1          # Max 1 extra pod during update
      maxUnavailable: 0    # Zero downtime
```

**Process**:
1. Create 1 new pod (v2)
2. Wait for v2 pod to be ready
3. Terminate 1 old pod (v1)
4. Repeat until all pods replaced

### Deployment Commands

```bash
# Apply updated deployment
kubectl apply -f deployments/k8s/api-gateway/deployment.yaml

# Check rollout status
kubectl rollout status deployment/api-gateway -n all-chat

# View rollout history
kubectl rollout history deployment/api-gateway -n all-chat

# Rollback to previous version
kubectl rollout undo deployment/api-gateway -n all-chat

# Rollback to specific revision
kubectl rollout undo deployment/api-gateway --to-revision=3 -n all-chat

# Pause rollout (for canary testing)
kubectl rollout pause deployment/api-gateway -n all-chat

# Resume rollout
kubectl rollout resume deployment/api-gateway -n all-chat
```

---

## Monitoring & Logging

### Prometheus Metrics (Planned)

Each service exposes `/metrics` endpoint:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

**Example metrics**:
- `http_requests_total{method="GET",path="/api/v1/overlays",status="200"}`
- `http_request_duration_seconds{method="GET",path="/api/v1/overlays"}`
- `websocket_connections_active{overlay_id="abc-123"}`
- `redis_stream_messages_pending{stream="raw-messages",consumer_group="processor-group"}`

### Structured Logging

All services use **Zap** for structured JSON logging:

```json
{
  "level": "info",
  "ts": "2025-11-11T12:34:56.789Z",
  "caller": "services/overlay_service.go:123",
  "msg": "overlay created",
  "user_id": "uuid-123",
  "overlay_id": "uuid-456",
  "name": "My Overlay"
}
```

**Log Aggregation** (planned):
- Fluent Bit / Fluentd → Elasticsearch / Loki
- Accessible via Kibana / Grafana

---

## Deployment Commands

### Initial Deployment

```bash
# Create namespace
kubectl apply -f deployments/k8s/namespace.yaml

# Create ConfigMap
kubectl apply -f deployments/k8s/configmaps/all-chat-config.yaml

# Create Secrets (use your own values!)
kubectl create secret generic all-chat-secrets \
  --namespace=all-chat \
  --from-literal=jwt_secret='your-secret' \
  --from-literal=database_user='allchat' \
  --from-literal=database_password='password' \
  --from-literal=twitch_client_id='...' \
  --from-literal=twitch_client_secret='...' \
  --from-literal=twitch_bot_username='...' \
  --from-literal=twitch_bot_oauth='oauth:...'

# Deploy PostgreSQL
kubectl apply -f deployments/k8s/postgresql/

# Deploy Redis
kubectl apply -f deployments/k8s/redis/

# Deploy application services
kubectl apply -f deployments/k8s/auth-service/
kubectl apply -f deployments/k8s/overlay-manager/
kubectl apply -f deployments/k8s/emote-service/
kubectl apply -f deployments/k8s/source-manager/
kubectl apply -f deployments/k8s/twitch-listener/
kubectl apply -f deployments/k8s/youtube-listener/
kubectl apply -f deployments/k8s/message-processor/
kubectl apply -f deployments/k8s/api-gateway/

# Deploy Ingress
kubectl apply -f deployments/k8s/ingress.yaml

# Verify all pods are running
kubectl get pods -n all-chat

# Check logs
kubectl logs -f deployment/api-gateway -n all-chat
```

### Update Deployment

```bash
# Update image tag
kubectl set image deployment/api-gateway api-gateway=allchat/api-gateway:v1.2.0 -n all-chat

# Or apply updated YAML
kubectl apply -f deployments/k8s/api-gateway/deployment.yaml

# Watch rollout
kubectl rollout status deployment/api-gateway -n all-chat
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment/message-processor --replicas=5 -n all-chat

# View HPA status
kubectl get hpa -n all-chat

# Describe HPA
kubectl describe hpa api-gateway-hpa -n all-chat
```

---

## Summary

This document provides comprehensive Kubernetes deployment specifications for All-Chat:

1. **Cluster Architecture**: Multi-layer architecture with node groups
2. **Deployments**: Complete YAML specs for all services
3. **Service Networking**: ClusterIP and Headless services
4. **Ingress**: NGINX Ingress with WebSocket support
5. **ConfigMaps & Secrets**: Configuration management
6. **Persistent Storage**: StatefulSets for PostgreSQL and Redis
7. **Health Checks**: Liveness and readiness probes
8. **Autoscaling**: HPA based on CPU/memory
9. **Rolling Updates**: Zero-downtime deployments

**Next Steps**:
- [SCALING_PERFORMANCE.md](./SCALING_PERFORMANCE.md) - Detailed scaling strategies
- [OBSERVABILITY_MONITORING.md](./OBSERVABILITY_MONITORING.md) - Monitoring and alerting setup
- [SECURITY_ARCHITECTURE.md](./SECURITY_ARCHITECTURE.md) - Security hardening

---

**Document Maintainers**: DevOps Team
**Last Review**: 2025-11-11
