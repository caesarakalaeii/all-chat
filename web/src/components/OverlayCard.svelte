<script lang="ts">
  interface Props {
    overlay: any;
    onDelete: () => void;
  }

  let { overlay, onDelete }: Props = $props();

  function copyOverlayURL() {
    const token = localStorage.getItem('access_token');
    const url = `${window.location.origin}/overlay/${overlay.id}?token=${token}`;
    navigator.clipboard.writeText(url);
    alert('Overlay URL copied to clipboard!');
  }

  function openOverlay() {
    const token = localStorage.getItem('access_token');
    window.open(`/overlay/${overlay.id}?token=${token}`, '_blank');
  }
</script>

<div class="overlay-card">
  <div class="card-header">
    <h3>{overlay.name}</h3>
    <span class="status" class:active={overlay.is_active}>
      {overlay.is_active ? 'Active' : 'Inactive'}
    </span>
  </div>

  <div class="card-body">
    <div class="info-row">
      <span class="label">Channel:</span>
      <span class="value">{overlay.twitch_channel}</span>
    </div>
    <div class="info-row">
      <span class="label">Created:</span>
      <span class="value">{new Date(overlay.created_at).toLocaleDateString()}</span>
    </div>
  </div>

  <div class="card-actions">
    <button class="action-button" onclick={openOverlay}>View</button>
    <button class="action-button" onclick={copyOverlayURL}>Copy URL</button>
    <button class="action-button delete" onclick={onDelete}>Delete</button>
  </div>
</div>

<style>
  .overlay-card {
    background: #18181b;
    border: 1px solid #2d2d31;
    border-radius: 8px;
    padding: 1.5rem;
    transition: transform 0.2s, box-shadow 0.2s;
  }

  .overlay-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  h3 {
    margin: 0;
    font-size: 1.25rem;
  }

  .status {
    padding: 0.25rem 0.75rem;
    border-radius: 12px;
    font-size: 0.75rem;
    font-weight: 600;
    background: #3a3a3d;
    color: #adadb8;
  }

  .status.active {
    background: #00c851;
    color: white;
  }

  .card-body {
    margin-bottom: 1rem;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }

  .label {
    color: #adadb8;
    font-size: 0.9rem;
  }

  .value {
    font-weight: 500;
  }

  .card-actions {
    display: flex;
    gap: 0.5rem;
  }

  .action-button {
    flex: 1;
    background: #2d2d31;
    color: #efeff1;
    border: none;
    padding: 0.5rem;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
    font-size: 0.9rem;
  }

  .action-button:hover {
    background: #3a3a3d;
  }

  .action-button.delete {
    background: #e74c3c;
  }

  .action-button.delete:hover {
    background: #c0392b;
  }
</style>
