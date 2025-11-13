# API Gateway

The API Gateway serves as the single entry point for all HTTP requests to the All-Chat platform. It provides reverse proxy functionality, JWT authentication, CORS handling, and health aggregation for backend services.

## Features

- **Reverse Proxy**: Routes requests to appropriate backend services
- **JWT Authentication**: Validates tokens for protected routes
- **CORS**: Configurable cross-origin resource sharing
- **Request Logging**: Structured logging with latency tracking
- **Health Aggregation**: Monitors health of all backend services
- **Graceful Shutdown**: Allows in-flight requests to complete

## Architecture

```
┌─────────┐
│ Client  │
└────┬────┘
     │ HTTP/HTTPS
     ▼
┌──────────────────────┐
│   API Gateway        │
│   (Port 8080)        │
│   ┌──────────────┐   │
│   │ Middleware   │   │
│   │ - CORS       │   │
│   │ - Logging    │   │
│   │ - JWT Auth   │   │
│   └──────────────┘   │
│   ┌──────────────┐   │
│   │ Proxy        │   │
│   │ Handler      │   │
│   └──────────────┘   │
└──────┬───────────────┘
       │
       ├─── /api/v1/auth/*      → auth-service:8081
       ├─── /api/v1/overlays/*  → overlay-manager:8082
       └─── /api/v1/emotes/*    → emote-service:8083
```

## Environment Variables

### Required

```bash
# JWT secret (must match auth-service)
JWT_SECRET=your-secret-key-here

# Backend service URLs
AUTH_SERVICE_URL=http://localhost:8081
OVERLAY_SERVICE_URL=http://localhost:8082
EMOTE_SERVICE_URL=http://localhost:8083
```

### Optional

```bash
# Server configuration
PORT=8080                               # API Gateway port
LOG_LEVEL=info                          # debug, info, warn, error

# CORS configuration
CORS_ORIGIN=http://localhost:3000       # Allowed origins (comma-separated or *)
```

## API Routes

### Health Check

```
GET /health
```

Returns aggregated health status of all backend services.

**Response**:
```json
{
  "status": "healthy",
  "services": {
    "auth-service": {
      "status": "up",
      "latency_ms": 5
    },
    "overlay-manager": {
      "status": "up",
      "latency_ms": 8
    },
    "emote-service": {
      "status": "up",
      "latency_ms": 3
    }
  },
  "timestamp": "2025-11-13T10:00:00Z"
}
```

**Status Values**:
- `healthy`: All services are up
- `degraded`: Some services are down
- `unhealthy`: All services are down

### Auth Service Routes

| Method | Path | Auth Required | Description |
|--------|------|---------------|-------------|
| GET | `/api/v1/auth/login` | No | Start OAuth flow |
| GET | `/api/v1/auth/callback` | No | OAuth callback |
| POST | `/api/v1/auth/refresh` | No | Refresh token |
| GET | `/api/v1/auth/me` | Yes | Get current user |
| POST | `/api/v1/auth/logout` | Yes | Logout |

### Overlay Manager Routes

All overlay routes require JWT authentication.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/overlays` | List overlays |
| POST | `/api/v1/overlays` | Create overlay |
| GET | `/api/v1/overlays/:id` | Get overlay |
| PUT | `/api/v1/overlays/:id` | Update overlay |
| DELETE | `/api/v1/overlays/:id` | Delete overlay |
| GET | `/api/v1/overlays/:id/config` | Get overlay config |
| PUT | `/api/v1/overlays/:id/config` | Update overlay config |
| GET | `/api/v1/overlays/:id/sources` | List chat sources |
| POST | `/api/v1/overlays/:id/sources` | Add chat source |
| PUT | `/api/v1/overlays/:id/sources/:source_id` | Update chat source |
| DELETE | `/api/v1/overlays/:id/sources/:source_id` | Remove chat source |

### Emote Service Routes

| Method | Path | Auth Required | Description |
|--------|------|---------------|-------------|
| GET | `/api/v1/emotes/channel/:channel` | No | All emotes for channel |
| GET | `/api/v1/emotes/7tv/:channel` | No | 7TV emotes only |
| GET | `/api/v1/emotes/bttv/:channel` | No | BTTV emotes only |
| GET | `/api/v1/emotes/ffz/:channel` | No | FFZ emotes only |

## Authentication

Protected routes require a JWT token in the `Authorization` header:

```bash
Authorization: Bearer <token>
```

**Example**:
```bash
curl -H "Authorization: Bearer eyJhbGci..." \
     http://localhost:8080/api/v1/overlays
```

## Running Locally

### Prerequisites

- Go 1.25+
- Backend services running (auth-service, overlay-manager, emote-service)

### Development

```bash
# Set environment variables
export JWT_SECRET=your-secret-key
export AUTH_SERVICE_URL=http://localhost:8081
export OVERLAY_SERVICE_URL=http://localhost:8082
export EMOTE_SERVICE_URL=http://localhost:8083

# Run the service
go run ./cmd

# Or build and run
go build -o api-gateway ./cmd
./api-gateway
```

### With Docker Compose

```bash
cd deployments
docker-compose up api-gateway
```

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./handlers -v
go test ./middleware -v
go test ./models -v
```

## Performance

- **Proxy Overhead**: < 10ms (p95)
- **Health Check**: < 50ms (all services)
- **Concurrent Requests**: Handles 1000s/sec

## Error Handling

### Client Errors (4xx)

- `401 Unauthorized`: Missing or invalid JWT token
- `404 Not Found`: No backend service for path

### Server Errors (5xx)

- `500 Internal Server Error`: Gateway internal error
- `502 Bad Gateway`: Backend service unavailable
- `504 Gateway Timeout`: Backend service timeout

## Monitoring

The gateway logs all requests with:
- Method and path
- Response status code
- Latency
- Client IP
- User agent

**Example Log**:
```json
{
  "level": "info",
  "ts": "2025-11-13T10:00:00Z",
  "service": "api-gateway",
  "method": "GET",
  "path": "/api/v1/overlays",
  "status": 200,
  "latency": "15ms",
  "client_ip": "192.168.1.1"
}
```

## Development Tips

### Adding a New Backend Service

1. Update `models/service_config.go`:
   ```go
   registry.Services["new-service"] = &ServiceConfig{
       Name:       "new-service",
       BaseURL:    getEnvOrDefault("NEW_SERVICE_URL", "http://localhost:8084"),
       HealthPath: "/health/live",
       PathPrefix: "/api/v1/new",
   }
   ```

2. Add routes in `cmd/main.go`:
   ```go
   publicAPI.GET("/new/*path", proxyHandler.ForwardRequest)
   ```

3. Update environment variables and docker-compose.yml

### Debugging

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Test health endpoint
curl http://localhost:8080/health | jq

# Test proxy with verbose output
curl -v http://localhost:8080/api/v1/emotes/channel/xqc
```

## Security Considerations

- JWT secrets must match across auth-service and API Gateway
- Use HTTPS in production (add TLS termination)
- Configure CORS for production domains only
- Consider rate limiting for public endpoints
- Implement API key authentication for service-to-service calls

## License

Copyright © 2025 All-Chat. All rights reserved.
