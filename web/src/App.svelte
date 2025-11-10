<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from './lib/api';
  import { parseRoute, onRouteChange, type Route } from './lib/router';
  import Dashboard from './components/Dashboard.svelte';
  import Landing from './components/Landing.svelte';
  import OverlayViewerPage from './components/OverlayViewerPage.svelte';

  let user = $state<any>(null);
  let loading = $state(true);
  let currentRoute = $state<Route>(parseRoute());

  onMount(async () => {
    // Set up route change listener
    const cleanup = onRouteChange(() => {
      currentRoute = parseRoute();
    });

    // Check if we have tokens in URL (OAuth callback)
    const params = new URLSearchParams(window.location.search);
    const accessToken = params.get('access_token');
    const refreshToken = params.get('refresh_token');

    if (accessToken && refreshToken) {
      localStorage.setItem('access_token', accessToken);
      localStorage.setItem('refresh_token', refreshToken);
      window.history.replaceState({}, document.title, window.location.pathname);
    }

    // Try to get current user (except for overlay viewer route which doesn't require auth)
    if (currentRoute.path !== '/overlay/:id') {
      try {
        const response = await auth.getMe();
        user = response.data.user;
      } catch (error) {
        console.log('Not authenticated');
      }
    }

    loading = false;

    return cleanup;
  });

  function handleLogout() {
    auth.logout();
    user = null;
  }
</script>

<main>
  {#if loading}
    <div class="loading">Loading...</div>
  {:else if currentRoute.path === '/overlay/:id' && currentRoute.params?.id}
    <OverlayViewerPage overlayId={currentRoute.params.id} />
  {:else if user}
    <Dashboard {user} onLogout={handleLogout} />
  {:else}
    <Landing />
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    background: #0e0e10;
    color: #efeff1;
  }

  main {
    min-height: 100vh;
  }

  .loading {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    font-size: 1.5rem;
  }
</style>
