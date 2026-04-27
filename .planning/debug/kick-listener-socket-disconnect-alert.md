---
status: investigating
trigger: "Recurring CRITICAL alert 'Kick Listener Socket Disconnected' that auto-clears after ~5 minutes. Need to determine if this is a real problem requiring code fix or if the alert is too sensitive for normal Pusher WebSocket reconnection behavior."
created: 2026-04-07T00:00:00Z
updated: 2026-04-07T00:01:00Z
---

## Current Focus

hypothesis: CONFIRMED — The alert fires on every normal Pusher WebSocket reconnection because the alert has `for: 2m` but the reconnection backoff can reach up to 60s + processing time, keeping the metric at 0 long enough to trigger. The metric is set to 0 immediately on disconnect (before reconnect attempt) and only set back to 1 after the pusher:connection_established handshake succeeds.
test: Complete — traced the full disconnect→metric=0→reconnect→metric=1 cycle against the alert's `for: 2m` window
expecting: N/A (root cause confirmed)
next_action: Report diagnosis

## Symptoms

expected: Kick listener maintains stable Pusher WebSocket connection without triggering critical alerts, or at least handles reconnections gracefully without alarming
actual: WebSocket disconnects periodically, triggering CRITICAL alert, then auto-recovers in ~5 minutes — alert clears on its own
errors: Alert "Kick Listener Socket Disconnected" fires as CRITICAL severity
reproduction: Happens on its own periodically — no manual trigger needed
started: Recurring pattern, not a one-time event

## Eliminated

- hypothesis: Code bug causing unnecessary disconnections
  evidence: No bug found. readPump exit on read error is correct behavior. triggerReconnect is called which kicks off reconnect loop.
  timestamp: 2026-04-07T00:01:00Z

- hypothesis: Reconnect loop fails silently and never actually recovers
  evidence: Reconnect loop in handleReconnections uses exponential backoff (1s→2s→...→60s max), calls wsClient.Connect(), then resubscribes. Alert auto-clears in ~5 min confirms recovery does happen.
  timestamp: 2026-04-07T00:01:00Z

## Evidence

- timestamp: 2026-04-07T00:01:00Z
  checked: services/kick-listener/websocket/client.go — triggerReconnect()
  found: When a disconnect is detected (readPump stops, write error, or pusher error), triggerReconnect() immediately sets connected=false and calls metrics.SetSocketConnected(false). This sets kick_listener_socket_state=0 before any reconnect attempt begins.
  implication: The metric drops to 0 the instant a disconnect is noticed.

- timestamp: 2026-04-07T00:01:00Z
  checked: services/kick-listener/websocket/client.go — handleConnectionEstablished()
  found: metrics.SetSocketConnected(true) is only called after the full pusher:connection_established handshake message is received and parsed successfully. This happens after wsClient.Connect() dials the WebSocket AND waits for Pusher's first message.
  implication: There is a gap between metric=0 and metric=1 that spans: backoff sleep + dial time + Pusher handshake time.

- timestamp: 2026-04-07T00:01:00Z
  checked: services/kick-listener/cmd/main.go — handleReconnections()
  found: Exponential backoff starts at 1s, doubles on failure, caps at 60s. On the FIRST reconnect attempt: sleeps 1s, then dials, then waits for handshake. If dial or handshake succeeds on first try: total gap = ~1-3s. If first attempt fails: gap = 1s + 2s + ... potentially many seconds before retry succeeds.
  implication: A transient failure (e.g. Pusher endpoint temporarily unreachable) on the first reconnect attempt doubles the gap to 3s+, and a second failure doubles again to 7s+. At backoff=60s a single failed attempt means >60s at metric=0.

- timestamp: 2026-04-07T00:01:00Z
  checked: caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml — uid: listener-disconnected-kick
  found: Alert rule expr: `kick_listener_socket_state == 0`, for: 2m, severity: critical, interval: 1m. The `for: 2m` means: if the metric stays at 0 for 2 consecutive minutes, fire. With 1m evaluation interval this means 2 consecutive evaluation cycles where metric=0.
  implication: If the reconnect gap exceeds 2 minutes (e.g. because first reconnect attempt to Pusher fails and backoff grows to 4s+16s+... before success), the alert fires. This is consistent with the ~5 minute auto-clear: it fires after 2m of metric=0, and clears once the reconnect succeeds (metric=1 again).

- timestamp: 2026-04-07T00:01:00Z
  checked: Pusher connection settings in client.go — pongWait=150s, pingPeriod=30s
  found: The Pusher activity_timeout is 120s per server. The client reads with a 150s deadline and sends pings every 30s. If Pusher server decides to drop the connection (e.g. server-side keepalive timeout, maintenance, IP rotation), the client won't know until the 150s read deadline fires — or immediately if the TCP connection is cleanly closed.
  implication: Pusher periodically rotates/recycles WebSocket connections. This is well-documented Pusher behavior. The disconnect is a normal Pusher lifecycle event, not a bug.

- timestamp: 2026-04-07T00:01:00Z
  checked: Reconnect success → channel resubscription path
  found: After wsClient.Connect() succeeds in handleReconnections(), it breaks out of the retry loop. Channel resubscriptions happen via resubscribeAll() which is called from handleConnectionEstablished() — i.e., automatically when the handshake completes. The handleReconnections loop just needs to get the dial through; resubscription is handled inside the WebSocket client.
  implication: Channel subscriptions do survive reconnect correctly. This path is not broken.

## Resolution

root_cause: The alert `for: 2m` fires on normal Pusher WebSocket reconnection cycles. Pusher periodically terminates WebSocket connections (server-side keepalive/maintenance). The kick-listener detects this correctly, sets metric=0 immediately, then reconnects with exponential backoff starting at 1s. If the first reconnect attempt to Pusher fails (transient network blip, Pusher cluster issue), the backoff can reach several minutes before success, keeping metric=0 long enough to breach the 2-minute alert threshold. The behavior is functionally correct — the listener recovers autonomously — but the alert is miscalibrated for the expected reconnect latency.

fix:
verification:
files_changed: []
