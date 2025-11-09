# Next Steps Guide

This guide provides detailed instructions for completing the remaining components of the All-Chat service.

---

## 🎯 Overview

**Completed**: 60% (3/5 services + infrastructure)
**Remaining**: 40% (2 services + frontend)

**Estimated Time**:
- Chat Listener Service: 2-3 hours
- API Gateway + WebSocket: 2-3 hours
- Svelte 5 Frontend: 4-6 hours
- **Total**: 8-12 hours

---

## 📝 Implementation Order

The remaining work should be completed in this order due to dependencies:

1. ✅ **Emote Service** (DONE)
2. → **Chat Listener Service** (reads from Emote Service)
3. → **API Gateway** (consumes Chat Listener via Redis)
4. → **Frontend** (connects to API Gateway)

---

## 1️⃣ Chat Listener Service

### Purpose
Connects to Twitch IRC, listens to chat messages from configured channels, enriches messages with emotes, and publishes to Redis pub/sub.

### Key Files to Create

#### `internal/chat-listener/core/domain/message.go`
```go
package domain

import "time"

type ChatMessage struct {
    OverlayID string
    Channel   string
    User      User
    Message   Message
    Timestamp time.Time
}

type User struct {
    Name        string
    DisplayName string
    Color       string
    Badges      []string
}

type Message struct {
    Text   string
    Emotes []Emote
}

type Emote struct {
    Code     string
    URL      string
    Provider string
    Animated bool
}
```

#### `internal/chat-listener/adapters/twitch/irc_client.go`
Use `github.com/gempir/go-twitch-irc/v4`:

```go
package twitch

import (
    "github.com/gempir/go-twitch-irc/v4"
)

type IRCClient struct {
    client *twitch.Client
    // ... fields
}

func NewIRCClient(username, oauth string) *IRCClient {
    client := twitch.NewClient(username, oauth)
    return &IRCClient{client: client}
}

func (c *IRCClient) OnMessage(callback func(message twitch.PrivateMessage)) {
    c.client.OnPrivateMessage(callback)
}

func (c *IRCClient) Join(channels ...string) {
    c.client.Join(channels...)
}
```

#### `internal/chat-listener/core/services/chat_service.go`
- Fetch active channels from database
- Connect to Twitch IRC
- On message received:
  1. Parse user info (name, badges, color)
  2. Fetch emotes for channel from Emote Service
  3. Replace emote codes in text with Emote objects
  4. Publish to Redis channel `overlay:{overlay_id}`

#### `cmd/chat-listener/main.go`
- Initialize database, Redis
- Start IRC client
- Poll database every 30 seconds for channel changes
- Gracefully shutdown (leave channels, close connections)

### Testing Chat Listener
```bash
# Set environment variables
export TWITCH_BOT_USERNAME=your_bot_username
export TWITCH_BOT_OAUTH=oauth:your_token_here

# Run service
make run-chat

# Test by:
# 1. Create an overlay with a Twitch channel
# 2. Send a message in that channel
# 3. Check Redis: redis-cli PSUBSCRIBE "overlay:*"
```

---

## 2️⃣ API Gateway with WebSocket

### Purpose
- Routes HTTP requests to backend services
- Manages WebSocket connections for overlays
- Subscribes to Redis pub/sub and broadcasts to connected clients

### Key Files to Create

#### `internal/api-gateway/adapters/websocket/hub.go`
Manages all WebSocket connections:

```go
package websocket

type Hub struct {
    clients    map[string]map[*Client]bool  // overlayID -> clients
    register   chan *Client
    unregister chan *Client
    broadcast  chan *Message
    redisClient *redis.Client
}

func NewHub(redisClient *redis.Client) *Hub {
    // ...
}

func (h *Hub) Run() {
    // Handle register/unregister
    // Subscribe to Redis overlay:* channels
    // Broadcast messages to clients
}
```

#### `internal/api-gateway/adapters/websocket/client.go`
Represents a single WebSocket connection:

```go
package websocket

import "github.com/gorilla/websocket"

type Client struct {
    hub       *Hub
    conn      *websocket.Conn
    send      chan []byte
    overlayID string
}

func (c *Client) readPump() {
    // Read messages from WebSocket (ping/pong)
}

func (c *Client) writePump() {
    // Write messages to WebSocket
}
```

#### `internal/api-gateway/adapters/api/websocket_handler.go`
HTTP handler that upgrades to WebSocket:

```go
func (h *Handler) HandleWebSocket(c *gin.Context) {
    overlayID := c.Param("id")
    token := c.Query("token")

    // Validate JWT token
    claims, err := auth.ValidateToken(token, h.jwtSecret)
    if err != nil {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }

    // Upgrade connection
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }

    // Create client and register
    client := &Client{
        hub:       h.hub,
        conn:      conn,
        send:      make(chan []byte, 256),
        overlayID: overlayID,
    }
    h.hub.register <- client

    go client.writePump()
    go client.readPump()
}
```

#### `internal/api-gateway/adapters/proxy/service_proxy.go`
Proxy requests to backend services:

```go
func (p *Proxy) ProxyToAuthService(c *gin.Context) {
    // Forward to auth-service:8081
}

func (p *Proxy) ProxyToOverlayManager(c *gin.Context) {
    // Forward to overlay-manager:8082
}
```

#### `cmd/api-gateway/main.go`
- Initialize all clients (Redis)
- Start WebSocket Hub
- Register routes:
  - `/api/v1/auth/*` → auth-service
  - `/api/v1/overlays/*` → overlay-manager
  - `/api/v1/emotes/*` → emote-service
  - `/ws/overlay/:id` → WebSocket handler
  - `/*` → Static files (frontend)

### Testing API Gateway
```bash
# Run gateway
make run-gateway

# Test WebSocket
curl http://localhost:8080/health/ready

# Test WebSocket with wscat:
npm install -g wscat
wscat -c "ws://localhost:8080/ws/overlay/OVERLAY_ID?token=JWT_TOKEN"
```

---

## 3️⃣ Svelte 5 Frontend

### Initialize Project
```bash
cd web
npm create vite@latest . -- --template svelte-ts
npm install
```

### Install Dependencies
```bash
npm install -D @sveltejs/adapter-static
npm install axios
```

### Project Structure
```
web/
├── src/
│   ├── lib/
│   │   ├── api.ts          # API client
│   │   ├── websocket.ts    # WebSocket client
│   │   └── stores.ts       # Svelte stores
│   ├── routes/
│   │   ├── +page.svelte            # Landing page
│   │   ├── dashboard/
│   │   │   └── +page.svelte        # Dashboard
│   │   ├── editor/
│   │   │   └── [id]/+page.svelte   # Overlay editor
│   │   └── overlay/
│   │       └── [id]/+page.svelte   # Overlay viewer
│   └── app.html
├── vite.config.ts
└── package.json
```

### Key Components

#### `src/lib/api.ts`
```typescript
import axios from 'axios';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
});

// Add JWT token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export const auth = {
  login: () => window.location.href = `${api.defaults.baseURL}/api/v1/auth/login`,
  getMe: () => api.get('/api/v1/auth/me'),
};

export const overlays = {
  list: () => api.get('/api/v1/overlays'),
  create: (data) => api.post('/api/v1/overlays', data),
  get: (id) => api.get(`/api/v1/overlays/${id}`),
  update: (id, data) => api.put(`/api/v1/overlays/${id}`, data),
  delete: (id) => api.delete(`/api/v1/overlays/${id}`),
  getConfig: (id) => api.get(`/api/v1/overlays/${id}/config`),
  updateConfig: (id, data) => api.put(`/api/v1/overlays/${id}/config`, data),
};
```

#### `src/lib/websocket.ts`
```typescript
export class OverlayWebSocket {
  private ws: WebSocket | null = null;

  connect(overlayId: string, token: string) {
    const url = `ws://localhost:8080/ws/overlay/${overlayId}?token=${token}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => console.log('WebSocket connected');
    this.ws.onclose = () => console.log('WebSocket disconnected');
    this.ws.onerror = (error) => console.error('WebSocket error:', error);

    return this.ws;
  }

  onMessage(callback: (data: any) => void) {
    if (this.ws) {
      this.ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        callback(data);
      };
    }
  }

  disconnect() {
    this.ws?.close();
  }
}
```

#### `src/routes/+page.svelte` (Landing Page)
```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/api';

  let user = $state(null);

  onMount(async () => {
    try {
      const response = await auth.getMe();
      user = response.data.user;
    } catch (error) {
      // Not logged in
    }
  });
</script>

<main>
  <h1>All-Chat Overlay Service</h1>
  {#if user}
    <p>Welcome, {user.display_name}!</p>
    <a href="/dashboard">Go to Dashboard</a>
  {:else}
    <button onclick={() => auth.login()}>Login with Twitch</button>
  {/if}
</main>
```

#### `src/routes/dashboard/+page.svelte`
```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { overlays } from '$lib/api';

  let overlayList = $state([]);

  onMount(async () => {
    const response = await overlays.list();
    overlayList = response.data.overlays;
  });

  async function createOverlay() {
    const name = prompt('Overlay name:');
    const channel = prompt('Twitch channel:');
    await overlays.create({ name, twitch_channel: channel });
    // Refresh list
  }
</script>

<h1>My Overlays</h1>
<button onclick={createOverlay}>Create New Overlay</button>

{#each overlayList as overlay}
  <div class="overlay-card">
    <h2>{overlay.name}</h2>
    <a href="/editor/{overlay.id}">Edit</a>
    <a href="/overlay/{overlay.id}">View</a>
  </div>
{/each}
```

#### `src/routes/overlay/[id]/+page.svelte` (Overlay Viewer)
```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { OverlayWebSocket } from '$lib/websocket';

  let messages = $state([]);
  const overlayId = $page.params.id;
  const token = localStorage.getItem('access_token');

  onMount(() => {
    const ws = new OverlayWebSocket();
    ws.connect(overlayId, token);

    ws.onMessage((data) => {
      messages = [...messages, data].slice(-50); // Keep last 50

      // Remove after duration
      setTimeout(() => {
        messages = messages.filter(m => m !== data);
      }, data.duration * 1000);
    });

    return () => ws.disconnect();
  });
</script>

<div class="overlay">
  {#each messages as message}
    <div class="message" style="color: {message.user.color}">
      <span class="username">{message.user.display_name}:</span>
      <span class="text">
        {#each message.message.text.split(' ') as word}
          {#if message.message.emotes.find(e => e.code === word)}
            <img src={message.message.emotes.find(e => e.code === word).url} alt={word} />
          {:else}
            {word}
          {/if}
        {/each}
      </span>
    </div>
  {/each}
</div>

<style>
  .overlay {
    background: transparent;
    font-family: Arial, sans-serif;
  }

  .message {
    animation: slideIn 0.3s ease-out;
    margin: 4px 0;
  }

  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  img {
    height: 28px;
    vertical-align: middle;
  }
</style>
```

### Build Frontend
```bash
cd web
npm run dev      # Development server (http://localhost:5173)
npm run build    # Production build
```

---

## 🧪 End-to-End Testing

1. **Start all services**:
   ```bash
   make docker-up
   ```

2. **Login**:
   - Visit http://localhost:8080
   - Click "Login with Twitch"
   - Authorize application

3. **Create Overlay**:
   - Go to Dashboard
   - Create new overlay with Twitch channel name

4. **Configure Overlay**:
   - Edit overlay
   - Enable 7TV, BTTV emotes
   - Set display settings

5. **View Overlay**:
   - Open overlay viewer URL
   - Send message in Twitch chat
   - See message appear with emotes

6. **Add to OBS**:
   - Add Browser Source
   - URL: `http://localhost:8080/overlay/{id}?token={jwt}`
   - Width: 1920, Height: 1080
   - Background transparent

---

## 📊 Project Status After Completion

- ✅ **5/5 Microservices**: Fully functional
- ✅ **Frontend**: Complete user interface
- ✅ **Infrastructure**: Docker + Kubernetes ready
- ✅ **Documentation**: Comprehensive guides
- ✅ **Testing**: End-to-end flow working
- → **Production Ready**: Deploy to cloud!

---

## 🚀 Deployment to Production

### Prerequisites
- Kubernetes cluster (GKE, EKS, AKS, etc.)
- Domain name
- SSL certificate (Let's Encrypt)

### Steps
1. Build and push Docker images
2. Create Kubernetes secrets with production values
3. Apply all manifests
4. Configure DNS
5. Set up monitoring (optional)

See `docs/DEPLOYMENT.md` for detailed instructions (create this file if needed).

---

## 💡 Tips

- **Start small**: Get each service working individually before integration
- **Test frequently**: Use Postman or curl to test APIs as you build
- **Log everything**: Add logger calls to debug issues
- **Use hot reload**: Gin has live reload, Vite has HMR
- **Check health endpoints**: All services have `/health/ready`

---

## 🆘 Troubleshooting

### Chat Listener not receiving messages
- Check Twitch OAuth token is valid
- Verify channel name is correct (lowercase)
- Check if channel has overlays configured

### WebSocket not connecting
- Verify JWT token is valid
- Check CORS settings
- Inspect browser console for errors

### Messages not appearing in overlay
- Check Redis pub/sub with: `redis-cli PSUBSCRIBE "overlay:*"`
- Verify WebSocket is connected
- Check browser console for JavaScript errors

---

## 📚 Additional Resources

- **Twitch IRC**: https://dev.twitch.tv/docs/irc
- **go-twitch-irc**: https://github.com/gempir/go-twitch-irc
- **WebSocket RFC**: https://tools.ietf.org/html/rfc6455
- **Svelte 5 Docs**: https://svelte-5-preview.vercel.app/

---

Good luck with the implementation! You're 60% done and the hardest architectural decisions are already made. The remaining work is straightforward integration code. 🚀
