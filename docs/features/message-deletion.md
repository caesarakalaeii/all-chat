# Message Deletion

Cross-platform deletion sync: when a moderator deletes a message on the streaming platform, it is removed from connected overlays. Related guides: [twitch-moderation-events.md](./twitch-moderation-events.md) (who acted), ADR-0015, [ADR-0054](../adr/0054-twitch-moderation-and-automod-event-feed.md).

## Platform support

| Platform | Single | Batch (timeout/ban) | Clear chat | Latency | Event source |
|----------|--------|---------------------|------------|---------|--------------|
| Twitch   | yes    | yes                 | yes        | <500ms  | IRC CLEARMSG/CLEARCHAT |
| YouTube  | yes    | yes (user ban)      | no (API)   | 30-60s  | HTTP polling (liveChatMessages) |
| Kick     | yes    | unvalidated         | unvalidated | <500ms | Pusher WebSocket `ChatMessageDeletedEvent` |
| TikTok   | no     | no                  | no         | n/a     | not supported by the unofficial library |

No configuration is required: deletion handling is enabled for all platforms automatically. There is no per-overlay toggle.

## How it works

1. A listener detects the deletion event on its platform.
2. The Message ID Registry (in `services/message-processor/registry/`, a Redis hash of platform message id to internal UUID, ~1h TTL refreshed on message add) maps the platform id to the internal UUID.
3. Deletions that reference an unknown UUID are buffered in Redis for 60 seconds, so a deletion arriving before its message still applies.
4. Events are normalized to one schema and published on the overlay's Redis pub/sub channel; the API gateway and the overlay remove the row.

Unified schema:

```json
{
  "type": "chat_message",
  "data": {
    "event": {
      "type": "message_deletion",
      "metadata": {
        "deletion_type": "single|batch|clear",
        "target_msg_id": "platform-message-id",
        "target_user_id": "user-id-for-batch",
        "target_username": "username-for-batch",
        "platform": "twitch|youtube|kick"
      }
    }
  }
}
```

Frontend behavior by `deletion_type`: `single` removes the row with the matching `target_uuid`; `batch` removes every row for `target_user_id`; `clear` empties the feed.

## Platform notes

- **Twitch** covers all three deletion types in real time and is production-stable.
- **YouTube** supports single plus batch deletions at the cost of the existing liveChatMessages polling cycle alone (no extra quota); there is no clear-chat API.
- **Kick**: single deletion is implemented against the unofficial Pusher event; batch and clear are not yet validated in production.
- **TikTok**: not supported; deleted messages persist on the overlay.

## Reconnection

Reconnection uses a `?since=` watermark (persisted in `localStorage`), not an in-band replay request: after a drop the frontend reconnects with the watermark and the API gateway sends the chat it cached while the socket was down. Deletion frames are not stored for replay, so deletions that happened during the gap are lost by design; reloading the overlay fetches a fresh message set that already excludes them.

## Monitoring

Deletion flow stages are measured with the shared processor and gateway metrics (deletion normalization stage, deletion handler success/failure counters, replay traffic in the websocket layer). Rising errors there are the signal for a platform API change; a growing deletion buffer means its 60-second TTL is not expiring.
