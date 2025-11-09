<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { OverlayWebSocket, type ChatMessage } from '../lib/websocket';

  interface Props {
    overlayId: string;
    token: string;
  }

  let { overlayId, token }: Props = $props();

  let messages = $state<(ChatMessage & { id: string })[]>([]);
  let ws: OverlayWebSocket | null = null;

  onMount(() => {
    ws = new OverlayWebSocket();
    ws.connect(overlayId, token);

    ws.onMessage((data: ChatMessage) => {
      const messageWithId = {
        ...data,
        id: `${Date.now()}-${Math.random()}`,
      };

      messages = [...messages, messageWithId].slice(-50); // Keep last 50 messages

      // Remove message after 30 seconds (default duration)
      setTimeout(() => {
        messages = messages.filter((m) => m.id !== messageWithId.id);
      }, 30000);
    });
  });

  onDestroy(() => {
    ws?.disconnect();
  });

  function parseMessageWithEmotes(message: ChatMessage['message']) {
    const words = message.text.split(' ');
    const emoteMap = new Map(message.emotes.map((e) => [e.code, e]));

    return words.map((word, idx) => ({
      text: word,
      emote: emoteMap.get(word),
      key: `${idx}-${word}`,
    }));
  }
</script>

<div class="overlay">
  {#each messages as message (message.id)}
    <div class="message" style="color: {message.user.color || '#ffffff'}">
      <div class="badges">
        {#each message.user.badges as badge}
          <span class="badge">{badge}</span>
        {/each}
      </div>

      <span class="username">{message.user.display_name}:</span>

      <span class="text">
        {#each parseMessageWithEmotes(message.message) as part (part.key)}
          {#if part.emote}
            <img
              src={part.emote.url}
              alt={part.emote.code}
              class="emote"
              class:animated={part.emote.animated}
            />
          {:else}
            {part.text + ' '}
          {/if}
        {/each}
      </span>
    </div>
  {/each}
</div>

<style>
  .overlay {
    position: fixed;
    bottom: 20px;
    left: 20px;
    right: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    pointer-events: none;
  }

  .message {
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(10px);
    padding: 12px 16px;
    border-radius: 8px;
    animation: slideIn 0.3s ease-out;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 18px;
    line-height: 1.4;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
  }

  @keyframes slideIn {
    from {
      transform: translateX(-100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }

  .badges {
    display: flex;
    gap: 4px;
  }

  .badge {
    background: #9147ff;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 600;
  }

  .username {
    font-weight: 700;
    text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.8);
  }

  .text {
    color: #ffffff;
    text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.8);
    display: flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
  }

  .emote {
    height: 28px;
    vertical-align: middle;
    display: inline-block;
  }

  .emote.animated {
    /* Animated emotes are typically GIFs/WebP, no special handling needed */
  }
</style>
