# All-Chat Secret Rotation Runbook

**Owner:** Platform / SRE
**Last reviewed:** 2026-04-27
**Applies to:** TOKEN\_ENCRYPTION\_KEY, JWT\_SECRET, SERVICE\_JWT\_SECRET
**Prerequisites:** Phase 14 (Secret Rotation Infrastructure) fully deployed — versioned
`MultiKeyEncryptor` and `KeyChain` present in all services; all twelve deployment YAMLs
contain `_V1` env entries; `key-rotator` CronJob registered in kustomization.yaml.

For DB password rotation see `docs/runbooks/db-password-rotation.md`.

---

## When to Use This Runbook

| Trigger | Action |
|---------|--------|
| Proactive / scheduled rotation (TOKEN\_ENCRYPTION\_KEY) | Sections 1 + 4 (Steps 1–6 + sweeper run) |
| Proactive / scheduled rotation (JWT\_SECRET) | Section 2 (Steps 1–6, includes T+24 h wait) |
| Proactive / scheduled rotation (SERVICE\_JWT\_SECRET) | Section 3 (Steps 1–6, T+15 min wait) |
| Suspected or confirmed KEY LEAK | Start with Section 1, 2, or 3 depending on which key is affected; all steps are the same — rotation IS the remediation |
| First post-Phase-14 rotation (initial rotate of leaked keys from the incident) | All of Sections 1, 2, and 3; this is decision D-18 — the mechanism was built to make THIS rotation the first run |
| Retire a legacy kid after deadline | Step 6 of the relevant section |

---

## Operational Hazards — READ FIRST

### Hazard 1: `allchat-secrets` SOPS-vs-live drift

The K8s Secret `allchat-secrets` has **31 keys live** but only **11 keys in the SOPS-encrypted
source** at `caesar-deployment/apps/workloads/all-chat/secrets/allchat-secret.enc.yaml`
(last drift audit). See project memory `project_secrets_drift.md`.

**NEVER run `sops set`, `sops edit`, or `sops updatekeys` on this file.** These commands
overwrite the live secret with the 11-key SOPS snapshot, silently deleting all 20 extra
live keys — including the rotation keys you just added. Every K8s Secret edit in this
runbook uses `kubectl patch` exclusively.

### Hazard 2: Never inspect secrets with the `-o yaml` output format

Using `-o yaml` (or `-o json`) when getting a K8s Secret writes secret values into the
`last-applied-configuration` annotation, which is then visible via `kubectl describe`.
This is a credential leak.

**Always inspect keys without values:**

```bash
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys'
```

This shows which keys exist without exposing any values. The `-o yaml` output format
is explicitly prohibited for any `kubectl get secret` call in this runbook.

### Hazard 3: JWT TTL windows

User/Viewer JWT TTL is 24 hours (`JWT_EXPIRY_HOURS` configmap). Service JWT TTL is 15
minutes. Plan your rotation windows accordingly — the old kid must remain accepted by
validators until all tokens issued under it have expired.

### Hazard 4: TTS overlay JWTs are NOT covered here

Phase 13 per-overlay `tts_signing_secret` (decision D-11) is intentionally excluded from
Phase 14 rotation. Those are regenerated per the `overlay-manager` TTS settings flow
(user copies a fresh OBS link). Do not modify `overlay_tts_configs.tts_signing_secret` via
this runbook.

---

## Pre-flight Checklist

Run ALL of these before starting ANY rotation:

```bash
# 1. Confirm cluster context
kubectl config current-context
# Expected: "default" (allchat cluster per reference_k8s_context.md memory)
# If different, override every kubectl command below with --context <actual-context>

# 2. Inspect live secret keys — keys only, no values
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys'
# Post-Phase-14 expected keys include (at minimum):
#   "token-encryption-key", "token-encryption-key-v1",
#   "jwt-secret", "jwt-secret-v1",
#   "service-jwt-secret", "service-jwt-secret-v1",
#   "database-password", "database-url", "youtube-token-encryption-key"
# If token-encryption-key-v1 / jwt-secret-v1 / service-jwt-secret-v1 are ABSENT,
# you must kubectl patch to add them BEFORE ArgoCD syncs the Phase 14 manifests.
# See Plan 14-07 SUMMARY for the complete pre-ArgoCD-sync procedure.

# 3. Check all consumer pods are healthy before touching secrets
kubectl --context default get pods -n allchat -l '!job-name'
# All in Running state; restartCount low and not increasing

# 4. Check sweeper state
kubectl --context default get cronjob/key-rotator-weekly -n allchat
kubectl --context default get jobs -n allchat -l app=key-rotator
# No stuck jobs in Unknown state; no recent failure counts

# 5. Verify current kid (what key is active for new encryptions)
kubectl --context default exec -n allchat deploy/auth-service -- \
  printenv | grep TOKEN_ENCRYPTION_KEY_V | sort
# If TOKEN_ENCRYPTION_KEY_V1 is listed, current kid is 0x01.
# The next rotation will add V2 (kid 0x02).
```

---

## Section 1: TOKEN\_ENCRYPTION\_KEY Rotation

TOKEN\_ENCRYPTION\_KEY protects OAuth access and refresh tokens stored in the database
(`users`, `viewer_sessions`, `youtube_oauth_tokens`, `kick_oauth_tokens`,
`overlay_tts_configs`). The ciphertext format is `base64([kid(1B)][nonce(12B)][ct][tag(16B)])`.

### Consumer Services

| Service | Env Var | Secret Key |
|---------|---------|------------|
| auth-service | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| auth-service | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| overlay-manager | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| overlay-manager | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| token-refresh-service | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| token-refresh-service | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| twitch-eventsub-listener | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| twitch-eventsub-listener | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| kick-listener | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| kick-listener | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| key-rotator (Job/CronJob) | TOKEN\_ENCRYPTION\_KEY | token-encryption-key |
| key-rotator (Job/CronJob) | TOKEN\_ENCRYPTION\_KEY\_V1 | token-encryption-key-v1 |
| key-rotator (Job/CronJob) | YOUTUBE\_TOKEN\_ENCRYPTION\_KEY | youtube-token-encryption-key |

Note: `youtube-listener-innertube` does NOT use the encryption key (no token writes).
`tiktok-listener` is Node.js scope — deferred (D-17 partial).

### Step 1 — Determine the current latest kid

```bash
# Check what V<n> keys are currently mounted
kubectl --context default exec -n allchat deploy/auth-service -- \
  printenv | grep 'TOKEN_ENCRYPTION_KEY_V' | sort

# Typical output after Phase 14 ships for the first time:
#   TOKEN_ENCRYPTION_KEY_V1=<value>
# => current kid is V1 (byte 0x01); next will be V2 (byte 0x02)

CURRENT_VERSION=1   # adjust if further rotations have already happened
NEXT_VERSION=2      # the new kid number
SECRET_KEY_NAME="token-encryption-key-v${NEXT_VERSION}"
```

### Step 2 — Generate the new V<n+1> key

```bash
# 32 bytes of entropy; base64 is the canonical format for this secret
NEW_KEY=$(head -c 32 /dev/urandom | base64)
echo "New key length: ${#NEW_KEY}"
# Save NEW_KEY in 1Password / Bitwarden before proceeding.
# Lose it now and every row this key encrypts becomes unrecoverable.
```

### Step 3 — Add the new key to `allchat-secrets` via kubectl patch (NEVER sops set)

```bash
ENCODED=$(printf '%s' "$NEW_KEY" | base64)
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"add\", \"path\": \"/data/${SECRET_KEY_NAME}\", \"value\": \"${ENCODED}\"}]"

# Verify the key was added (key name only — no values):
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep "token-encryption-key-v${NEXT_VERSION}"
# Expected: "token-encryption-key-v2" (or the next version number)
```

### Step 4 — Update deployment manifests to mount the new env var

Edit each deployment YAML in `caesar-deployment/apps/workloads/all-chat/` that already
mounts `TOKEN_ENCRYPTION_KEY_V1` and add a parallel block for V<n+1>:

```yaml
# Add after the TOKEN_ENCRYPTION_KEY_V1 block in each consumer's env section:
- name: TOKEN_ENCRYPTION_KEY_V2
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: token-encryption-key-v2
```

Services requiring this change: `auth-service-deployment.yaml`,
`overlay-manager-deployment.yaml`, `token-refresh-service-deployment.yaml`,
`twitch-eventsub-listener-deployment.yaml`, `kick-listener-deployment.yaml`,
`key-rotator-cronjob.yaml`, `key-rotator-job.yaml` (template).

Commit to caesar-deployment, push, let ArgoCD sync. Confirm rollout:

```bash
for d in auth-service overlay-manager token-refresh-service twitch-eventsub-listener kick-listener; do
  kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
done
```

After rollout: every consumer picks up the new key. New writes are encrypted under kid V2.
Existing rows with kid V1 or legacy kid (0x00) continue to decrypt transparently via the
multi-key chain.

### Step 5 — Run the sweeper Job to re-encrypt long-tail rows

```bash
# Dry run FIRST to assess the scope of changes:
kubectl --context default create job \
  key-rotator-dryrun-$(date +%s) \
  --from=cronjob/key-rotator-weekly \
  -n allchat -- /app/key-rotator --dry-run --batch-size=100 --batch-delay-ms=50

kubectl --context default logs -n allchat \
  -l app=key-rotator --tail=500 | jq 'select(.msg=="table sweep complete")'
# Review rows_re_encrypted per table; confirm numbers are sane before live sweep.

# Live sweep (during off-peak hours — Sunday 03:00 UTC is the scheduled window):
kubectl --context default create job \
  key-rotator-rotate-$(date +%s) \
  --from=cronjob/key-rotator-weekly \
  -n allchat

kubectl --context default wait --for=condition=complete \
  job -l app=key-rotator -n allchat --timeout=60m

# Verify zero remaining rows at old kid:
kubectl --context default logs -n allchat \
  -l app=key-rotator --tail=500 | jq 'select(.msg=="table sweep complete")'
# All tables should show rows_re_encrypted = 0 on a second dry run.
```

For the `--skip-table` flag and telemetry format see Section 4 (Sweeper Operations).

### Step 6 — Retire the legacy/V<n> key (after sweeper confirms 0 rows remaining)

Unlike JWT rotation, there is no TTL wait — the sweeper migration is the gate. Once the
sweeper reports 0 re-encryptions on a second dry run, no row in the database holds a
ciphertext under the old kid.

1. Remove the legacy `TOKEN_ENCRYPTION_KEY` env block (and `TOKEN_ENCRYPTION_KEY_V1` if
   V2 is now the sole active key) from deployment manifests. Commit, ArgoCD sync.

2. Remove the old key from `allchat-secrets`:

```bash
# Remove the plain TOKEN_ENCRYPTION_KEY (legacy / V0 era):
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"remove\", \"path\": \"/data/token-encryption-key\"}]"

# Verify it is gone:
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep "token-encryption-key" || echo "key removed"
```

3. Rolling restart to pick up the reduced env:

```bash
for d in auth-service overlay-manager token-refresh-service twitch-eventsub-listener kick-listener; do
  kubectl --context default rollout restart deployment/$d -n allchat
done
for d in auth-service overlay-manager token-refresh-service twitch-eventsub-listener kick-listener; do
  kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
done
```

### Rollback — TOKEN\_ENCRYPTION\_KEY

1. **Detect:** services fail to decrypt OAuth tokens; error logs show
   `decrypt: cipher: message authentication failed` or `token decryption failed`.
2. **Restore:** revert the deployment manifest commit in caesar-deployment. ArgoCD
   redeploys the previous env configuration. The OLD key (`token-encryption-key`) remains
   in `allchat-secrets` until Step 6 is run, so revert is valid until then.
3. **Validate:**
   ```bash
   kubectl --context default rollout status deployment/auth-service -n allchat --timeout=5m
   kubectl --context default exec -n allchat deploy/auth-service -- \
     wget -qO- http://localhost:8081/health/ready
   # Expected: HTTP 200 with {"db":"ok","redis":"ok"}
   ```

---

## Section 2: JWT\_SECRET Rotation

JWT\_SECRET signs User and Viewer JWTs. The `KeyChain` in `shared/auth` picks the latest
kid for new issues and validates any kid present in the active chain.

**User JWT TTL is 24 hours.** You MUST wait `T+24h` after the issuer switches to V<n+1>
before retiring the V<n> kid — active sessions use V<n> tokens that remain valid for up to
24 hours after issue. Retiring early forces all users to re-login.

### Consumer Services

| Service | Env Var | Secret Key | Role |
|---------|---------|------------|------|
| auth-service | JWT\_SECRET | jwt-secret | Issuer + Validator |
| auth-service | JWT\_SECRET\_V1 | jwt-secret-v1 | Issuer (post-Phase-14) |
| api-gateway | JWT\_SECRET | jwt-secret | Validator |
| api-gateway | JWT\_SECRET\_V1 | jwt-secret-v1 | Validator |
| overlay-manager | JWT\_SECRET | jwt-secret | Validator |
| overlay-manager | JWT\_SECRET\_V1 | jwt-secret-v1 | Validator |
| share-service | JWT\_SECRET | jwt-secret | Validator |
| share-service | JWT\_SECRET\_V1 | jwt-secret-v1 | Validator |

### Step 1 — Determine current latest kid

```bash
kubectl --context default exec -n allchat deploy/auth-service -- \
  printenv | grep 'JWT_SECRET_V' | sort
# Expected post-Phase-14: JWT_SECRET_V1=<value>
# Next rotation: add JWT_SECRET_V2

CURRENT_VERSION=1
NEXT_VERSION=2
SECRET_KEY_NAME="jwt-secret-v${NEXT_VERSION}"
```

### Step 2 — Generate the new V<n+1> key

```bash
NEW_JWT_SECRET=$(head -c 32 /dev/urandom | base64)
echo "New secret length: ${#NEW_JWT_SECRET}"
# Save NEW_JWT_SECRET in password manager before proceeding.
ENCODED=$(printf '%s' "$NEW_JWT_SECRET" | base64)
```

### Step 3 — Add to `allchat-secrets` (NEVER sops set)

```bash
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"add\", \"path\": \"/data/${SECRET_KEY_NAME}\", \"value\": \"${ENCODED}\"}]"

# Verify (key name only):
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep "jwt-secret-v${NEXT_VERSION}"
```

### Step 4 — Update deployment manifests

Add `JWT_SECRET_V2` env block to all four consumer deployments in caesar-deployment:

```yaml
- name: JWT_SECRET_V2
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: jwt-secret-v2
```

Services: `auth-service-deployment.yaml`, `api-gateway-deployment.yaml`,
`overlay-manager-deployment.yaml`, `share-service-deployment.yaml`.

Commit, push, let ArgoCD sync. Confirm rollout:

```bash
for d in auth-service api-gateway overlay-manager share-service; do
  kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
done
```

### Step 5 — Verify the issuer is now using V<n+1>

The `KeyChain.LatestKid()` returns `"v2"` automatically once `JWT_SECRET_V2` is present in
env after restart. New tokens carry `kid: "v2"` in their header. Validators accept both
`"v1"` (from `JWT_SECRET_V1`) and `"v2"` (from `JWT_SECRET_V2`) plus the legacy kid-less
fallback.

```bash
# Sample a fresh JWT from auth-service and inspect the kid header:
# (replace with actual test credentials in staging)
TOKEN=$(curl -s -X POST http://localhost:8080/auth/twitch/callback ... | jq -r '.token')
echo "$TOKEN" | cut -d. -f1 | base64 -d 2>/dev/null | jq '.kid'
# Expected: "v2"
```

### Step 6 — Wait T+24h, then retire V<n>

```bash
# Verify current JWT expiry hours (should be 24 or configured value):
kubectl --context default describe cm allchat-config -n allchat | grep JWT_EXPIRY_HOURS
# Expected: JWT_EXPIRY_HOURS=24

# Wait at least 24 hours from the time Step 4 rollout completed.
# ALL tokens issued under kid "v1" expire within this window.
# After T+24h:
```

Remove `JWT_SECRET_V1` env block from the four consumer deployments. Commit, ArgoCD sync.
Then remove the secret key:

```bash
PREVIOUS_VERSION=1
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"remove\", \"path\": \"/data/jwt-secret-v${PREVIOUS_VERSION}\"}]"

# Verify removal:
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep "jwt-secret-v${PREVIOUS_VERSION}" \
  || echo "key removed successfully"
```

The kid-less legacy `JWT_SECRET` and `jwt-secret` key remain until after the FIRST rotation
has completed (there are no kid-less tokens in flight post-rotation). On the SECOND rotation
(V2 → V3), the legacy key can be retired in lockstep with removing `JWT_SECRET_V1`.

### Rollback — JWT\_SECRET

1. **Detect:** users receive HTTP 401 on protected endpoints; client tokens fail with
   `"invalid or expired token"` after Step 4 deploy; api-gateway logs show
   `auth.ValidateJWTWithKeyChain` failures.
2. **Restore:** revert the deployment manifest commit. The V<n> key remains in
   `allchat-secrets` until Step 6 is executed, so the rollback is valid until then.
3. **Validate:**
   ```bash
   curl -H "Authorization: Bearer <known-good-token>" http://localhost:8080/api/v1/users/me
   # Expected: 200 OK, user JSON body
   kubectl --context default logs -n allchat deploy/api-gateway --tail=100 \
     | grep -i "ValidateJWT\|auth.*error" || echo "No JWT validation errors"
   ```

---

## Section 3: SERVICE\_JWT\_SECRET Rotation

SERVICE\_JWT\_SECRET signs service-to-service JWTs (listeners → source-manager, share
service link generation). Service JWT TTL is **15 minutes**. The retirement wait is T+15m
from the point the issuer switches to V<n+1>.

### Consumer Services

| Service | Env Var | Secret Key | Role |
|---------|---------|------------|------|
| source-manager | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Validator |
| api-gateway | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Validator (internal routes) |
| share-service | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer + Validator |
| twitch-eventsub-listener | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer + Validator |
| twitch-listener | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer (via SDK) |
| kick-listener | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer (via SDK) |
| tiktok-listener | SERVICE\_JWT\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer (via SDK) |
| youtube-listener-innertube | SERVICE\_JWT\_SECRET + SOURCE\_MANAGER\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer (via SDK) |
| discord-listener | SERVICE\_JWT\_SECRET + SOURCE\_MANAGER\_SECRET + \_V1 | service-jwt-secret(-v1) | Issuer (via SDK) |

Note: `SOURCE_MANAGER_SECRET` is a legacy alias that maps to the same
`service-jwt-secret` key — it must be retired in lockstep with `SERVICE_JWT_SECRET` legacy
once V<n+1> is the sole active kid. `share-service` issues 30-second TTL share JWTs; those
expire within seconds. The 15-minute wait covers all listener service tokens.

### Steps 1–4 (identical structure to JWT\_SECRET)

Follow Section 2 Steps 1–4, substituting:
- `JWT_SECRET` → `SERVICE_JWT_SECRET`
- `jwt-secret` → `service-jwt-secret`
- All nine consumer deployments instead of four

```bash
SECRET_KEY_NAME="service-jwt-secret-v${NEXT_VERSION}"
```

Services needing manifest updates: `auth-service-deployment.yaml` (no — auth-service is not
a SERVICE\_JWT consumer), `api-gateway-deployment.yaml`, `share-service-deployment.yaml`,
`source-manager-deployment.yaml`, `twitch-eventsub-listener-deployment.yaml`,
`twitch-listener-deployment.yaml`, `kick-listener-deployment.yaml`,
`tiktok-listener-deployment.yaml`, `youtube-listener-innertube-deployment.yaml`,
`discord-listener-deployment.yaml`.

### Step 5 — Wait T+15 min (not T+24h)

Service JWT TTL is 15 minutes. All tokens issued under kid V<n> expire within 15 minutes
of the rollout completing. No user sessions are affected.

```bash
# Confirm SERVICE_JWT_SECRET TTL is indeed 15 minutes:
kubectl --context default exec -n allchat deploy/source-manager -- \
  printenv | grep -i JWT_TTL || echo "TTL not in env — hardcoded 15m in shared/auth"
```

### Step 6 — Retire V<n>

```bash
PREVIOUS_VERSION=1
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"remove\", \"path\": \"/data/service-jwt-secret-v${PREVIOUS_VERSION}\"}]"

# Verify:
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep "service-jwt-secret-v${PREVIOUS_VERSION}" \
  || echo "key removed successfully"
```

Remove `SERVICE_JWT_SECRET_V<n>` env blocks from all nine consumer deployments. Commit,
ArgoCD sync. The legacy `SERVICE_JWT_SECRET` and `SOURCE_MANAGER_SECRET` stay until the
second rotation.

### Rollback — SERVICE\_JWT\_SECRET

1. **Detect:** listener pods receive HTTP 401 from `/internal/*` routes; source-manager
   admin endpoints reject service tokens; share-link generation fails (share-service logs
   show `service JWT validation failed`).
2. **Restore:** revert the deployment manifest commit; V<n> key remains in `allchat-secrets`
   until Step 6 has been run.
3. **Validate:**
   ```bash
   kubectl --context default logs -n allchat deploy/source-manager --tail=100 \
     | grep -i "ServiceJWTAuth\|jwt.*invalid\|jwt.*error" || echo "No service JWT errors"
   # Check a listener can reach source-manager:
   kubectl --context default exec -n allchat deploy/twitch-listener -- \
     wget -qO- http://source-manager:8083/health/ready
   ```

---

## Section 4: Sweeper Operations

The `key-rotator` binary lives at `/app/key-rotator` inside the auth-service Docker image.
It re-encrypts all database rows from old kids to the current kid. It is idempotent — a
second run with the same current kid touches 0 rows.

### Manual run (ad-hoc)

```bash
# Create a one-off Job from the weekly CronJob template.
# Canonical form (without --context, for scripting):
# Short form: kubectl create job --from=cronjob/key-rotator-weekly key-rotator-manual -n allchat
# With explicit context override (recommended):
kubectl --context default create job key-rotator-manual-$(date +%s) --from=cronjob/key-rotator-weekly -n allchat

# Follow logs in real time:
JOB_NAME=$(kubectl --context default get jobs -n allchat -l app=key-rotator \
  --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')
kubectl --context default logs -n allchat -l "job-name=${JOB_NAME}" -f | jq .
```

### Dry run (verify scope before mutating)

```bash
kubectl --context default create job \
  key-rotator-dryrun-$(date +%s) \
  --from=cronjob/key-rotator-weekly \
  -n allchat -- /app/key-rotator --dry-run --batch-size=100
```

Dry-run output example:

```json
{"level":"info","msg":"table sweep complete","table":"users","rows_scanned":1542,"rows_re_encrypted":1238,"rows_skipped":304,"errors":0,"current_kid":2}
{"level":"info","msg":"sweep complete","rows_re_encrypted":{"kick_oauth_tokens":47,"overlay_tts_configs":12,"users":1238,"viewer_sessions":89,"youtube_oauth_tokens":203}}
```

`--dry-run` counts are accurate; no DB mutations occur.

### Available flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Log re-encryption counts without writing to DB |
| `--batch-size` | 200 | Rows per UPDATE batch |
| `--batch-delay-ms` | 25 | Sleep between batches (ms) — reduces DB pressure |
| `--skip-table` | (none) | Skip a specific table; repeat flag for multiple |

Examples:

```bash
# Skip tiktok_oauth_tokens entirely (Node.js scope deferral — already gated in SQL too)
/app/key-rotator --skip-table=tiktok_oauth_tokens

# Skip the BYTEA table if the build has a known BYTEA regression
/app/key-rotator --skip-table=overlay_tts_configs
```

### Aggregate telemetry

```bash
kubectl --context default logs -n allchat \
  -l "job-name=${JOB_NAME}" --tail=500 \
  | jq 'select(.msg=="table sweep complete")'
```

### Common pitfalls (from Phase 14 RESEARCH.md)

- **Pitfall 5 (BYTEA scan):** `overlay_tts_configs.encrypted_api_key` is BYTEA, not TEXT.
  The sweeper casts the BYTEA value to string (the stored base64 ciphertext) and writes
  back as BYTEA. If you upgrade the sweeper binary, re-run `TestSweeper_TTSBytea` to
  verify the BYTEA path still works.
- **Pitfall 3 (false-positive kid):** A legacy ciphertext whose first byte coincidentally
  equals a registered kid byte triggers an AEAD authentication failure, then falls back to
  the legacy key successfully. Log output will include
  `decrypt: cipher: message authentication failed` for these rows — this is expected and
  recoverable. The sweeper increments the error counter for visibility but continues.
- **TikTok v0 rows are intentionally skipped.** Migration 051 sets
  `encryption_version=0` for all existing `tiktok_oauth_tokens` rows. The sweeper
  SQL filter is `WHERE encryption_version >= 1`, so v0 rows are never touched. The
  running Node.js tiktok-listener expects plaintext; encrypting these rows would break it.

---

## Section 5: Verification Reference

After each mutation step, run the service-specific health check:

```bash
# auth-service
kubectl --context default exec -n allchat deploy/auth-service -- \
  wget -qO- http://localhost:8081/health/ready

# api-gateway (readiness via K8s)
kubectl --context default get pods -n allchat -l app=api-gateway \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'

# All services (quick overview)
kubectl --context default get pods -n allchat -l '!job-name' \
  -o wide | grep -v Running | grep -v NAME && echo "All Running" || echo "Some pods not Running"
```

---

## Appendix A: Cluster Context

```bash
kubectl config get-contexts | grep '\*'
# Current context must be "default" (allchat cluster per reference_k8s_context.md)
# If the current context is wrong, override every command with --context <name>
# or switch: kubectl config use-context default
```

---

## Appendix B: SOPS Drift Reconciliation Is Out of Scope for Phase 14

The 31-vs-11 SOPS drift in `allchat-secret.enc.yaml` is tracked in
`project_secrets_drift.md`. Reconciling it is explicitly deferred to a later phase.
Until that phase ships, this runbook assumes the drift persists and uses `kubectl patch`
for every K8s Secret mutation.

When drift reconciliation is complete, this runbook can be simplified to use SOPS as the
single source of truth and `sops set` (or SOPS file edit) for adding keys. Until then,
**kubectl patch is the only safe path**.

---

## Appendix C: Related Documentation

- `docs/runbooks/db-password-rotation.md` — DB password rotation (CNPG fallback)
- `docs/migrations/2025-02-auth-token-encryption.md` — original AES-GCM rollout precedent
- `.planning/phases/14-secret-rotation-infrastructure/14-01-shared-encryption-versioning-SUMMARY.md` — MultiKeyEncryptor design
- `.planning/phases/14-secret-rotation-infrastructure/14-02-shared-auth-jwt-keychain-SUMMARY.md` — KeyChain design
- `.planning/phases/14-secret-rotation-infrastructure/14-04-encryption-callsite-migration-SUMMARY.md` — callsite migration + Pitfall 1 fix
- `.planning/phases/14-secret-rotation-infrastructure/14-05-jwt-validators-and-kick-encryption-gapfill-SUMMARY.md` — JWT middleware migration + cross-chain bug fixes
- `.planning/phases/14-secret-rotation-infrastructure/14-06-key-rotator-sweeper-SUMMARY.md` — sweeper flags + per-table behavior matrix
- `.planning/phases/14-secret-rotation-infrastructure/14-07-deployment-manifests-and-sweeper-job-SUMMARY.md` — which secret keys exist in which deployments; Job/CronJob shapes
- `caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml` — manual Job template
- `caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml` — weekly CronJob (Sunday 03:00 UTC)
