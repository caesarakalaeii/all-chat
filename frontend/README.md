# All-Chat Frontend

Modern web application for managing multi-platform chat overlays, built with **Next.js 16**, **React 19**, and **TypeScript**.

## Why This Stack?

- **Next.js 16** - Modern React framework with SSR, optimized for LLMs to understand
- **TypeScript** - Full type safety, easier for LLMs to work with
- **Zustand** - Simple state management, minimal boilerplate
- **TailwindCSS** - Utility-first CSS, highly composable
- **Clear Structure** - Organized folders, extensive comments

## Features

- 🔐 **Twitch OAuth** - Secure authentication
- 📊 **Dashboard** - Manage multiple overlays
- 🌐 **Multi-Platform Sources** - Add Twitch and YouTube channels
- ⚡ **Real-Time Preview** - WebSocket-powered live chat display
- 🎨 **Customization** - Configure appearance and emote providers (see [CSS Guide](../docs/CSS_CUSTOMIZATION.md))
- 📋 **OBS Integration** - Copy overlay URL for OBS Browser Source

## Project Structure

```
frontend/
├── src/
│   ├── app/                          # Next.js App Router
│   │   ├── layout.tsx                # Root layout
│   │   ├── page.tsx                  # Landing page (/)
│   │   ├── globals.css               # Global styles
│   │   ├── auth/
│   │   │   └── callback/
│   │   │       └── page.tsx          # OAuth callback handler
│   │   ├── dashboard/
│   │   │   └── page.tsx              # User dashboard
│   │   └── overlays/
│   │       ├── new/
│   │       │   └── page.tsx          # Create overlay
│   │       └── [id]/
│   │           ├── page.tsx          # Edit overlay
│   │           └── preview/
│   │               └── page.tsx      # Live preview
│   └── lib/
│       ├── api/                      # API clients
│       │   ├── client.ts             # Base HTTP client
│       │   ├── auth.ts               # Auth API
│       │   ├── overlays.ts           # Overlays API
│       │   └── websocket.ts          # WebSocket client
│       ├── stores/                   # Zustand stores
│       │   ├── auth-store.ts         # Auth state
│       │   └── overlay-store.ts      # Overlay state
│       └── types/                    # TypeScript types
│           ├── auth.ts               # Auth types
│           ├── overlay.ts            # Overlay types
│           └── message.ts            # Message types
├── public/                           # Static assets
├── package.json
├── next.config.js                    # Next.js configuration
├── tailwind.config.js                # TailwindCSS configuration
├── tsconfig.json                     # TypeScript configuration
├── Dockerfile                        # Production Docker build
└── README.md                         # This file
```

## Development

### Prerequisites

- Node.js 20+
- npm (comes with Node.js)
- Backend services running (see main repository)

### Install Dependencies

```bash
cd frontend
npm install
```

### Run Development Server

```bash
npm run dev
```

Open http://localhost:3000 in your browser.

**Note**: The backend API Gateway must be running on port 8080 for full functionality.

### Run Backend Services

In another terminal:

```bash
cd ../deployments
docker-compose up -d
```

### Type Checking

```bash
npm run type-check
```

### Linting

```bash
npm run lint
```

## Testing

### Unit Tests

```bash
npm run test
```

### Watch Mode

```bash
npm run test:watch
```

## Building

### Production Build

```bash
npm run build
```

### Start Production Server

```bash
npm start
```

### Docker Build

```bash
# Build image
docker build -t allchat-frontend .

# Run container
docker run -p 3000:3000 \
  -e NEXT_PUBLIC_API_URL=http://localhost:8080 \
  -e NEXT_PUBLIC_WS_URL=ws://localhost:8080 \
  allchat-frontend
```

## Same-origin requirement (cookie auth)

Streamer/admin authentication uses **httpOnly cookies** (`access_token` +
`refresh_token`, `SameSite=Lax`, host-only path). This requires the frontend
and the API gateway to be served from the **same origin** — otherwise the
browser will not send the cookies and login silently fails with no obvious
error.

### Production

In production, both are served behind a single ingress that terminates TLS
and routes `/api/*` to the gateway. For example, `allch.at` serves the Next.js
app and proxies `/api/*` to the api-gateway. `NEXT_PUBLIC_API_URL` must resolve
to the **same origin** as the frontend window (`window.location.origin`).

### Development

The default dev setup (frontend `:3000`, gateway `:8080`) is **cross-origin** —
cookies will not be sent. To use cookie auth locally either:

1. Serve both from one origin (e.g. configure Next.js rewrites to proxy
   `/api/*` to `localhost:8080`), **or**
2. Use the token-based dev fallback (set `NEXT_PUBLIC_API_URL` to the gateway
   and rely on the `Authorization: Bearer` backward-compat path).

A dev-mode console warning is emitted by `src/lib/api/client.ts` when the API
base origin differs from `window.location.origin`.

> If a cross-origin deployment is ever needed: `credentials: 'include'` +
> `SameSite=None` + strict CORS allowlist + revisiting CSRF are required (out
> of scope for the current rollout).

See: [H3 cookie-auth design spec](../docs/pi/specs/2026-06-23-h3-cookie-auth-design.md).

## Environment Variables

Create `.env.local` for development:

```bash
# ⚠️ Cookie auth requires same-origin — see "Same-origin requirement" above.
# For same-origin dev: omit this or set it to window.location.origin.
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080
```

For production (must be same-origin as the frontend):

```bash
# Same origin as the frontend (ingress proxies /api/* to the gateway)
NEXT_PUBLIC_API_URL=https://allch.at
NEXT_PUBLIC_WS_URL=wss://allch.at
```

## Code Organization

### API Layer (`src/lib/api/`)

All API communication is centralized here:

- **client.ts** - Base HTTP client with auth and error handling
- **auth.ts** - Authentication endpoints
- **overlays.ts** - Overlay CRUD operations
- **websocket.ts** - Real-time message streaming

### State Management (`src/lib/stores/`)

Using Zustand for global state:

- **auth-store.ts** - User authentication and token management
- **overlay-store.ts** - Overlay list and operations

### Types (`src/lib/types/`)

Complete TypeScript definitions matching backend API:

- **auth.ts** - User, AuthState, responses
- **overlay.ts** - Overlay, Config, ChatSource, requests
- **message.ts** - ChatMessage, UserInfo, Emote, WebSocket messages

### Pages (`src/app/`)

Next.js App Router pages:

- **Landing** (`/`) - Marketing page with login button
- **OAuth Callback** (`/auth/callback`) - Handles Twitch OAuth redirect
- **Dashboard** (`/dashboard`) - List of user's overlays
- **Create Overlay** (`/overlays/new`) - Form to create new overlay
- **Edit Overlay** (`/overlays/[id]`) - Manage sources for an overlay
- **Preview** (`/overlays/[id]/preview`) - Real-time chat preview with WebSocket

## LLM-Friendly Features

This codebase is designed to be easy for LLMs to understand and modify:

1. **Extensive Comments** - Every file has header comments explaining its purpose
2. **Clear Naming** - Descriptive function and variable names
3. **Type Safety** - Full TypeScript coverage
4. **Organized Structure** - Logical folder hierarchy
5. **Single Responsibility** - Each file/function has one clear purpose
6. **Standard Patterns** - Follows Next.js best practices
7. **No Magic** - Explicit over implicit code

## Key Concepts

### Client vs Server Components

- **Server Components** (default) - Rendered on server, no browser APIs
- **Client Components** (`'use client'`) - Interactive, use hooks, access browser APIs

In this app:

- All pages are **Client Components** (they use hooks, state, browser APIs)
- We use `'use client'` directive at the top of each page

### State Management with Zustand

```typescript
// Create store
const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  setUser: (user) => set({ user }),
}))

// Use in component
function MyComponent() {
  const { user, setUser } = useAuthStore()
  // ...
}
```

### API Client Pattern

```typescript
// Make authenticated API calls
const overlay = await overlaysApi.get(overlayId)
const user = await authApi.getMe()
```

The client automatically:

- Adds JWT token from localStorage
- Handles 401 errors (logout)
- Parses JSON responses
- Throws typed errors

### WebSocket Pattern

```typescript
// Create client
const wsClient = new WebSocketClient()

// Connect
wsClient.connect(overlayId, token)

// Listen for messages
wsClient.onMessage((message) => {
  console.log(message)
})

// Cleanup
wsClient.disconnect()
```

## Deployment

### Development

```bash
npm run dev
```

### Production

```bash
# Build
npm run build

# Start
npm start
```

### Docker

```bash
docker build -t allchat-frontend .
docker run -p 3000:3000 allchat-frontend
```

### Kubernetes

See `caesar-deployment/all-chat/frontend-deployment.yaml`

## Common Tasks

### Add a New Page

1. Create file in `src/app/new-page/page.tsx`
2. Add `'use client'` if using hooks or browser APIs
3. Import and use stores/API clients as needed

### Add a New API Endpoint

1. Add function to appropriate API file (`src/lib/api/`)
2. Define types in `src/lib/types/`
3. Use in components with error handling

### Add a New Store

1. Create file in `src/lib/stores/`
2. Define interface and create store with Zustand
3. Import and use in components

## Troubleshooting

### "Module not found" errors

```bash
npm install
```

### TypeScript errors

```bash
npm run type-check
```

### Port 3000 already in use

```bash
# Use different port
npm run dev -- -p 3001
```

### WebSocket not connecting

- Check backend is running on port 8080
- Check JWT token is valid
- Check browser console for errors

## Documentation

### Frontend Customization

- **[CSS Customization Guide](../docs/CSS_CUSTOMIZATION.md)** - Complete CSS reference for overlay styling
- **[Theme Gallery](../docs/overlay-themes/README.md)** - Browse and create custom themes

### Development

- **[Main README](../README.md)** - Project overview and quick start
- **[Developer Guide](../CLAUDE.md)** - Architecture and development principles
- **[Getting Started](../GETTING_STARTED.md)** - Navigate the codebase

## Support

For issues:

1. Check Next.js console output
2. Check browser console (F12)
3. Verify backend services are running
4. Check CHECKPOINT.md in main repository
5. For CSS issues, see [CSS Customization Guide](../docs/CSS_CUSTOMIZATION.md)

## License

See repository root LICENSE file.

---

**Built with**: Next.js 16 • React 19 • TypeScript • TailwindCSS • Zustand
**Optimized for**: LLM collaboration and rapid development
