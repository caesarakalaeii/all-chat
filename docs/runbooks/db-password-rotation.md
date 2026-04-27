# All-Chat DB Password Rotation Runbook

**Owner:** Platform / SRE
**Last reviewed:** 2026-04-27
**Applies to:** PostgreSQL `allchat_user` password — the application-level database user
that all Go services use to connect to the CNPG cluster `allchat-cluster`.
**Prerequisites:** None — this runbook does NOT depend on Phase 14 application code changes.
DB password rotation is operationally independent from token/JWT rotation.

See `docs/runbooks/secret-rotation.md` for TOKEN\_ENCRYPTION\_KEY, JWT\_SECRET, and
SERVICE\_JWT\_SECRET rotation procedures.

---

## When to Use This Runbook

| Trigger | Action |
|---------|--------|
| Proactive / scheduled rotation (90-day cadence) | Full 7-step sequence |
| Staff change — someone with DB credential knowledge has left | Full 7-step sequence immediately |
| Credential confirmed or suspected compromised | Full 7-step sequence immediately |
| First post-Phase-14 rotation (initial rotate of leaked `allchat_user` password) | Full 7-step sequence — per decision D-18, Phase 14 builds the mechanism; THIS is the first run |

---

## Why a Manual Runbook (Not CNPG ManagedRoles)?

Per Phase 14 RESEARCH.md §2 (decision D-13/D-14): CNPG `ManagedRoles` is suitable for
**creating** the app user declaratively, but NOT for zero-downtime password rotation.

| Criterion | CNPG ManagedRoles verdict |
|-----------|--------------------------|
| Dual-password window so running pods don't drop mid-rotation | **FAIL** — CNPG issues a single `ALTER ROLE … PASSWORD …` atomically on reconcile. No grace window. Running pods lose DB auth immediately when the reconcile fires, before their `DATABASE_PASSWORD` env is refreshed. |
| Services pick up new password without manual deployment intervention | **FAIL** — `DATABASE_PASSWORD` is injected via `secretKeyRef` at pod-start time. Running pods do not see the new value until they are restarted. CNPG does not trigger rolling restarts of application pods. |
| No application code changes required | PASS — once pods restart with the new env value, no code changes are needed. |

**Therefore:** We own the rotation window ourselves. The safe sequence is:
`ALTER ROLE` → `kubectl patch secret` → wait for propagation → rolling restart → verify.

Design rationale: see `caesar-deployment/apps/workloads/all-chat/allchat-cluster.yaml`
(no `ManagedRoles` section) and Phase 14 RESEARCH.md §2.

---

## Operational Hazards — READ FIRST

### Hazard 1: Never `sops set`

The K8s Secret `allchat-secrets` has 31 live keys but only 11 in the SOPS source. Running
`sops set`, `sops edit`, or `sops updatekeys` on
`caesar-deployment/apps/workloads/all-chat/secrets/allchat-secret.enc.yaml` silently
overwrites the live secret with the SOPS snapshot, deleting 20 keys. See
`project_secrets_drift.md` and `docs/runbooks/secret-rotation.md` Hazard 1.

**Always use `kubectl patch` for K8s Secret edits.**

### Hazard 2: Never inspect secrets with the `-o yaml` output format

Using the `-o yaml` (or `-o json`) output format on a K8s Secret writes secret values into
the `last-applied-configuration` annotation, leaking them via `kubectl describe`. Use the
safe inspection pattern instead (keys only, no values):

```bash
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys'
```

### Hazard 3: Do NOT rotate the CNPG superuser secret

`allchat-cluster-secret` is the CNPG bootstrap superuser secret (used by the operator and
replicas). It is NOT the application user secret. This runbook covers ONLY the
`allchat_user` application user. Never use this runbook against the superuser credentials.

### Hazard 4: Capture the OLD password before changing anything

If `ALTER ROLE` succeeds but a later step fails, you need the old password for rollback.
Retrieve it from 1Password / Bitwarden before starting — or note it from the current
`allchat-secrets.database-password` value (decode via a trusted isolated terminal; do not
paste into chat logs or commit history).

---

## Pre-flight Checklist

```bash
# 1. Confirm cluster context
kubectl config current-context
# Expected: "default" (per reference_k8s_context.md memory)

# 2. Inspect current live secret keys (keys only — no values)
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys'
# Confirm "database-password" is present

# 3. Confirm CNPG cluster and identify the primary
kubectl --context default get pods -n allchat \
  -l cnpg.io/cluster=allchat-cluster
# Identify the primary pod (role: primary):

PRIMARY=$(kubectl --context default get pods -n allchat \
  -l cnpg.io/cluster=allchat-cluster,cnpg.io/instanceRole=primary \
  -o jsonpath='{.items[0].metadata.name}')
echo "Primary pod: $PRIMARY"

# 4. Confirm allchat_user exists on the primary
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat -c "\du allchat_user"
# Expected: row showing allchat_user with Login role

# 5. Confirm all consumer pods are healthy before starting
kubectl --context default get pods -n allchat -l '!job-name' \
  -o wide | grep -v Running | grep -v NAME && echo "WARNING: unhealthy pods found" \
  || echo "All pods Running"
# Investigate any non-Running pods before proceeding
```

---

## Rotation Sequence — 7 Steps

### Step 1 — Generate the new password

```bash
# 32 bytes of entropy; strip characters that cause psql quoting issues
NEW_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -d '/=+')
echo "New password length: ${#NEW_PASSWORD}"
# Expected: 43 characters

# IMPORTANT: Save NEW_PASSWORD in your password manager (1Password / Bitwarden)
# RIGHT NOW before proceeding. If you lose it after ALTER ROLE and before Step 3,
# you will need emergency DB access to recover.
```

### Step 2 — Change the password on `allchat_user` (recommended: ALTER ROLE, atomic)

**Option A — Recommended (atomic, in-place password change):**

```bash
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "ALTER ROLE allchat_user PASSWORD '$NEW_PASSWORD';"
# Expected output: ALTER ROLE
```

The old password becomes invalid immediately. There is a brief window (Steps 2–5) during
which running pods still hold the old password in their connection pools. This window is
acceptable — TCP connections to PostgreSQL already established survive; new connection
attempts from restarted pods will pick up the new password from Step 3 onward.

**Option B — Dual-window (create a new user, then swap; only when zero connection drop is mandatory):**

```bash
# Create new user with new password:
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "CREATE ROLE allchat_user_v2 LOGIN PASSWORD '$NEW_PASSWORD';"

# Grant identical privileges:
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON DATABASE allchat TO allchat_user_v2;"
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "GRANT ALL ON SCHEMA public TO allchat_user_v2;"
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO allchat_user_v2;"
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO allchat_user_v2;"
```

For Option B you must also update `DATABASE_USER` in `allchat-secrets` alongside
`database-password`, and verify every service reads `DATABASE_USER` dynamically. This adds
complexity. **Use Option A unless you have a specific reason for Option B.**

### Step 3 — Patch `allchat-secrets` (NEVER sops set)

```bash
# Canonical: kubectl patch secret allchat-secrets -n allchat --type=json -p="[...]"
ENCODED_PASSWORD=$(printf '%s' "$NEW_PASSWORD" | base64)
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${ENCODED_PASSWORD}\"}]"

# Verify the key is present and the encoded length looks right (key name only — no value):
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep database-password
# Expected: "database-password" in the key list

# Optional length sanity check (decodes the base64 value — run in a trusted terminal only):
kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data.database-password}' | base64 -d | wc -c
# Expected: 43 or 44 (depends on trailing newline handling)
```

### Step 4 — Wait 60 seconds for kubelet secret propagation

K8s caches secrets at the node level. Already-running pods do not pick up the new value
until they restart (Step 5), but new pods started during the rolling restart will use the
new value from the moment they start. Wait 60 seconds to allow all kubelets to pick up
the new version of the secret before triggering pod restarts.

```bash
echo "Waiting 60s for kubelet propagation..."
sleep 60
```

### Step 5 — Rolling restart of all DB-consuming services

```bash
# List of all services known to mount database-password (verified from deployment manifests):
DB_SERVICES=(
  auth-service
  token-refresh-service
  twitch-eventsub-listener
  api-gateway
  overlay-manager
  share-service
  message-processor
  source-manager
  kick-listener
  tiktok-listener
  twitch-listener
)

# Trigger rolling restart for each:
for d in "${DB_SERVICES[@]}"; do
  echo "Restarting $d..."
  kubectl --context default rollout restart deployment/$d -n allchat
done

# Wait for all rollouts to complete:
for d in "${DB_SERVICES[@]}"; do
  kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
done
echo "All services restarted successfully"
```

Verify that `youtube-listener-innertube` and `discord-listener` do NOT use the DB by
checking their deployment manifests before adding them to the list:

```bash
kubectl --context default get deployment youtube-listener-innertube -n allchat \
  -o jsonpath='{.spec.template.spec.containers[0].env[*].name}' \
  | tr ' ' '\n' | grep -i database || echo "No DATABASE_ env vars — skip this service"
```

### Step 6 — Verify connectivity and zero authentication failures

```bash
# auth-service health (DB + Redis readiness):
kubectl --context default exec -n allchat deploy/auth-service -- \
  wget -qO- http://localhost:8081/health/ready
# Expected: HTTP 200 with {"status":"ok","db":"ok","redis":"ok"} or equivalent

# Scan recent logs for PostgreSQL auth failures:
for d in auth-service overlay-manager token-refresh-service; do
  echo "=== $d ==="
  kubectl --context default logs -n allchat deploy/$d --tail=50 \
    | grep -iE "password authentication failed|allchat_user|pq:.*auth" \
    || echo "No auth failures"
done
```

### Step 7 — Cleanup and documentation

**For Option A (ALTER ROLE — recommended):** No DB cleanup needed. The password is
replaced atomically; the old credentials are gone.

**For Option B (dual-user) only:**

```bash
# After all pods are verified on allchat_user_v2, drop the old user:
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "DROP ROLE IF EXISTS allchat_user;"
# Rename new user to canonical name (requires no active connections):
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "ALTER ROLE allchat_user_v2 RENAME TO allchat_user;"
```

**SOPS drift register:** The new `database-password` value now exists only in the live
K8s secret — not in the SOPS source. Document this in the project secrets drift register
and in the incident/change log. The drift from `allchat-secret.enc.yaml` grows by one more
key that diverges. This is expected and tracked; reconciliation is a separate future phase.

---

## Rollback

If services begin logging `pq: password authentication failed for user "allchat_user"` or
pods enter `CrashLoopBackOff` due to DB connection failures:

1. **Detect:** Check logs immediately after Step 5.

   ```bash
   kubectl --context default logs -n allchat deploy/auth-service --tail=100 \
     | grep -iE "password authentication failed|allchat_user|connection.*refused|connect.*fail"
   ```

2. **Restore — re-issue the OLD password on the DB:**

   ```bash
   # You saved the OLD password before Step 1, right?
   OLD_PASSWORD="<value from password manager>"

   kubectl --context default exec -n allchat "$PRIMARY" -- \
     psql -U postgres allchat \
     -c "ALTER ROLE allchat_user PASSWORD '$OLD_PASSWORD';"
   # Expected: ALTER ROLE
   ```

3. **Restore — revert the K8s Secret patch:**

   ```bash
   OLD_ENCODED=$(printf '%s' "$OLD_PASSWORD" | base64)
   kubectl --context default patch secret allchat-secrets -n allchat \
     --type='json' \
     -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${OLD_ENCODED}\"}]"
   ```

4. **Rolling restart to pick up the restored secret:**

   ```bash
   for d in auth-service token-refresh-service twitch-eventsub-listener \
             api-gateway overlay-manager share-service message-processor \
             source-manager kick-listener tiktok-listener twitch-listener; do
     kubectl --context default rollout restart deployment/$d -n allchat
   done
   for d in auth-service overlay-manager api-gateway; do
     kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
   done
   ```

5. **Validate:**

   ```bash
   kubectl --context default exec -n allchat deploy/auth-service -- \
     wget -qO- http://localhost:8081/health/ready
   # Expected: HTTP 200
   ```

---

## Failure Modes and Mitigations

| Failure | Symptom | Mitigation |
|---------|---------|-----------|
| Pod starts AFTER `ALTER ROLE` but BEFORE `kubectl patch` completes (Step 2–3 race) | New pod gets old `database-password` from secret but DB has new password → immediate auth failure | Always complete Step 3 (patch secret) immediately after Step 2 (ALTER ROLE). The 60s wait in Step 4 is AFTER Step 3, not between Steps 2 and 3. |
| Kubelet secret cache stale at pod restart | Pod picks up old password from kubelet cache despite secret being patched | Step 4's 60s wait covers normal cache refresh. If pods still fail after restart, force a second restart — kubelet will have refreshed by then. |
| Connection pool exhaustion during rolling restart | Remaining old-password pods hold connections; new pods fail to connect during brief overlap | All services use pgx connection pooling with `MaxConns`; brief exhaustion is transient and resolves as old pods terminate. Alert threshold: watch for `connection refused` in a 2-minute window. |
| PodDisruptionBudget blocks rolling restart | Restart stalls because PDB prevents all pods from restarting | `kubectl --context default get pdb -n allchat` — if a PDB is blocking, check its `minAvailable`; the rolling restart respects it. If critical, drain one pod manually then let rollout proceed. |

---

## Recommended Rotation Cadence

- **Every 90 days** (quarterly), or
- **Immediately** upon staff departure or credential exposure.

Phase 14 ships the mechanism. The **first rotation is the operator's post-Phase-14 manual
execution of this runbook against the previously leaked `allchat_user` password** (per
decisions D-18 and D-19 in `14-CONTEXT.md`).

---

## Related Documentation

- `docs/runbooks/secret-rotation.md` — TOKEN\_ENCRYPTION\_KEY, JWT\_SECRET,
  SERVICE\_JWT\_SECRET rotation
- `.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §2` — CNPG
  ManagedRoles verdict and original 7-step fallback runbook
- `.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md` — decisions D-13
  (CNPG path), D-14 (fallback chosen), D-15 (kubectl patch not sops set), D-18 (first
  rotation timing), D-19 (leaked key acceptance during Phase 14 development)
- `caesar-deployment/apps/workloads/all-chat/allchat-cluster.yaml` — CNPG Cluster spec
  (no ManagedRoles section; bootstrap owner pattern used instead)
