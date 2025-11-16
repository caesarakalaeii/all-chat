## Kick Listener Hardening Roadmap

1. **Activation Parity with Other Listeners**
   - Adopt the same activation workflow used by existing listeners (e.g., TikTok). Rely on PostgreSQL `LISTEN/NOTIFY` to react immediately when overlay sources are (de)activated.
   - Ensure the Kick listener spins services up/down based on the activation events so idle Pusher subscriptions never linger.

2. **Pusher Lifecycle Handling**
   - Implement the full Pusher control plane: handle `pusher:connection_established`, emit heartbeats (`pusher:ping`/`pusher:pong`), subscribe to `chatrooms.<chatroom_id>.v2`, and gracefully back off on `pusher:error`.
   - After reconnects, re-subscribe automatically to all active channels. Use the same behavior libraries such as KickLib expect.

3. **On-Demand Channel Management**
   - Subscribe to a Kick channel only when at least one overlay referencing the source is active. Use the shared “active source” bookkeeping so behavior matches other platforms.
   - Unsubscribe when the last consumer leaves, freeing capacity to scale to many overlays without global subscriptions.

4. **Multi-Cluster Resilience**
   - Parameterize the Pusher cluster and app key through environment variables. Implement fallbacks to retry across multiple regions or clusters.
   - Update the Go 1.25 Dockerfile to include readiness and liveness probes, allowing Kubernetes to restart pods during persistent socket failures.

5. **Message Normalization Parity**
   - Ensure normalized Kick messages match the unified schema (badges, colors, message parts, emotes) already emitted for Twitch/YouTube so downstream services remain platform-agnostic.
   - Expand tests/fixtures to cover Kick-specific attributes and confirm parity.

6. **Observability & Prometheus Metrics**
   - Add structured logs around subscription changes, reconnect attempts, latency from socket receipt to Redis publish, and dropped/duplicate message counts.
   - Expose Prometheus metrics for socket health (connected/disconnected state), subscription counts, reconnect counters, processing latency histograms, and message drop rates. Emit an explicit service health metric for alerting when channels silently stop delivering events.

7. **Security & Scope Clarity**
   - Keep the listener read-only. If outbound messaging is ever needed, implement it as a separate service gated by Kick OAuth.
   - Preserve the existing separation between OAuth flows and listening logic so new write capabilities never share the same surface area.

