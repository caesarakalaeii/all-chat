# Phase 5: Frontend & User Experience - Detailed Plan

**Version**: 1.0
**Created**: 2025-11-13
**Duration**: 3-4 weeks (Jan 12 - Feb 8)
**Priority**: P1 (Critical for User Adoption)

---

## Overview

Phase 5 builds the user-facing web application, making All-Chat accessible to streamers without requiring API knowledge. The frontend enables users to authenticate, manage overlays, configure sources, and preview their chat in real-time.

**Key Deliverables**:
1. **Svelte 5 Application** - Modern, reactive web app
2. **Authentication Pages** - Twitch + YouTube OAuth flows
3. **Overlay Management** - CRUD operations for overlays and sources
4. **Real-Time Preview** - WebSocket-powered overlay preview
5. **OBS Integration** - Browser source URL generation

---

## Technology Stack

### Core Framework
- **Svelte 5** - Latest version with runes and improved reactivity
- **SvelteKit** - Full-stack framework with routing and SSR
- **Vite** - Fast build tool and dev server
- **TypeScript** - Type safety

### Styling & UI
- **TailwindCSS** - Utility-first CSS framework
- **shadcn-svelte** - Accessible component library
- **Lucide Icons** - Icon library

### State Management
- **Svelte Stores** - Built-in reactive stores
- **Context API** - For auth and global state

### API & WebSocket
- **Fetch API** - HTTP requests to API Gateway
- **Native WebSocket** - Real-time message streaming

### Testing
- **Vitest** - Unit and integration tests
- **Playwright** - E2E browser tests
- **Testing Library Svelte** - Component testing

### Build & Deploy
- **Vite** - Production builds
- **Docker** - Containerization
- **Nginx** - Static file serving

---

## Architecture

```
┌─────────────────────────────────────────────┐
│         Frontend (SvelteKit)                │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Public Routes                     │    │
│  │  - / (landing)                     │    │
│  │  - /auth/callback                  │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Protected Routes (require JWT)    │    │
│  │  - /dashboard                      │    │
│  │  - /overlays/new                   │    │
│  │  - /overlays/:id                   │    │
│  │  - /overlays/:id/preview           │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  API Client (fetch)                │    │
│  │  - Auth endpoints                  │    │
│  │  - Overlay CRUD                    │    │
│  │  - Source management               │    │
│  └─────────────┬──────────────────────┘    │
│                │                            │
│  ┌─────────────▼──────────────────────┐    │
│  │  WebSocket Client                  │    │
│  │  - Connect to overlay              │    │
│  │  - Receive messages                │    │
│  │  - Display in preview              │    │
│  └────────────────────────────────────┘    │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
         ┌──────────────────┐
         │   API Gateway    │
         │   (Port 8080)    │
         │  - HTTP REST API │
         │  - WebSocket     │
         └──────────────────┘
```

---

## Directory Structure

```
all-chat/
├── frontend/
│   ├── src/
│   │   ├── routes/
│   │   │   ├── +page.svelte                    # Landing page
│   │   │   ├── +layout.svelte                  # Root layout
│   │   │   ├── auth/
│   │   │   │   └── callback/
│   │   │   │       └── +page.svelte            # OAuth callback
│   │   │   ├── dashboard/
│   │   │   │   └── +page.svelte                # Dashboard
│   │   │   └── overlays/
│   │   │       ├── new/
│   │   │       │   └── +page.svelte            # Create overlay
│   │   │       └── [id]/
│   │   │           ├── +page.svelte            # Edit overlay
│   │   │           └── preview/
│   │   │               └── +page.svelte        # Live preview
│   │   ├── lib/
│   │   │   ├── api/
│   │   │   │   ├── client.ts                   # API client
│   │   │   │   ├── auth.ts                     # Auth API calls
│   │   │   │   ├── overlays.ts                 # Overlay API calls
│   │   │   │   └── websocket.ts                # WebSocket client
│   │   │   ├── stores/
│   │   │   │   ├── auth.ts                     # Auth store
│   │   │   │   ├── overlays.ts                 # Overlay store
│   │   │   │   └── messages.ts                 # Message store
│   │   │   ├── components/
│   │   │   │   ├── ui/                         # shadcn-svelte components
│   │   │   │   ├── Navbar.svelte
│   │   │   │   ├── OverlayCard.svelte
│   │   │   │   ├── SourceList.svelte
│   │   │   │   ├── ChatMessage.svelte
│   │   │   │   └── EmoteImage.svelte
│   │   │   ├── types/
│   │   │   │   ├── auth.ts                     # Auth types
│   │   │   │   ├── overlay.ts                  # Overlay types
│   │   │   │   └── message.ts                  # Message types
│   │   │   └── utils/
│   │   │       ├── jwt.ts                      # JWT utilities
│   │   │       └── clipboard.ts                # Clipboard helper
│   │   └── app.css                             # Global styles
│   ├── static/
│   │   └── favicon.png
│   ├── tests/
│   │   ├── unit/
│   │   └── e2e/
│   ├── package.json
│   ├── vite.config.ts
│   ├── svelte.config.js
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── playwright.config.ts
│   ├── Dockerfile
│   └── README.md
```

---

## Phase 5 Tasks

### Task 1: Svelte 5 Framework Setup (2-3 days)

#### 1.1 Initialize SvelteKit Project

```bash
cd /home/moersener/Hobby/all-chat

# Create SvelteKit project
npm create svelte@latest frontend

# Choose options:
# - Which template? → Skeleton project
# - Add type checking? → Yes, using TypeScript
# - Add Prettier? → Yes
# - Add ESLint? → Yes
# - Add Vitest? → Yes
# - Add Playwright? → Yes

cd frontend
npm install
```

#### 1.2 Install Dependencies

```bash
# TailwindCSS
npx svelte-add@latest tailwindcss
npm install

# shadcn-svelte (component library)
npx shadcn-svelte@latest init

# Additional dependencies
npm install --save \
  @sveltejs/adapter-static \
  lucide-svelte \
  clsx \
  tailwind-merge \
  date-fns

# Dev dependencies
npm install --save-dev \
  @testing-library/svelte \
  @vitest/ui
```

#### 1.3 Configure Vite

```typescript
// vite.config.ts
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  test: {
    include: ['src/**/*.{test,spec}.{js,ts}'],
    globals: true,
    environment: 'jsdom'
  }
});
```

#### 1.4 Configure TailwindCSS

```javascript
// tailwind.config.js
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        twitch: '#9146FF',
        youtube: '#FF0000',
        discord: '#5865F2'
      }
    }
  },
  plugins: []
};
```

#### 1.5 Environment Configuration

```bash
# .env
PUBLIC_API_URL=http://localhost:8080
PUBLIC_WS_URL=ws://localhost:8080
```

---

### Task 2: Authentication & Layout (5-7 days)

#### 2.1 Auth Store (Svelte Store)

```typescript
// src/lib/stores/auth.ts
import { writable } from 'svelte/store';
import type { User } from '$lib/types/auth';

interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    token: null,
    loading: true
  });

  return {
    subscribe,
    setToken: (token: string) => {
      localStorage.setItem('jwt_token', token);
      update(state => ({ ...state, token }));
    },
    setUser: (user: User) => {
      update(state => ({ ...state, user, loading: false }));
    },
    logout: () => {
      localStorage.removeItem('jwt_token');
      set({ user: null, token: null, loading: false });
    },
    init: async () => {
      const token = localStorage.getItem('jwt_token');
      if (token) {
        // Verify token and fetch user
        // Implementation in auth.ts
      } else {
        update(state => ({ ...state, loading: false }));
      }
    }
  };
}

export const auth = createAuthStore();
```

#### 2.2 API Client

```typescript
// src/lib/api/client.ts
import { auth } from '$lib/stores/auth';
import { get } from 'svelte/store';

const API_URL = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

export class ApiClient {
  private async fetch(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<Response> {
    const authState = get(auth);
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers
    };

    if (authState.token) {
      headers['Authorization'] = `Bearer ${authState.token}`;
    }

    const response = await fetch(`${API_URL}${endpoint}`, {
      ...options,
      headers
    });

    if (response.status === 401) {
      // Token expired, logout
      auth.logout();
      window.location.href = '/';
    }

    return response;
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint);
    return response.json();
  }

  async post<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(data)
    });
    return response.json();
  }

  async put<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data)
    });
    return response.json();
  }

  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new ApiClient();
```

#### 2.3 Landing Page

```svelte
<!-- src/routes/+page.svelte -->
<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Github, Twitch, Youtube } from 'lucide-svelte';

  const API_URL = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

  function loginWithTwitch() {
    window.location.href = `${API_URL}/api/v1/auth/login`;
  }
</script>

<div class="min-h-screen bg-gradient-to-br from-purple-900 via-blue-900 to-indigo-900">
  <div class="container mx-auto px-4 py-20">
    <div class="text-center">
      <h1 class="text-6xl font-bold text-white mb-6">
        All-Chat
      </h1>
      <p class="text-xl text-gray-300 mb-12">
        Aggregate chat from Twitch, YouTube, and more in one overlay
      </p>

      <div class="max-w-md mx-auto space-y-4">
        <Button
          on:click={loginWithTwitch}
          class="w-full bg-twitch hover:bg-purple-700 text-white py-6 text-lg"
        >
          <Twitch class="mr-2 h-6 w-6" />
          Login with Twitch
        </Button>

        <div class="flex items-center gap-4 text-gray-400 text-sm">
          <div class="flex items-center gap-2">
            <Twitch class="h-4 w-4" />
            Twitch
          </div>
          <div class="flex items-center gap-2">
            <Youtube class="h-4 w-4" />
            YouTube
          </div>
          <div class="text-xs">+ More coming soon</div>
        </div>
      </div>

      <div class="mt-20 grid grid-cols-1 md:grid-cols-3 gap-8 max-w-4xl mx-auto">
        <div class="text-center">
          <h3 class="text-xl font-semibold text-white mb-2">Multi-Platform</h3>
          <p class="text-gray-400">Combine chat from Twitch and YouTube in one overlay</p>
        </div>
        <div class="text-center">
          <h3 class="text-xl font-semibold text-white mb-2">Real-Time</h3>
          <p class="text-gray-400">Low latency chat delivery under 500ms</p>
        </div>
        <div class="text-center">
          <h3 class="text-xl font-semibold text-white mb-2">Customizable</h3>
          <p class="text-gray-400">Full control over appearance and emotes</p>
        </div>
      </div>
    </div>
  </div>
</div>
```

#### 2.4 OAuth Callback Handler

```svelte
<!-- src/routes/auth/callback/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { auth } from '$lib/stores/auth';
  import { apiClient } from '$lib/api/client';

  let error = '';
  let loading = true;

  onMount(async () => {
    const code = $page.url.searchParams.get('code');
    const state = $page.url.searchParams.get('state');

    if (!code) {
      error = 'No authorization code received';
      loading = false;
      return;
    }

    try {
      // Exchange code for token (handled by backend)
      // The URL already includes the code, backend will handle it
      // For now, the backend should return a JWT token

      // Fetch user info with token
      const user = await apiClient.get('/api/v1/auth/me');
      auth.setUser(user);

      // Redirect to dashboard
      goto('/dashboard');
    } catch (err) {
      error = 'Authentication failed. Please try again.';
      loading = false;
    }
  });
</script>

<div class="min-h-screen flex items-center justify-center bg-gray-900">
  {#if loading}
    <div class="text-center">
      <div class="animate-spin rounded-full h-16 w-16 border-b-2 border-white mx-auto mb-4"></div>
      <p class="text-white">Authenticating...</p>
    </div>
  {:else if error}
    <div class="text-center">
      <p class="text-red-500 mb-4">{error}</p>
      <a href="/" class="text-blue-400 hover:underline">Return to home</a>
    </div>
  {/if}
</div>
```

#### 2.5 Root Layout with Auth

```svelte
<!-- src/routes/+layout.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/stores/auth';
  import '../app.css';

  onMount(() => {
    auth.init();
  });
</script>

<slot />
```

---

### Task 3: Overlay Management UI (7-10 days)

#### 3.1 Dashboard Page

```svelte
<!-- src/routes/dashboard/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';
  import { overlays } from '$lib/stores/overlays';
  import { Button } from '$lib/components/ui/button';
  import OverlayCard from '$lib/components/OverlayCard.svelte';
  import Navbar from '$lib/components/Navbar.svelte';
  import { Plus } from 'lucide-svelte';

  let loading = true;

  onMount(async () => {
    if (!$auth.token) {
      goto('/');
      return;
    }

    await overlays.fetchAll();
    loading = false;
  });
</script>

<div class="min-h-screen bg-gray-900">
  <Navbar />

  <div class="container mx-auto px-4 py-8">
    <div class="flex justify-between items-center mb-8">
      <h1 class="text-3xl font-bold text-white">My Overlays</h1>
      <Button on:click={() => goto('/overlays/new')}>
        <Plus class="mr-2 h-4 w-4" />
        Create Overlay
      </Button>
    </div>

    {#if loading}
      <div class="text-center text-gray-400">Loading...</div>
    {:else if $overlays.length === 0}
      <div class="text-center py-20">
        <p class="text-gray-400 mb-4">No overlays yet</p>
        <Button on:click={() => goto('/overlays/new')}>Create Your First Overlay</Button>
      </div>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {#each $overlays as overlay}
          <OverlayCard {overlay} />
        {/each}
      </div>
    {/if}
  </div>
</div>
```

#### 3.2 Create Overlay Page

```svelte
<!-- src/routes/overlays/new/+page.svelte -->
<script lang="ts">
  import { goto } from '$app/navigation';
  import { overlays } from '$lib/stores/overlays';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import Navbar from '$lib/components/Navbar.svelte';

  let name = '';
  let description = '';
  let loading = false;
  let error = '';

  async function createOverlay() {
    if (!name) {
      error = 'Name is required';
      return;
    }

    loading = true;
    error = '';

    try {
      const overlay = await overlays.create({ name, description });
      goto(`/overlays/${overlay.id}`);
    } catch (err) {
      error = 'Failed to create overlay';
      loading = false;
    }
  }
</script>

<div class="min-h-screen bg-gray-900">
  <Navbar />

  <div class="container mx-auto px-4 py-8 max-w-2xl">
    <h1 class="text-3xl font-bold text-white mb-8">Create New Overlay</h1>

    <form on:submit|preventDefault={createOverlay} class="space-y-6">
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Overlay Name *
        </label>
        <Input
          bind:value={name}
          placeholder="My Awesome Overlay"
          required
          class="bg-gray-800 text-white border-gray-700"
        />
      </div>

      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Description
        </label>
        <Textarea
          bind:value={description}
          placeholder="Optional description..."
          rows={3}
          class="bg-gray-800 text-white border-gray-700"
        />
      </div>

      {#if error}
        <p class="text-red-500 text-sm">{error}</p>
      {/if}

      <div class="flex gap-4">
        <Button type="submit" disabled={loading}>
          {loading ? 'Creating...' : 'Create Overlay'}
        </Button>
        <Button type="button" variant="outline" on:click={() => goto('/dashboard')}>
          Cancel
        </Button>
      </div>
    </form>
  </div>
</div>
```

#### 3.3 Overlay Editor Page

```svelte
<!-- src/routes/overlays/[id]/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { overlays } from '$lib/stores/overlays';
  import { Button } from '$lib/components/ui/button';
  import Navbar from '$lib/components/Navbar.svelte';
  import SourceList from '$lib/components/SourceList.svelte';
  import AddSourceModal from '$lib/components/AddSourceModal.svelte';
  import { Eye, Settings, Trash2 } from 'lucide-svelte';

  let overlayId = $page.params.id;
  let overlay = null;
  let sources = [];
  let loading = true;
  let showAddSource = false;

  onMount(async () => {
    overlay = await overlays.fetchOne(overlayId);
    sources = await overlays.fetchSources(overlayId);
    loading = false;
  });

  async function addSource(platform: string, channelId: string) {
    await overlays.addSource(overlayId, { platform, channel_id: channelId });
    sources = await overlays.fetchSources(overlayId);
    showAddSource = false;
  }

  async function removeSource(sourceId: string) {
    if (confirm('Remove this source?')) {
      await overlays.removeSource(overlayId, sourceId);
      sources = await overlays.fetchSources(overlayId);
    }
  }
</script>

<div class="min-h-screen bg-gray-900">
  <Navbar />

  {#if loading}
    <div class="container mx-auto px-4 py-8">
      <p class="text-gray-400">Loading...</p>
    </div>
  {:else if overlay}
    <div class="container mx-auto px-4 py-8">
      <div class="flex justify-between items-center mb-8">
        <div>
          <h1 class="text-3xl font-bold text-white">{overlay.name}</h1>
          {#if overlay.description}
            <p class="text-gray-400 mt-2">{overlay.description}</p>
          {/if}
        </div>

        <div class="flex gap-2">
          <Button href="/overlays/{overlayId}/preview">
            <Eye class="mr-2 h-4 w-4" />
            Preview
          </Button>
          <Button variant="outline">
            <Settings class="mr-2 h-4 w-4" />
            Settings
          </Button>
        </div>
      </div>

      <div class="bg-gray-800 rounded-lg p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold text-white">Chat Sources</h2>
          <Button on:click={() => showAddSource = true}>Add Source</Button>
        </div>

        <SourceList {sources} on:remove={(e) => removeSource(e.detail)} />
      </div>
    </div>

    {#if showAddSource}
      <AddSourceModal
        on:add={(e) => addSource(e.detail.platform, e.detail.channelId)}
        on:close={() => showAddSource = false}
      />
    {/if}
  {/if}
</div>
```

---

### Task 4: Overlay Preview (7-10 days)

#### 4.1 WebSocket Client

```typescript
// src/lib/api/websocket.ts
import type { ChatMessage } from '$lib/types/message';

export class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectTimeout: number | null = null;
  private messageCallbacks: ((message: ChatMessage) => void)[] = [];

  connect(overlayId: string, token: string) {
    const wsUrl = import.meta.env.PUBLIC_WS_URL || 'ws://localhost:8080';
    const url = `${wsUrl}/ws/overlay/${overlayId}?token=${token}`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
    };

    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === 'chat_message') {
        this.messageCallbacks.forEach(cb => cb(data.data));
      } else if (data.type === 'ping') {
        this.ws?.send(JSON.stringify({ type: 'pong', timestamp: new Date().toISOString() }));
      }
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('WebSocket closed, reconnecting...');
      this.reconnectTimeout = window.setTimeout(() => {
        this.connect(overlayId, token);
      }, 3000);
    };
  }

  onMessage(callback: (message: ChatMessage) => void) {
    this.messageCallbacks.push(callback);
  }

  disconnect() {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
    }
    this.ws?.close();
  }
}
```

#### 4.2 Overlay Preview Page

```svelte
<!-- src/routes/overlays/[id]/preview/+page.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { auth } from '$lib/stores/auth';
  import { WebSocketClient } from '$lib/api/websocket';
  import ChatMessage from '$lib/components/ChatMessage.svelte';
  import type { ChatMessage as ChatMessageType } from '$lib/types/message';

  let overlayId = $page.params.id;
  let messages: ChatMessageType[] = [];
  let wsClient: WebSocketClient;
  let connected = false;

  onMount(() => {
    if (!$auth.token) return;

    wsClient = new WebSocketClient();
    wsClient.connect(overlayId, $auth.token);

    wsClient.onMessage((message) => {
      messages = [...messages, message].slice(-50); // Keep last 50 messages
      connected = true;
    });
  });

  onDestroy(() => {
    wsClient?.disconnect();
  });

  function copyOverlayUrl() {
    const url = `${window.location.origin}/overlay/${overlayId}`;
    navigator.clipboard.writeText(url);
    alert('Overlay URL copied to clipboard!');
  }
</script>

<div class="min-h-screen bg-gray-900 p-4">
  <div class="max-w-6xl mx-auto">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold text-white">Overlay Preview</h1>
      <div class="flex gap-2">
        <span class="text-sm {connected ? 'text-green-500' : 'text-red-500'}">
          {connected ? '● Connected' : '● Disconnected'}
        </span>
        <Button size="sm" on:click={copyOverlayUrl}>Copy OBS URL</Button>
      </div>
    </div>

    <!-- Preview Area -->
    <div class="bg-black rounded-lg p-4 h-[600px] overflow-y-auto">
      {#if messages.length === 0}
        <div class="text-center text-gray-500 py-20">
          Waiting for messages...
        </div>
      {:else}
        <div class="space-y-2">
          {#each messages as message (message.id)}
            <ChatMessage {message} />
          {/each}
        </div>
      {/if}
    </div>

    <!-- Customization Panel -->
    <div class="bg-gray-800 rounded-lg p-4 mt-4">
      <h2 class="text-lg font-semibold text-white mb-4">Customization</h2>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-sm text-gray-300">Font Size</label>
          <input type="range" min="12" max="32" value="16" class="w-full" />
        </div>
        <div>
          <label class="text-sm text-gray-300">Message Duration (s)</label>
          <input type="range" min="5" max="60" value="15" class="w-full" />
        </div>
      </div>
    </div>
  </div>
</div>
```

---

## Timeline

| Week | Days | Tasks | Status |
|------|------|-------|--------|
| **Week 1** | Mon-Wed | Svelte setup, TailwindCSS, TypeScript | ⏳ |
| | Thu-Fri | Auth store, API client, landing page | ⏳ |
| **Week 2** | Mon-Tue | OAuth callback, dashboard | ⏳ |
| | Wed-Thu | Overlay CRUD pages | ⏳ |
| | Fri | Source management UI | ⏳ |
| **Week 3** | Mon-Tue | WebSocket client, preview page | ⏳ |
| | Wed-Thu | Chat message rendering, emotes | ⏳ |
| | Fri | Customization controls | ⏳ |
| **Week 4** | Mon-Wed | E2E tests, bug fixes | ⏳ |
| | Thu-Fri | Polish, deployment, docs | ⏳ |

---

## Success Criteria

Phase 5 is complete when:

- [ ] SvelteKit project initialized and configured
- [ ] Users can log in with Twitch OAuth
- [ ] Dashboard displays user's overlays
- [ ] Users can create new overlays
- [ ] Users can add Twitch sources
- [ ] Users can add YouTube sources (after OAuth)
- [ ] Users can remove sources
- [ ] Overlay preview shows real-time messages via WebSocket
- [ ] Messages display with user badges and colors
- [ ] Emotes render correctly
- [ ] Users can customize overlay appearance
- [ ] OBS Browser Source URL can be copied
- [ ] All E2E tests passing
- [ ] Docker build succeeds
- [ ] Deployed to staging

---

## Definition of Done

- [ ] All pages implemented and functional
- [ ] WebSocket integration working
- [ ] OAuth flows (Twitch + YouTube) working
- [ ] Tests passing (Vitest + Playwright)
- [ ] Responsive design (mobile-friendly)
- [ ] Dockerfile created
- [ ] Kubernetes manifests created
- [ ] Documentation complete
- [ ] Ready for user acceptance testing

---

**Next Phase**: [Phase 6: Production Hardening](./PHASE_6_PLAN.md) (Metrics, observability, security)
