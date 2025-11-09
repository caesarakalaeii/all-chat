<script lang="ts">
  interface Props {
    onSubmit: (data: { name: string; twitch_channel: string }) => void;
    onClose: () => void;
  }

  let { onSubmit, onClose }: Props = $props();

  let name = $state('');
  let twitchChannel = $state('');

  function handleSubmit(e: Event) {
    e.preventDefault();
    if (name && twitchChannel) {
      onSubmit({ name, twitch_channel: twitchChannel.toLowerCase() });
    }
  }
</script>

<div class="modal-overlay" onclick={onClose}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h2>Create New Overlay</h2>

    <form onsubmit={handleSubmit}>
      <div class="form-group">
        <label for="name">Overlay Name</label>
        <input
          id="name"
          type="text"
          bind:value={name}
          placeholder="My Stream Overlay"
          required
        />
      </div>

      <div class="form-group">
        <label for="channel">Twitch Channel</label>
        <input
          id="channel"
          type="text"
          bind:value={twitchChannel}
          placeholder="channelname"
          required
        />
        <small>Enter the Twitch channel name (without @)</small>
      </div>

      <div class="form-actions">
        <button type="button" class="cancel-button" onclick={onClose}>
          Cancel
        </button>
        <button type="submit" class="submit-button">
          Create Overlay
        </button>
      </div>
    </form>
  </div>
</div>

<style>
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
  }

  .modal {
    background: #18181b;
    border-radius: 8px;
    padding: 2rem;
    max-width: 500px;
    width: 90%;
    border: 1px solid #2d2d31;
  }

  h2 {
    margin: 0 0 1.5rem 0;
  }

  .form-group {
    margin-bottom: 1.5rem;
  }

  label {
    display: block;
    margin-bottom: 0.5rem;
    font-weight: 600;
  }

  input {
    width: 100%;
    padding: 0.75rem;
    background: #0e0e10;
    border: 1px solid #2d2d31;
    border-radius: 4px;
    color: #efeff1;
    font-size: 1rem;
    box-sizing: border-box;
  }

  input:focus {
    outline: none;
    border-color: #9147ff;
  }

  small {
    display: block;
    margin-top: 0.25rem;
    color: #adadb8;
    font-size: 0.85rem;
  }

  .form-actions {
    display: flex;
    gap: 1rem;
    justify-content: flex-end;
  }

  button {
    padding: 0.75rem 1.5rem;
    border: none;
    border-radius: 4px;
    font-size: 1rem;
    cursor: pointer;
    transition: background 0.2s;
    font-weight: 600;
  }

  .cancel-button {
    background: #2d2d31;
    color: #efeff1;
  }

  .cancel-button:hover {
    background: #3a3a3d;
  }

  .submit-button {
    background: #9147ff;
    color: white;
  }

  .submit-button:hover {
    background: #772ce8;
  }
</style>
