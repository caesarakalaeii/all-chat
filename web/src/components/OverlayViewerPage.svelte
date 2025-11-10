<script lang="ts">
  import { onMount } from 'svelte';
  import OverlayViewer from './OverlayViewer.svelte';
  import { overlays } from '../lib/api';

  interface Props {
    overlayId: string;
  }

  let { overlayId }: Props = $props();

  let loading = $state(true);
  let error = $state<string | null>(null);
  let overlay = $state<any>(null);

  onMount(async () => {
    try {
      // Fetch overlay configuration
      const response = await overlays.get(overlayId);
      overlay = response.data.overlay;
    } catch (err: any) {
      console.error('Failed to load overlay:', err);
      error = err.response?.data?.message || 'Failed to load overlay';
    } finally {
      loading = false;
    }
  });

  // Get token from localStorage or URL query params
  const token = $derived(() => {
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get('token') || localStorage.getItem('access_token') || '';
  });
</script>

<div class="viewer-page">
  {#if loading}
    <div class="status">Loading overlay...</div>
  {:else if error}
    <div class="error">
      <h2>Error</h2>
      <p>{error}</p>
      <a href="/">← Back to Dashboard</a>
    </div>
  {:else if overlay}
    <OverlayViewer {overlayId} token={token()} />
  {/if}
</div>

<style>
  .viewer-page {
    width: 100vw;
    height: 100vh;
    overflow: hidden;
  }

  .status,
  .error {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    text-align: center;
    padding: 20px;
  }

  .error {
    color: #ff4444;
  }

  .error h2 {
    margin-bottom: 10px;
  }

  .error a {
    color: #9147ff;
    text-decoration: none;
    margin-top: 20px;
    padding: 10px 20px;
    border: 2px solid #9147ff;
    border-radius: 6px;
    transition: all 0.2s;
  }

  .error a:hover {
    background: #9147ff;
    color: white;
  }
</style>
