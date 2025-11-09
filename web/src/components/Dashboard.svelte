<script lang="ts">
  import { onMount } from 'svelte';
  import { overlays } from '../lib/api';
  import OverlayCard from './OverlayCard.svelte';
  import CreateOverlayModal from './CreateOverlayModal.svelte';

  interface Props {
    user: any;
    onLogout: () => void;
  }

  let { user, onLogout }: Props = $props();

  let overlayList = $state<any[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);

  onMount(async () => {
    await loadOverlays();
  });

  async function loadOverlays() {
    try {
      const response = await overlays.list();
      overlayList = response.data.overlays || [];
    } catch (error) {
      console.error('Failed to load overlays:', error);
    } finally {
      loading = false;
    }
  }

  async function handleCreateOverlay(data: { name: string; twitch_channel: string }) {
    try {
      await overlays.create(data);
      await loadOverlays();
      showCreateModal = false;
    } catch (error) {
      console.error('Failed to create overlay:', error);
      alert('Failed to create overlay');
    }
  }

  async function handleDeleteOverlay(id: string) {
    if (!confirm('Are you sure you want to delete this overlay?')) {
      return;
    }

    try {
      await overlays.delete(id);
      await loadOverlays();
    } catch (error) {
      console.error('Failed to delete overlay:', error);
      alert('Failed to delete overlay');
    }
  }
</script>

<div class="dashboard">
  <header>
    <div class="header-content">
      <h1>All-Chat Dashboard</h1>
      <div class="user-info">
        <span>Welcome, {user.display_name}!</span>
        <button class="logout-button" onclick={onLogout}>Logout</button>
      </div>
    </div>
  </header>

  <main class="content">
    <div class="actions">
      <button class="create-button" onclick={() => showCreateModal = true}>
        + Create New Overlay
      </button>
    </div>

    {#if loading}
      <div class="loading">Loading overlays...</div>
    {:else if overlayList.length === 0}
      <div class="empty">
        <p>No overlays yet. Create your first overlay to get started!</p>
      </div>
    {:else}
      <div class="overlay-grid">
        {#each overlayList as overlay (overlay.id)}
          <OverlayCard {overlay} onDelete={() => handleDeleteOverlay(overlay.id)} />
        {/each}
      </div>
    {/if}
  </main>

  {#if showCreateModal}
    <CreateOverlayModal
      onSubmit={handleCreateOverlay}
      onClose={() => showCreateModal = false}
    />
  {/if}
</div>

<style>
  .dashboard {
    min-height: 100vh;
    background: #0e0e10;
  }

  header {
    background: #18181b;
    border-bottom: 1px solid #2d2d31;
    padding: 1rem 2rem;
  }

  .header-content {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  h1 {
    margin: 0;
    font-size: 1.5rem;
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .logout-button {
    background: #2d2d31;
    color: #efeff1;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
  }

  .logout-button:hover {
    background: #3a3a3d;
  }

  .content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2rem;
  }

  .actions {
    margin-bottom: 2rem;
  }

  .create-button {
    background: #9147ff;
    color: white;
    border: none;
    padding: 0.75rem 1.5rem;
    border-radius: 4px;
    font-size: 1rem;
    cursor: pointer;
    transition: background 0.2s;
    font-weight: 600;
  }

  .create-button:hover {
    background: #772ce8;
  }

  .loading,
  .empty {
    text-align: center;
    padding: 3rem;
    color: #adadb8;
  }

  .overlay-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1.5rem;
  }
</style>
