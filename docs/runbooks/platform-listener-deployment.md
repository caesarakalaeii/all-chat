# New-listener deployment runbook (owncast, goodgame, picarto, altstream, facebook, instagram)

**Last reviewed:** 2026-11-18

Deploys the platform-expansion listeners (ADR-0058-0062, ADR-0082). See `services/<platform>-listener/README.md` for each listener's behavior and `docs/runbooks/secret-rotation.md` for the `allchat-secrets` hazards that apply here (`kubectl patch` only, never sops writes).

## Secret requirements

| Listener | Third-party secrets at the pod |
|---|---|
| owncast, goodgame, picarto, altstream | none; deploy as-is |
| facebook | none in the pod (reads stored Page tokens), but the platform is inert until auth-service + moderation-service have `facebook-app-id`/`facebook-app-secret` |
| instagram | same story as facebook, plus the Instagram product on the same Meta app |

Infrastructure secrets every listener needs already exist in `allchat-secrets`: `database-password`, `service-jwt-secret` (wired as `SOURCE_MANAGER_SECRET`), and `token-encryption-key-v1` for facebook/instagram. Prod Redis runs without AUTH: never set `REDIS_PASSWORD`.

## Hazard: repo base manifests do not apply as-is

The manifests under `deployments/k8s/base/<platform>-listener/` were written for local dev and reference secret keys that do not match the live `allchat-secrets` (`DATABASE_PASSWORD`, `SERVICE_JWT_SECRET`, a `redis-auth/redis-password` Secret). Applying them verbatim yields `CreateContainerConfigError`. Use the live lowercase keys and copy the pattern from `caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml` (keel annotations `keel.sh/policy: force`, `keel.sh/trigger: poll`, image `ghcr.io/caesarakalaeii/allchat-<name>:beta`, `imagePullPolicy: Always`).

## Deploy a secretless listener (owncast / goodgame / picarto / altstream)

Prereq: PR merged to `beta` and images built (workflow `build-and-push.yml` tags on branch push). altstream-listener runs from `:main`.

1. Add one Deployment per listener in the deployment repo, plus an optional HPA (`minReplicas: 1, maxReplicas: 5`; leadership coordination makes extra replicas safe — with `SOURCE_MANAGER_SECRET` set only the leader ingests). Ports: owncast 8095, goodgame 8096, picarto 8097, altstream 8101.

2. Env block (identical for all):

```yaml
env:
  - name: PORT
    value: "<listener port>"
  - name: LOG_LEVEL
    valueFrom: { configMapKeyRef: { name: allchat-config, key: LOG_LEVEL } }
  - name: DATABASE_HOST
    valueFrom: { configMapKeyRef: { name: allchat-config, key: DATABASE_HOST } }
  - name: DATABASE_PORT
    valueFrom: { configMapKeyRef: { name: allchat-config, key: DATABASE_PORT } }
  - name: DATABASE_NAME
    valueFrom: { configMapKeyRef: { name: allchat-config, key: DATABASE_NAME } }
  - name: DATABASE_USER
    valueFrom: { configMapKeyRef: { name: allchat-config, key: DATABASE_USER } }
  - name: DATABASE_PASSWORD
    valueFrom:
      secretKeyRef: { name: allchat-secrets, key: database-password }   # lowercase
  - name: REDIS_HOST
    valueFrom: { configMapKeyRef: { name: allchat-config, key: REDIS_HOST } }
  - name: REDIS_PORT
    valueFrom: { configMapKeyRef: { name: allchat-config, key: REDIS_PORT } }
  - name: SOURCE_MANAGER_URL
    valueFrom: { configMapKeyRef: { name: allchat-config, key: SOURCE_MANAGER_URL } }
  - name: SOURCE_MANAGER_SECRET
    valueFrom:
      secretKeyRef: { name: allchat-secrets, key: service-jwt-secret }  # lowercase
```

   Altstream only, optional: `ALTSTREAM_BASE_URL` (default `https://altstream.online`).

3. Add the Deployment to `kustomization.yaml` resources, and add the listener to the `listeners` NetworkPolicy (pod selector label + monitoring ingress port). Egress is open in the namespace.

4. Commit to the deployment repo, push, let ArgoCD sync; watch:

```bash
kubectl --context default rollout status deployment/<name>-listener -n allchat --timeout=5m
```

5. Verify:

```bash
kubectl --context default port-forward -n allchat deploy/<name>-listener <port>:<port> >/dev/null &
sleep 1; curl -s http://localhost:<port>/health/ready; kill $!
kubectl --context default get pods -n allchat -l platform=<name>   # restartCount stable
```

End-to-end flow also needs a premium/gate-flipped user to add a source (see troubleshooting table below for the gate); then watch `kubectl --context default logs -n allchat deploy/<name>-listener --tail=100` for leadership and websocket connect lines.

## Deploy Facebook

The listener itself needs no Facebook credentials; the platform is inert until step 3 finishes (auth-service logs "Facebook OAuth will not be available" without them). Deploying the listener before that is harmless but pointless.

1. Create the Meta app (longest lead time: business verification + App Review for `pages_read_engagement` and `pages_manage_engagement`). Walkthrough in `services/facebook-listener/README.md`. OAuth redirect URI: `<FRONTEND_URL>/api/v1/auth/facebook/callback`.
2. Store the credentials via kubectl patch (live key names lowercase):
   ```bash
   kubectl --context default patch secret allchat-secrets -n allchat \
     --type='json' -p="[
       {\"op\": \"add\", \"path\": \"/data/facebook-app-id\", \"value\": \"$(printf '%s' "$FB_APP_ID" | base64)\"},
       {\"op\": \"add\", \"path\": \"/data/facebook-app-secret\", \"value\": \"$(printf '%s' "$FB_APP_SECRET" | base64)\"}
     ]"
   ```
3. Wire `FACEBOOK_APP_ID`/`FACEBOOK_APP_SECRET` into auth-service and moderation-service manifests, rolling-restart both.
4. Deploy the listener (port 8099, single replica by design, ADR-0060, no HPA). Env: the shared block plus `TOKEN_ENCRYPTION_KEY_V1` (`token-encryption-key-v1`), `FACEBOOK_POLLING_INTERVAL_MS: "10000"`, `FACEBOOK_SOURCE_SYNC_SECONDS: "30"`.
5. Verify: dashboard add-source, OAuth consent, poller appears in `GET /status`, live comments reach a beta overlay.

Token lifecycle: Page tokens from a long-lived user token do not expire (ADR-0060); an invalidated token surfaces as a Graph 401 and the streamer reconnects from the dashboard. token-refresh-service intentionally skips facebook. No rotation procedure.

## Deploy Instagram

Same Meta app as facebook, plus the Instagram product. Streamer needs an Instagram professional (business/creator) account linked to a Facebook Page. Scopes: `instagram_basic`, `instagram_manage_comments`, `pages_show_list`; fold into the same App Review cycle.

1. Complete the facebook section first.
2. Patch in `instagram-app-id`/`instagram-app-secret` (same Meta app values, same patch pattern).
3. Wire them into the auth-service deployment.
4. Deploy the listener (port 8100, single replica, ADR-0062, no HPA). Env: shared block plus `TOKEN_ENCRYPTION_KEY_V1`, `INSTAGRAM_POLLING_INTERVAL_MS: "10000"`, `INSTAGRAM_SOURCE_SYNC_SECONDS: "30"`.
5. Verify: add Instagram source, consent, comments from a live IG broadcast flow to a beta overlay.

Token lifecycle: same as facebook; token-refresh-service has nothing to do here.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `CreateContainerConfigError` on a new listener | Uppercase key names or a `redis-auth` Secret in the manifest | Use the lowercase live keys from the env block; drop `REDIS_PASSWORD` |
| "SOURCE_MANAGER_SECRET not set - leadership coordination disabled" | Env missing or key mismatch | Harmless for single replicas (every replica ingests); fix the env anyway |
| "FACEBOOK_APP_ID not set" in facebook-listener | Expected; creds live in auth-service | Not an error |
| "Facebook OAuth will not be available" in auth-service | Meta creds unset | Steps 2-3 of the facebook section |
| Graph 401 on a Page poll | Streamer's token invalidated | Streamer reconnects from the dashboard |
| No messages after adding a source | `platform_*` feature gate closed (ADR-0008) or leader has not synced | Flip the gate (feature-gate admin endpoint) or wait one sync interval (30s) |
| Altstream connected but only `: ping` frames | Empty/missing `lastId` (ADR-0082) | Listener bug; the SSE URL must always carry a non-empty `lastId` |
| Altstream 502 on SSE connect | Altstream restarting | Transient; listener backs off and reconnects |
