# All-Chat - Cloud-Native Streaming Overlay Service

A scalable, microservices-based platform for aggregating and displaying chat messages from **multiple live streaming platforms** (Twitch, YouTube, Kick, TikTok) on streaming overlays with support for 7TV, BTTV, and FFZ emotes.

**Multi-Source Chat Aggregation**: Create overlays that combine messages from multiple streaming platforms simultaneously. Perfect for streamers who multistream or want unified chat displays across platforms.

## 🏗️ Architecture

This project follows cloud-native principles with a microservices architecture:

- **API Gateway**: Entry point, WebSocket management, routes requests
- **Auth Service**: Handles Twitch OAuth and JWT token management
- **Overlay Manager**: CRUD operations for overlays and configurations
- **Emote Service**: Fetches and caches emotes from 7TV, BTTV, FFZ
- **Chat Listener**: Connects to multiple live streaming platforms (Twitch, YouTube, Kick, TikTok) and publishes normalized messages to Redis

### Tech Stack

- **Backend**: Go 1.23+ with Gin framework
- **Database**: PostgreSQL 16
- **Cache/Messaging**: Redis 7
- **Frontend**: Svelte 5 with Runes
- **Deployment**: Docker + Kubernetes
- **Real-time**: WebSockets + Redis Pub/Sub

## 🚀 Quick Start

### Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose
- PostgreSQL 16 (or use Docker)
- Redis 7 (or use Docker)
- Twitch Developer Account (for OAuth credentials)

### 1. Clone and Setup

```bash
git clone https://github.com/caesar/all-chat.git
cd all-chat

# Copy environment template
cp .env.example .env

# Edit .env with your Twitch credentials
nano .env
```

### 2. Get Twitch Credentials

1. Go to https://dev.twitch.tv/console/apps
2. Create a new application
3. Set OAuth Redirect URL to: `http://localhost:8080/api/v1/auth/callback`
4. Copy Client ID and Client Secret to `.env`
5. Get IRC OAuth token from https://twitchapps.com/tmi/

### 3. Start with Docker Compose (Recommended)

```bash
# Start all services
make docker-up

# View logs
make docker-logs

# Stop all services
make docker-down
```

Services will be available at:
- API Gateway: http://localhost:8080
- Auth Service: http://localhost:8081
- Overlay Manager: http://localhost:8082
- Emote Service: http://localhost:8083
- PostgreSQL: localhost:5432
- Redis: localhost:6379

### 4. Manual Setup (Development)

```bash
# Install dependencies
make deps

# Start PostgreSQL and Redis
docker-compose up postgres redis -d

# Run migrations
make migrate-up

# Build all services
make build

# Run services (in separate terminals)
make run-auth
make run-overlay
make run-emote
make run-chat
make run-gateway
```

## 📁 Project Structure

```
all-chat/
├── cmd/                    # Application entry points
│   ├── api-gateway/
│   ├── auth-service/
│   ├── chat-listener/
│   ├── emote-service/
│   └── overlay-manager/
├── internal/               # Private application code
│   ├── api-gateway/
│   ├── auth-service/
│   ├── chat-listener/
│   ├── emote-service/
│   └── overlay-manager/
│       ├── adapters/      # External implementations
│       │   ├── api/       # HTTP handlers
│       │   └── repository/# Database
│       └── core/          # Business logic
│           ├── domain/    # Entities
│           ├── ports/     # Interfaces
│           └── services/  # Use cases
├── pkg/                   # Shared libraries
│   ├── auth/             # JWT utilities
│   ├── database/         # Database helpers
│   ├── redis/            # Redis client
│   ├── logger/           # Structured logging
│   └── middleware/       # HTTP middleware
├── web/                  # Frontend (Svelte 5)
├── deployments/          # Deployment configs
│   ├── docker/          # Dockerfiles
│   ├── k8s/             # Kubernetes manifests
│   └── docker-compose.yml
├── migrations/           # Database migrations
└── docs/                 # Documentation
```

## 🔧 Development

### Available Make Commands

```bash
make help              # Show all available commands
make build             # Build all services
make test              # Run tests
make test-coverage     # Run tests with coverage
make fmt               # Format code
make lint              # Run linter
make clean             # Clean build artifacts
```

### Building Individual Services

```bash
make build-auth        # Build auth service
make build-overlay     # Build overlay manager
make build-emote       # Build emote service
make build-chat        # Build chat listener
make build-api-gateway # Build API gateway
```

### Running Individual Services

```bash
make run-auth          # Run auth service
make run-overlay       # Run overlay manager
make run-emote         # Run emote service
make run-chat          # Run chat listener
make run-gateway       # Run API gateway
```

## 🌐 API Endpoints

### Authentication

```
POST   /api/v1/auth/login      # Redirect to Twitch OAuth
GET    /api/v1/auth/callback   # OAuth callback
POST   /api/v1/auth/refresh    # Refresh access token
POST   /api/v1/auth/logout     # Logout
GET    /api/v1/auth/me         # Get current user
```

### Overlays

```
GET    /api/v1/overlays         # List user's overlays
POST   /api/v1/overlays         # Create overlay
GET    /api/v1/overlays/:id     # Get overlay
PUT    /api/v1/overlays/:id     # Update overlay
DELETE /api/v1/overlays/:id     # Delete overlay

GET    /api/v1/overlays/:id/config  # Get config
PUT    /api/v1/overlays/:id/config  # Update config
```

### WebSocket

```
WS     /ws/overlay/:id?token=JWT    # Overlay WebSocket connection
```

## 🎨 Frontend

The Svelte 5 frontend includes:

- **Landing Page**: Marketing and login
- **Dashboard**: Manage overlays
- **Overlay Editor**: Configure settings
- **Overlay Viewer**: Embedded in OBS

```bash
cd web
npm install
npm run dev     # Development server
npm run build   # Production build
```

## ☸️ Kubernetes Deployment

### Deploy to Kubernetes

```bash
# Create namespace
kubectl apply -f deployments/k8s/namespace.yaml

# Create secrets (update values first!)
kubectl create secret generic app-secrets \
  --from-literal=database-password=YOUR_PASSWORD \
  --from-literal=jwt-secret=YOUR_JWT_SECRET \
  --from-literal=twitch-client-id=YOUR_TWITCH_CLIENT_ID \
  --from-literal=twitch-client-secret=YOUR_TWITCH_CLIENT_SECRET \
  -n all-chat

# Apply config maps
kubectl apply -f deployments/k8s/configmaps/

# Deploy services
kubectl apply -f deployments/k8s/auth-service/
kubectl apply -f deployments/k8s/api-gateway/
# ... other services

# Apply ingress
kubectl apply -f deployments/k8s/ingress/
```

## 📊 Data Flow

```
Twitch IRC
    ↓
Chat Listener Service
    ↓
Redis Pub/Sub (channel: overlay:{id})
    ↓
WebSocket Server (API Gateway)
    ↓
Browser (Overlay Viewer)
```

## 🔒 Security

- JWT-based authentication with short-lived access tokens (15 min)
- Refresh tokens stored securely (7 days)
- OAuth tokens encrypted at rest
- CORS protection
- Rate limiting (TODO)
- Input validation and sanitization

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package tests
go test -v ./internal/auth-service/...
```

## 📈 Monitoring

Health check endpoints:

- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe (checks DB/Redis)

Prometheus metrics (TODO):
- `GET /metrics`

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) - Twitch IRC client
- [Gin Framework](https://gin-gonic.com/) - HTTP web framework
- Twitch, 7TV, BTTV, FFZ communities

## 📮 Support

For issues and questions:
- Open an issue on GitHub
- Check existing documentation in `/docs`

## 🗺️ Roadmap

- [x] Core microservices architecture
- [x] Twitch OAuth authentication
- [x] Overlay CRUD operations
- [ ] Emote service implementation
- [ ] Chat listener with Twitch IRC
- [ ] WebSocket overlay service
- [ ] Svelte 5 frontend
- [ ] Multi-source chat support (YouTube, Kick, TikTok)
- [ ] Custom emote animations
- [ ] Advanced filtering options
- [ ] Analytics dashboard
