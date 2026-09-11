# New-Listener Deployment Runbook (Owncast, GoodGame, Picarto, Facebook, Rumble)

**Owner:** Platform / SRE
**Last reviewed:** 2026-09-11
**Applies to:** the five platform-expansion listeners (PR to `beta`, ADR-0058–0061)
**Related:** `docs/runbooks/secret-rotation.md` (hazard notes on `allchat-secrets`
SOPS-vs-live drift apply here unchanged — `kubectl patch` only, never
`sops set`/`sops edit` on the encrypted file)

---

## When to Use This Runbook

| Trigger | Action |
|---------|--------|
| Deploy the four secretless listeners (owncast, goodgame, picarto, rumble) | Section 1 — no new secrets needed, manifests need key-name fixes first |
| Deploy the facebook listener | Section 2 — requires a Meta app and two new secret keys first |
| A listener CrashLoops after deploy | Section 4 — the two known failure modes are both key-name drift |

Decision taken 2026-09-11: deploy the secretless four when ready; facebook-listener
waits for the Meta app (business verification takes weeks — start it now if the
platform is wanted at all; see Section 2 step 1).

---

## Secret Requirements per Listener

Third-party credentials needed *at the pod*:

| Listener | Third-party secrets | Verdict |
|---|---|---|
| owncast-listener | none — connects to the streamer's own Owncast instance URL, no API key | deploy as-is |
| goodgame-listener | none — anonymous chat WebSocket | deploy as-is |
| picarto-listener | none — unofficial pop-out chat WebSocket | deploy as-is |
| rumble-listener | none required — anonymous SSE reads work; `RUMBLE_SESSION_COOKIE` is optional (age-gated streams need it, ADR-0061) | deploy as-is |
| facebook-listener | none *in the pod*, but the platform is dead end-to-end without Meta credentials in auth-service + moderation-service | **do not deploy yet** |

Infrastructure secrets every listener needs already exist in
`allchat-secrets` (verified live 2026-09-11): `database-password`,
`service-jwt-secret`, `token-encryption-key-v1` (facebook only). The live
cluster runs Redis **without AUTH** — no listener sets `REDIS_PASSWORD` today.

### Where each shared env comes from (live wiring, not the repo base manifests)

| Env var | Source | Live key name |
|---|---|---|
| `DATABASE_*` | ConfigMap `allchat-config` / Secret `allchat-secrets` | `database-password` (lowercase) |
| `SOURCE_MANAGER_URL` | ConfigMap `allchat-config` | `SOURCE_MANAGER_URL` |
| `SOURCE_MANAGER_SECRET` | Secret `allchat-secrets` | `service-jwt-secret` (lowercase) |
| `REDIS_PASSWORD` | **not set in prod** — omit it | — |
| `COORDINATOR_URL` | ConfigMap `allchat-config` | `SOURCE_MANAGER_URL` (legacy kick-listener env name) |
| `TOKEN_ENCRYPTION_KEY_V1` | Secret `allchat-secrets` | `token-encryption-key-v1` |

---

## Hazard: the repo base manifests will not apply as-is

The five new listener manifests in `deployments/k8s/base/<platform>-listener/`
were written for the local docker/kustomize setup and reference secret keys
that **do not match live `allchat-secrets`**:

| Manifest says | Live secret has | Failure mode if applied verbatim |
|---|---|---|
| `allchat-secrets/DATABASE_PASSWORD` | `database-password` | `CreateContainerConfigError`, pod never starts |
| `allchat-secrets/SERVICE_JWT_SECRET` | `service-jwt-secret` | same |
| `allchat-secrets/RUMBLE_SESSION_COOKIE` | (absent) | none — `optional: true`, kubelet skips it |
| `redis-auth/redis-password` | (no such Secret; Redis runs without AUTH) | `CreateContainerConfigError` |

Deployment-repo manifests must use the live lowercase key names. Pattern to
copy: `caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml`
(keel annotations `keel.sh/policy: force`, `keel.sh/trigger: poll`, image
`ghcr.io/caesarakalaeii/allchat-<name>:beta`, `imagePullPolicy: Always`).

---

## Section 1 — Deploy owncast, goodgame, picarto, rumble

Prereq: PR (platform expansion) merged to `beta`, images
`ghcr.io/caesarakalaeii/allchat-{owncast,goodgame,picarto,rumble}-listener:beta`
exist in ghcr (workflow `build-and-push.yml` tags `type=ref,event=branch` on
push to `beta`).

1. **One Deployment + optional HPA per listener** in
   `caesar-deployment/apps/workloads/all-chat/`, modeled on
   `kick-listener-deployment.yaml`. Per listener:

   - `owncast-listener-deployment.yaml` — port 8095
   - `goodgame-listener-deployment.yaml` — port 8096
   - `picarto-listener-deployment.yaml` — port 8097
   - `rumble-listener-deployment.yaml` — port 8098
   - HPA files optional (repo base ships `minReplicas: 1, maxReplicas: 5` on
     CPU; leadership coordination makes extra replicas safe — every listener
     calls `NewLeadershipListenerFromEnv`, and with `SOURCE_MANAGER_SECRET`
     set only the leader ingests)

2. **Env block per listener** (identical for all four):

   ```yaml
   env:
     - name: PORT
       value: "<8095-8098>"
     - name: LOG_LEVEL
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: LOG_LEVEL
     - name: DATABASE_HOST
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: DATABASE_HOST
     - name: DATABASE_PORT
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: DATABASE_PORT
     - name: DATABASE_NAME
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: DATABASE_NAME
     - name: DATABASE_USER
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: DATABASE_USER
     - name: DATABASE_PASSWORD
       valueFrom:
         secretKeyRef:
           name: allchat-secrets
           key: database-password        # lowercase — matches live
     - name: REDIS_HOST
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: REDIS_HOST
     - name: REDIS_PORT
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: REDIS_PORT
     - name: SOURCE_MANAGER_URL
       valueFrom:
         configMapKeyRef:
           name: allchat-config
           key: SOURCE_MANAGER_URL
     - name: SOURCE_MANAGER_SECRET
       valueFrom:
         secretKeyRef:
           name: allchat-secrets
           key: service-jwt-secret      # lowercase — matches live
   ```

   Rumble only, additionally (optional — pod starts without the key present):

   ```yaml
     - name: RUMBLE_SESSION_COOKIE
       valueFrom:
         secretKeyRef:
           name: allchat-secrets
           key: rumble-session-cookie
           optional: true
   ```

   Skip `REDIS_PASSWORD` entirely (no AUTH in prod, no `redis-auth` Secret).
   Skip `SERVICE_JWT_SECRET`/`_V1` — listeners use `SOURCE_MANAGER_SECRET`.

3. **Add to `kustomization.yaml`** resources list.

4. **Commit to caesar-deployment `main`**, push, let ArgoCD sync the `all-chat`
   app. Watch rollout:

   ```bash
   kubectl --context default rollout status deployment/owncast-listener -n allchat --timeout=5m
   kubectl --context default rollout status deployment/goodgame-listener -n allchat --timeout=5m
   kubectl --context default rollout status deployment/picarto-listener -n allchat --timeout=5m
   kubectl --context default rollout status deployment/rumble-listener -n allchat --timeout=5m
   ```

5. **Verify ingest path** (gate still closed for non-premium users — expected
   until `platform_*` feature gates are flipped, ADR-0008):

   ```bash
   # /health/ready returns 200 (Redis reachable)
   kubectl --context default exec -n allchat deploy/rumble-listener -- \
     wget -qO- http://localhost:8098/health/ready

   # no CrashLoop; restartCount stable at 0
   kubectl --context default get pods -n allchat -l platform=rumble
   ```

   End-to-end message flow additionally needs: a premium (or gate-flipped)
   user adds a source on the beta dashboard → overlay-manager writes the
   source row → listener's leadership coordinator picks it up within its
   sync interval → messages on `chat:raw` → message-processor normalizes →
   overlay renders. Watch with:

   ```bash
   kubectl --context default logs -n allchat deploy/rumble-listener --tail=100
   # expect "leadership coordination" and, once a source exists, SSE connect lines
   ```

---

## Section 2 — Facebook (blocked on Meta app, start early)

The listener pod itself needs **no Facebook credentials** — it reads stored
Page tokens from `facebook_oauth_tokens` (migration 094). But with no Meta app
configured the platform is inert: no streamer can complete the OAuth consent
(auth-service logs "Facebook OAuth will not be available" when
`FACEBOOK_APP_ID`/`FACEBOOK_APP_SECRET` are unset), so the listener would poll
zero Pages forever. Deploying it now is pointless, not dangerous — there is no
crash-loop; do not deploy until step 3 is done.

### Facebook secret keys to add (both required)

| Secret key (allchat-secrets) | Env var | Consumer services |
|---|---|---|
| `facebook-app-id` | `FACEBOOK_APP_ID` | auth-service, moderation-service |
| `facebook-app-secret` | `FACEBOOK_APP_SECRET` | auth-service, moderation-service |

Follow the lower-case live key naming convention (`facebook-app-id`, not
`FACEBOOK_APP_ID`).

### Steps

1. **Create the Meta app** (longest lead time — weeks, incl. business
   verification + App Review for `pages_read_engagement` and
   `pages_manage_engagement`). Full walkthrough:
   `services/facebook-listener/README.md` ("Operator setup (day 1)").
   OAuth redirect URI: `<FRONTEND_URL>/api/v1/auth/facebook/callback`.
2. **Store the credentials** (kubectl patch — never `sops set`, see
   `secret-rotation.md` Hazard 1; and never inspect with `-o yaml`,
   Hazard 2):

   ```bash
   FB_APP_ID='<from Meta dashboard>'
   FB_APP_SECRET='<from Meta dashboard>'
   kubectl --context default patch secret allchat-secrets -n allchat \
     --type='json' -p="[
       {\"op\": \"add\", \"path\": \"/data/facebook-app-id\", \"value\": \"$(printf '%s' "$FB_APP_ID" | base64)\"},
       {\"op\": \"add\", \"path\": \"/data/facebook-app-secret\", \"value\": \"$(printf '%s' "$FB_APP_SECRET" | base64)\"}
     ]"

   # Verify — key names only, no values:
   kubectl --context default get secret allchat-secrets -n allchat \
     -o jsonpath='{.data}' | jq 'keys' | grep facebook
   ```

3. **Wire the env into auth-service and moderation-service** deployment
   manifests (caesar-deployment), then rolling-restart both:

   ```yaml
   - name: FACEBOOK_APP_ID
     valueFrom:
       secretKeyRef:
         name: allchat-secrets
         key: facebook-app-id
   - name: FACEBOOK_APP_SECRET
     valueFrom:
       secretKeyRef:
         name: allchat-secrets
         key: facebook-app-secret
   ```

4. **Deploy the listener** (port 8099, single replica by design — ADR-0060,
   no HPA). Env: the shared block from Section 1 plus
   `TOKEN_ENCRYPTION_KEY_V1` (key `token-encryption-key-v1`) for decrypting
   Page tokens, `FACEBOOK_POLLING_INTERVAL_MS: "10000"`,
   `FACEBOOK_SOURCE_SYNC_SECONDS: "30"`.

5. **Verify**: dashboard → add Facebook source → consent → poller appears in
   `GET /status` of the listener; live comments flow to a beta overlay.

### Token lifecycle note

Page tokens from a long-lived user token do not expire (ADR-0060). An
invalidated token (password change, role removal) surfaces as a Graph 401;
the streamer reconnects from the dashboard. token-refresh-service
intentionally skips facebook (commit 6b4f945a). No rotation procedure needed.

---

## Section 3 — Rumble session cookie (optional, only if age-gated streams matter)

Anonymous SSE reads work without it (ADR-0061). If streams with chat
age-restriction must be supported:

1. Extract the `session_token` cookie value from a logged-in Rumble browser
   session (it is a long-lived bearer for read endpoints).
2. Patch it in:

   ```bash
   kubectl --context default patch secret allchat-secrets -n allchat \
     --type='json' -p="[{\"op\": \"add\", \"path\": \"/data/rumble-session-cookie\", \"value\": \"$(printf '%s' "$COOKIE" | base64)\"}]"
   ```

3. Ensure the deployment mounts it with `optional: true` (already the case in
   both repo base and the Section 1 manifest snippet). The pod picks it up on
   next restart; no rollout trigger needed for an optional key.

The cookie is personal to the operator's Rumble account — treat it as a
credential; it grants read access to that account's subscribed/age-gated
content.

---

## Section 4 — Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `CreateContainerConfigError` on new listener | Manifest references `DATABASE_PASSWORD`/`SERVICE_JWT_SECRET` (uppercase) or `redis-auth/redis-password` | Use lowercase live key names (`database-password`, `service-jwt-secret`), drop `REDIS_PASSWORD` — see Section 1 env block |
| Pod runs, logs "SOURCE_MANAGER_SECRET not set — leadership coordination disabled" | Secret key mismatch or env missing | Every replica then ingests independently — for single-replica deployments harmless, fix the env anyway |
| facebook-listener logs "FACEBOOK_APP_ID not set" | Expected — app creds live in auth-service, not the listener | Not an error; see Section 2 |
| auth-service logs "Facebook OAuth will not be available" | `FACEBOOK_APP_ID`/`FACEBOOK_APP_SECRET` unset | Section 2 step 2-3 |
| Graph 401 on a Page poll | Streamer's Page token invalidated | Streamer reconnects from dashboard; no operator action |
| No messages despite source added | `platform_*` feature gate closed (ADR-0008) — overlay-manager 403s the source add, or the add succeeded but the listener's leader hasn't synced yet (30s sync cadence) | Flip the gate for premium users or per-gate via the feature-gate admin endpoint; then wait one sync interval |
