# Troubleshooting: WebSocket disconnects

API Gateway overlay WebSocket issues. The gateway protocol and keepalive contract are in `services/api-gateway/README.md`; client logic lives in `useOverlayStream` (and the legacy `WebSocketClient`).

## Cannot connect

Browser console "WebSocket connection failed":

1. Gateway up? `curl http://localhost:8080/health/ready`
2. Overlay id valid? It must exist in the database.
3. Origin allowed? Check `CORS_ORIGIN` and the gateway logs for a rejected upgrade.

## Drops every 30-60 seconds

Causes: proxy/NAT idle timeout, gateway pod restart, or a client failing the read deadline. The keepalive contract:

- The gateway sends a protocol-level ping every 30s (`PingInterval`) and closes if nothing arrives within 60s (`PongWait`). Browsers answer protocol pings automatically.
- The client also sends an app-level `{"type":"ping"}` every 20s; the gateway echoes `{"type":"pong"}` (`connection.go`), which lets the client detect a dead path.

Do not add ad-hoc keepalive layers on top; check that the gateway still echoes pong and that no proxy strips it.

## Connection silently dies, badge stuck on "Live"

A half-open socket: the path broke (Wi-Fi, NAT timeout, sleep/wake) but no FIN/RST arrived, so `readyState` stays OPEN and `onclose` never fires. Handled client-side (`useOverlayStream`):

- Any inbound frame refreshes `lastActivity`; a 5s watchdog declares the socket dead after 40s (`LIVENESS_TIMEOUT_MS`) of silence, flips the badge to Reconnecting and force-reconnects with `?since=` (the gateway replays the gap).
- `online` and `visibilitychange -> visible` force an immediate reconnect; a fully suspended tab only recovers on those events.
- `isConnected()` / `connectionStatus` are liveness-aware.

If this regresses: confirm the gateway pong echo still works and client timers are not throttled.

## Connected but no messages

The break is upstream of the gateway. Trace from the source:

```bash
kubectl exec -n allchat deploy/redis -- redis-cli XINFO STREAM chat:raw        # listener published?
kubectl exec -n allchat deploy/redis -- redis-cli XINFO GROUPS chat:raw        # message-processor consuming, lag low?
kubectl exec -n allchat deploy/redis -- redis-cli PUBSUB CHANNELS "overlay:*"  # processor publishing to this overlay?
kubectl logs -n allchat -l app=api-gateway | grep -i subscript
```
