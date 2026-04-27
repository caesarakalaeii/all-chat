---
phase: 14-secret-rotation-infrastructure
plan: 08
type: execute
wave: 3
depends_on:
  - "14-06"
  - "14-07"
files_modified:
  - docs/runbooks/secret-rotation.md
  - docs/runbooks/db-password-rotation.md
autonomous: true
decisions_addressed:
  - D-13
  - D-14
  - D-15
  - D-18
  - D-19
must_haves:
  truths:
    - "docs/runbooks/secret-rotation.md exists and covers all four secret classes (TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET, DB password)"
    - "Every step uses `kubectl patch` for K8s Secret edits; `sops set` is explicitly forbidden in the doc text"
    - "Every step references the SOPS-vs-live drift hazard with a link to the project_secrets_drift memory"
    - "Pre-flight checklist verifies live secret keys via `kubectl get -o jsonpath='{.data}' | jq 'keys'` (NOT `-o yaml`)"
    - "Each rotation has explicit rollback steps"
    - "DB password rotation is a separate doc that documents the CNPG D-14 fallback (CNPG ManagedRoles unsuitable for app-user rotation)"
  artifacts:
    - path: "docs/runbooks/secret-rotation.md"
      provides: "Operator runbook for TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET rotations"
      min_lines: 250
    - path: "docs/runbooks/db-password-rotation.md"
      provides: "Operator runbook for DB password rotation (CNPG fallback per D-14)"
      min_lines: 100
  key_links:
    - from: "docs/runbooks/secret-rotation.md"
      to: "services/auth-service/cmd/key-rotator/ + caesar-deployment/.../key-rotator-cronjob.yaml"
      via: "documents how the operator triggers a manual sweep using `kubectl create job --from=cronjob/key-rotator-weekly`"
      pattern: "kubectl create job --from=cronjob/key-rotator-weekly"
    - from: "docs/runbooks/db-password-rotation.md"
      to: "RESEARCH.md §2 fallback runbook"
      via: "7-step procedure (ALTER ROLE → kubectl patch → rolling restart) — documented as canonical operator action"
      pattern: "ALTER ROLE allchat_user PASSWORD"
---

<objective>
Document the operator runbook for each rotation type. The doc is the deliverable — there is no production rotation in this phase (that is the FIRST RUN of the new tooling, executed manually by the operator after Phase 14 ships per D-18).

Purpose: Implements decisions D-13 (CNPG path investigated → fallback chosen), D-14 (fallback runbook), D-15 (`kubectl patch`, NOT `sops set`), D-18 (mechanism shipped, rotation deferred to first run), D-19 (leaked keys remain valid during Phase 14 development; mitigation is execution speed not architecture).

Output:
- `docs/runbooks/secret-rotation.md` — covers TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET rotation sequences end-to-end.
- `docs/runbooks/db-password-rotation.md` — covers the DB password rotation per RESEARCH.md §2 fallback (CNPG ManagedRoles is unsuitable; manual ALTER ROLE + rolling restart is the safe path).

Style follows `docs/migrations/2025-02-auth-token-encryption.md` (the model doc per PATTERNS.md and the user's project skill `doc-migration.md`).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md
@.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md
@.planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md
@.planning/phases/14-secret-rotation-infrastructure/14-06-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-07-SUMMARY.md
@docs/migrations/2025-02-auth-token-encryption.md

<interfaces>
<!-- Style template — copy structure from this doc. -->

From docs/migrations/2025-02-auth-token-encryption.md (per PATTERNS.md analog):
```
# [Title]

## Prerequisites
[numbered list]

## Deployment Flow
[numbered steps]

## Tool Usage
[command examples with flags table]

## Rollback Plan
[numbered steps: detect → restore → redeploy → validate]
```

Operational hazard reference (must cite in every K8s Secret edit step):
- `~/.claude/projects/-home-moersener-Hobby-all-chat/memory/project_secrets_drift.md` — `allchat-secrets` is OutOfSync (31 live keys vs 11 in SOPS source). `sops set` overwrites the live secret with the 11-key snapshot, deleting 20 live keys.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Write secret-rotation runbook covering TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET</name>
  <files>docs/runbooks/secret-rotation.md</files>
  <read_first>
    - docs/migrations/2025-02-auth-token-encryption.md (the style template)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §7 "K8s Rotation Runbook — Per Secret Class" (lines 575–680)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md "Common Pitfalls" section (lines 901–933)
    - .planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md decisions D-15 (kubectl patch never sops set)
  </read_first>
  <action>
    Step 1 — Verify the runbooks directory exists; create if not:
    ```
    mkdir -p /home/moersener/Hobby/all-chat/docs/runbooks
    ```

    Step 2 — Create `docs/runbooks/secret-rotation.md`. Use this structure (the body must be substantial — at least 250 lines of actionable content):

    ```markdown
    # All-Chat Secret Rotation Runbook

    **Owner:** Platform / SRE
    **Last reviewed:** <date>
    **Applies to:** TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET
    **Prerequisites:** Phase 14 (Secret Rotation Infrastructure) shipped — versioned encryption + JWT KeyChain present in all services.

    For DB password rotation, see `db-password-rotation.md`.

    ---

    ## Operational Hazards (READ FIRST)

    1. **`allchat-secrets` SOPS-vs-live drift.** The K8s Secret has more keys
       live than are tracked in `caesar-deployment/apps/workloads/all-chat/secrets/allchat-secret.enc.yaml`
       (31 live vs 11 in SOPS at last audit; verify with the pre-flight check below).
       NEVER use `sops set` or `sops edit` — they overwrite the live secret with
       the SOPS snapshot, silently deleting 20 keys including potentially the one
       you just added.
    2. **NEVER `kubectl get secret -o yaml`.** It writes secret values into the
       `last-applied-configuration` annotation, which leaks via `kubectl describe`.
       Use `kubectl get secret allchat-secrets -o jsonpath='{.data}' | jq 'keys'`
       to inspect WHICH keys exist without reading values.
    3. Service JWT TTL is 15 minutes; User/Viewer JWT TTL is 24 hours
       (`JWT_EXPIRY_HOURS` configmap). Plan rotation timelines accordingly.

    ## Pre-flight Checklist

    Run these before starting ANY rotation:

    ```bash
    # 1. Confirm cluster context
    kubectl config current-context  # MUST be "default" for the allchat cluster

    # 2. Inspect live secret keys (no values)
    kubectl get secret allchat-secrets -n allchat \
      -o jsonpath='{.data}' | jq 'keys'
    # Expected to include (post-Phase-14): token-encryption-key, token-encryption-key-v1,
    # jwt-secret, jwt-secret-v1, service-jwt-secret, service-jwt-secret-v1, ...

    # 3. Confirm all consumer pods healthy
    kubectl get pods -n allchat -l '!job-name'
    # All in Running state; recent restartCount low.

    # 4. Capture current sweeper Job state
    kubectl get cronjob/key-rotator-weekly -n allchat
    kubectl get jobs -n allchat -l app=key-rotator
    ```

    ---

    # 1. TOKEN_ENCRYPTION_KEY Rotation

    ## Consumers (verify before rotating)

    | Service | Env Var | Secret Key |
    |---------|---------|------------|
    | auth-service | TOKEN_ENCRYPTION_KEY | token-encryption-key |
    | overlay-manager | TOKEN_ENCRYPTION_KEY | token-encryption-key |
    | token-refresh-service | TOKEN_ENCRYPTION_KEY | token-encryption-key |
    | twitch-eventsub-listener | TOKEN_ENCRYPTION_KEY | token-encryption-key |
    | kick-listener | TOKEN_ENCRYPTION_KEY | token-encryption-key |
    | youtube-listener-innertube | (none — uses InnerTube no-token path) | — |

    Each of the above ALSO mounts `TOKEN_ENCRYPTION_KEY_V1` etc. via separate
    `secretKeyRef`. The legacy `TOKEN_ENCRYPTION_KEY` env stays mounted as the
    backwards-compat fallback.

    ## Rotation Sequence

    ### Step 1 — Generate the new V<n+1> key

    ```bash
    # Determine current latest kid by inspecting auth-service env at runtime:
    kubectl exec -n allchat deploy/auth-service -- printenv | grep TOKEN_ENCRYPTION_KEY_V | sort
    # If you see only TOKEN_ENCRYPTION_KEY_V1, the next is V2.

    NEW_KEY=$(head -c 32 /dev/urandom | base64)
    NEXT_VERSION=2  # adjust based on current latest
    SECRET_KEY_NAME="token-encryption-key-v${NEXT_VERSION}"
    ```

    ### Step 2 — Add the new key to allchat-secrets via kubectl patch (NEVER sops set)

    ```bash
    ENCODED=$(echo -n "$NEW_KEY" | base64)
    kubectl patch secret allchat-secrets -n allchat \
      --type='json' \
      -p="[{\"op\": \"add\", \"path\": \"/data/${SECRET_KEY_NAME}\", \"value\": \"${ENCODED}\"}]"

    # Verify the new key is present:
    kubectl get secret allchat-secrets -n allchat \
      -o jsonpath='{.data}' | jq 'keys' | grep "$SECRET_KEY_NAME"
    ```

    ### Step 3 — Update deployment manifests in caesar-deployment to mount the new env var

    Edit each `*-deployment.yaml` file in `caesar-deployment/apps/workloads/all-chat/`
    that mounts `TOKEN_ENCRYPTION_KEY` and add a parallel block:
    ```yaml
    - name: TOKEN_ENCRYPTION_KEY_V<n+1>
      valueFrom:
        secretKeyRef:
          name: allchat-secrets
          key: token-encryption-key-v<n+1>
    ```
    Commit, push, let ArgoCD sync. Confirm rollout:
    ```bash
    kubectl rollout status deployment/auth-service -n allchat --timeout=5m
    kubectl rollout status deployment/overlay-manager -n allchat --timeout=5m
    kubectl rollout status deployment/token-refresh-service -n allchat --timeout=5m
    kubectl rollout status deployment/twitch-eventsub-listener -n allchat --timeout=5m
    kubectl rollout status deployment/kick-listener -n allchat --timeout=5m
    ```
    After rollout: every consumer issues new ciphertext under kid V<n+1>; reads transparently handle V<n>, V<n+1>, and legacy.

    ### Step 4 — Run the sweeper Job to re-encrypt long-tail rows

    ```bash
    # Dry run FIRST to size the change:
    kubectl create job --from=cronjob/key-rotator-weekly key-rotator-dryrun-$(date +%s) -n allchat -- \
      /app/key-rotator --dry-run --batch-size=100 --batch-delay-ms=50

    kubectl logs -n allchat -l app=key-rotator --tail=200 | jq .
    # Inspect: rows_re_encrypted per table.

    # If sane, run live sweep:
    kubectl create job --from=cronjob/key-rotator-weekly key-rotator-rotate-$(date +%s) -n allchat
    kubectl wait --for=condition=complete job -l app=key-rotator -n allchat --timeout=60m
    ```

    ### Step 5 — Wait for max(token TTL) before retiring V<n>

    For TOKEN_ENCRYPTION_KEY, the encrypted data lives in the DB and is migrated
    by the sweeper, so there is no TTL-based wait. Once Step 4 completes successfully,
    no row in the database has kid V<n> remaining (verify by sample SELECT).

    ### Step 6 — Retire V<n>

    Update deployment manifests to remove the legacy `TOKEN_ENCRYPTION_KEY` env block
    (the V<n> = previous latest). Commit, ArgoCD sync, rolling restart.

    ```bash
    # Remove the legacy entry from allchat-secrets (only after rolling restart confirms no consumer reads it):
    kubectl patch secret allchat-secrets -n allchat \
      --type='json' \
      -p="[{\"op\": \"remove\", \"path\": \"/data/token-encryption-key\"}]"
    ```

    ## Rollback (TOKEN_ENCRYPTION_KEY)

    1. **Detect:** services failing to decrypt OAuth tokens; error logs include
       `decrypt: cipher: message authentication failed`.
    2. **Restore:** revert the deployment manifest commit. ArgoCD will redeploy
       the previous env-block configuration. The OLD key (`token-encryption-key`)
       remains in `allchat-secrets` until Step 6 above is run, so rollback is
       valid until then.
    3. **Validate:** restart auth-service; confirm token-refresh-service health
       endpoint returns 200; sample OAuth login flow.

    ---

    # 2. JWT_SECRET Rotation

    ## Consumers (verify)

    | Service | Env Var | Secret Key | Role |
    |---------|---------|------------|------|
    | auth-service | JWT_SECRET / JWT_SECRET_V<n> | jwt-secret / jwt-secret-v<n> | Issuer + Validator |
    | api-gateway | JWT_SECRET / JWT_SECRET_V<n> | jwt-secret / jwt-secret-v<n> | Validator |
    | share-service | JWT_SECRET / JWT_SECRET_V<n> | jwt-secret / jwt-secret-v<n> | Validator |
    | overlay-manager | JWT_SECRET / JWT_SECRET_V<n> | jwt-secret / jwt-secret-v<n> | Validator |

    ## Rotation Sequence

    ### Step 1 — Generate new V<n+1>

    ```bash
    NEW_JWT_SECRET=$(head -c 32 /dev/urandom | base64)
    NEXT_VERSION=2
    SECRET_KEY_NAME="jwt-secret-v${NEXT_VERSION}"
    ENCODED=$(echo -n "$NEW_JWT_SECRET" | base64)
    ```

    ### Step 2 — Add to allchat-secrets

    ```bash
    kubectl patch secret allchat-secrets -n allchat \
      --type='json' \
      -p="[{\"op\": \"add\", \"path\": \"/data/${SECRET_KEY_NAME}\", \"value\": \"${ENCODED}\"}]"
    ```

    ### Step 3 — Update deployment manifests

    Add `JWT_SECRET_V<n+1>` env block to all four consumer deployments. Commit,
    ArgoCD sync, rolling restart.

    ### Step 4 — Issuer flips to V<n+1>

    The auth-service `KeyChain` reads `JWT_SECRET_V<n+1>` and `LatestKid()` returns
    `"v<n+1>"` automatically once the env var is present after restart. New tokens
    are signed with kid V<n+1>; validators accept V<n>, V<n+1>, and legacy.

    ### Step 5 — Wait T+24h

    User JWT TTL is 24h (per `JWT_EXPIRY_HOURS` configmap; verify with
    `kubectl describe cm allchat-config -n allchat | grep JWT_EXPIRY_HOURS`).

    All tokens issued under V<n> expire by `T+24h` after Step 4 completes.

    ### Step 6 — Retire V<n>

    Remove `JWT_SECRET_V<n>` env entry from deployment manifests. Commit, ArgoCD
    sync, rolling restart. Then remove the secret key:
    ```bash
    kubectl patch secret allchat-secrets -n allchat \
      --type='json' \
      -p="[{\"op\": \"remove\", \"path\": \"/data/jwt-secret-v${PREVIOUS_VERSION}\"}]"
    ```
    The kid-less legacy `JWT_SECRET` (and `jwt-secret` key) stays in place
    until ALL versioned keys before V<n+1> have been retired AND no kid-less
    tokens are still in flight (which they cannot be after the FIRST rotation
    since the issuer set kid headers from then on).

    ## Rollback (JWT_SECRET)

    1. **Detect:** users get 401 on protected endpoints after Step 4 deploy;
       client tokens fail with "invalid or expired token".
    2. **Restore:** revert the deployment manifest commit. The V<n> key remains in
       `allchat-secrets` until Step 6, so reverting is valid.
    3. **Validate:** sample GET /api/v1/users/me with a known-good token; check
       api-gateway logs for `auth.ValidateJWTWithKeyChain` errors.

    ---

    # 3. SERVICE_JWT_SECRET Rotation

    ## Consumers (verify)

    | Service | Env Var | Secret Key | Role |
    |---------|---------|------------|------|
    | source-manager | SERVICE_JWT_SECRET / _V<n> | service-jwt-secret / -v<n> | Validator |
    | api-gateway | SERVICE_JWT_SECRET / _V<n> | same | Validator (post-Phase-14 bugfix) |
    | share-service | SERVICE_JWT_SECRET / _V<n> | same | Issuer (post-Phase-14 bugfix) + Validator |
    | twitch-eventsub-listener | SERVICE_JWT_SECRET / _V<n> | same | Issuer + Validator |
    | twitch-listener | SERVICE_JWT_SECRET / _V<n> | same | Issuer (via SDK) |
    | kick-listener | SERVICE_JWT_SECRET / _V<n> | same | Issuer (via SDK) |
    | tiktok-listener | SERVICE_JWT_SECRET / _V<n> | same | Issuer (via SDK) |
    | youtube-listener-innertube | SERVICE_JWT_SECRET / _V<n> + SOURCE_MANAGER_SECRET | same | Issuer (via SDK) |
    | discord-listener | SERVICE_JWT_SECRET / _V<n> + SOURCE_MANAGER_SECRET | same | Issuer (via SDK) |

    ## Rotation Sequence

    Identical structure to JWT_SECRET (Steps 1–6 above), but:
    - Wait window in Step 5 is **15 minutes** (Service JWT TTL), not 24 hours.
    - share-service issues with 30-second TTL; effectively no wait beyond 30s for
      that path. The 15min wait covers all other listeners.
    - The `SOURCE_MANAGER_SECRET` legacy alias (used by youtube-listener-innertube
      and discord-listener) maps to the same `service-jwt-secret` key — it is
      retired in lockstep with `SERVICE_JWT_SECRET` legacy. After both are removed,
      only the V<n+1> key is mounted.

    ## Rollback (SERVICE_JWT_SECRET)

    1. **Detect:** listener pods 401 from `/internal/*` routes; source-manager
       admin endpoints reject service tokens; share-link generation fails.
    2. **Restore:** revert the deployment manifest commit; V<n> key remains in
       allchat-secrets until Step 6.
    3. **Validate:** check source-manager logs for `ServiceJWTAuth` errors;
       sample source-manager admin endpoint with a fresh listener service token.

    ---

    # 4. Sweeper Operations

    ## Manual run

    ```bash
    kubectl create job --from=cronjob/key-rotator-weekly key-rotator-manual-$(date +%s) -n allchat
    kubectl logs -n allchat -l job-name=key-rotator-manual-<ts> -f | jq .
    ```

    ## Dry run

    Edit the Job spec to add `--dry-run` to args, OR:
    ```bash
    kubectl create -f - <<EOF
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: key-rotator-dryrun-$(date +%s)
      namespace: allchat
    spec:
      template:
        spec:
          containers:
            - name: key-rotator
              image: ghcr.io/caesarakalaeii/allchat-auth-service:latest
              command: ["/app/key-rotator"]
              args: ["--dry-run", "--batch-size=100"]
              env:
                # Same env block as key-rotator-job.yaml
              ...
    EOF
    ```

    ## Skip a table

    `--skip-table=tiktok_oauth_tokens` (default — Node.js scope deferral).
    `--skip-table=overlay_tts_configs` (if BYTEA scan path is buggy in this build).

    ## Telemetry

    Sweeper emits structured zap logs at end of each table:
    ```json
    {"level":"info","ts":"...","msg":"table sweep complete","table":"users","scanned":1234,"re_encrypted":1200,"skipped":34,"errors":0}
    ```
    Aggregate via `kubectl logs ... | jq 'select(.msg=="table sweep complete")'`.

    ## Common Pitfalls (from Phase 14 RESEARCH.md)

    - **Pitfall 5 (BYTEA scan):** `overlay_tts_configs.encrypted_api_key` is BYTEA, not TEXT. The sweeper handles this; if upgrading the sweeper, re-test BYTEA path.
    - **Pitfall 3 (false-positive kid):** legacy ciphertext whose first byte coincidentally equals a registered kid byte triggers AEAD failure → fallback to legacy key. The library handles this; logs may show "decrypt: cipher: message authentication failed" briefly per affected row — this is expected and recoverable.

    ---

    ## Appendix: Cluster Context Verification

    ```bash
    kubectl config get-contexts | grep "\\*" | awk '{print $2}'  # current context
    # Must equal "default" per global memory reference_k8s_context.md
    ```

    ## Appendix: SOPS Drift Reconciliation (NOT Phase 14)

    Reconciling the 31-vs-11 SOPS drift is OUT OF SCOPE for Phase 14. The
    operational policy in this runbook ASSUMES the drift continues to exist —
    that's why every step uses `kubectl patch`, not `sops set`.

    When drift is reconciled in a future phase, this runbook can be simplified
    to use SOPS as the single source of truth. Until then, `kubectl patch` it is.
    ```

    Step 3 — Validate the file is well-formed Markdown:
    ```
    test -f /home/moersener/Hobby/all-chat/docs/runbooks/secret-rotation.md
    wc -l /home/moersener/Hobby/all-chat/docs/runbooks/secret-rotation.md
    ```
    Expected: ≥ 250 lines.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && test -f docs/runbooks/secret-rotation.md && [ "$(wc -l < docs/runbooks/secret-rotation.md)" -ge 250 ] && grep -q "kubectl patch" docs/runbooks/secret-rotation.md && grep -q "NEVER use \`sops set\`\|NEVER \`sops set\`\|never use \`sops set\`\|sops set\|sops edit" docs/runbooks/secret-rotation.md && ! grep -q "kubectl get secret.*-o yaml" docs/runbooks/secret-rotation.md && grep -q "TOKEN_ENCRYPTION_KEY" docs/runbooks/secret-rotation.md && grep -q "JWT_SECRET" docs/runbooks/secret-rotation.md && grep -q "SERVICE_JWT_SECRET" docs/runbooks/secret-rotation.md && grep -q "kubectl create job --from=cronjob/key-rotator-weekly" docs/runbooks/secret-rotation.md</automated>
  </verify>
  <acceptance_criteria>
    - `test -f docs/runbooks/secret-rotation.md`
    - `[ $(wc -l < docs/runbooks/secret-rotation.md) -ge 250 ]`
    - `grep -q "kubectl patch" docs/runbooks/secret-rotation.md` (Step 2 of every rotation uses kubectl patch)
    - `grep -q "sops set" docs/runbooks/secret-rotation.md` AND `grep -q "NEVER\|never" docs/runbooks/secret-rotation.md` (the "NEVER sops set" warning is explicit)
    - `! grep -q 'kubectl get secret.*-o yaml' docs/runbooks/secret-rotation.md` (the leaky pattern is NOT in the runbook)
    - `grep -q "TOKEN_ENCRYPTION_KEY\|token-encryption-key" docs/runbooks/secret-rotation.md`
    - `grep -q "JWT_SECRET\|jwt-secret" docs/runbooks/secret-rotation.md`
    - `grep -q "SERVICE_JWT_SECRET\|service-jwt-secret" docs/runbooks/secret-rotation.md`
    - `grep -q "key-rotator-weekly\|key-rotator" docs/runbooks/secret-rotation.md`
    - `grep -q "Rollback\|rollback" docs/runbooks/secret-rotation.md` (each rotation has a rollback section)
    - `grep -q "project_secrets_drift\|31.*11\|drift" docs/runbooks/secret-rotation.md` (drift hazard explicitly noted)
    - `grep -q "T+24h\|T+15m\|max(token_TTL)\|TTL" docs/runbooks/secret-rotation.md` (D-09 retire timeline documented)
  </acceptance_criteria>
  <done>secret-rotation.md exists, ≥250 lines, covers TOKEN_ENCRYPTION_KEY + JWT_SECRET + SERVICE_JWT_SECRET. Every K8s Secret edit uses kubectl patch. The leaky `-o yaml` pattern is absent. Rollback steps exist for each rotation type. SOPS drift hazard called out.</done>
</task>

<task type="auto">
  <name>Task 2: Write db-password-rotation runbook (CNPG fallback per D-14)</name>
  <files>docs/runbooks/db-password-rotation.md</files>
  <read_first>
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §2 "CNPG ManagedRoles" + fallback runbook (lines 169–276)
    - .planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md decisions D-13, D-14, D-15
    - docs/runbooks/secret-rotation.md (Task 1 output — for cross-reference)
  </read_first>
  <action>
    Step 1 — Create `docs/runbooks/db-password-rotation.md`. Body should be substantial (≥100 lines) and follow the exact runbook from RESEARCH.md §2 verbatim:

    ```markdown
    # All-Chat DB Password Rotation Runbook

    **Owner:** Platform / SRE
    **Last reviewed:** <date>
    **Applies to:** PostgreSQL `allchat_user` password (the application user that all Go services use to connect to the CNPG cluster)
    **Prerequisites:** None — this runbook does NOT depend on Phase 14 application code changes; it is operationally distinct from token/JWT rotation.

    ## Why a Manual Runbook (not CNPG ManagedRoles)?

    Per Phase 14 RESEARCH.md §2: CNPG `ManagedRoles` is suitable for *creating*
    the app user declaratively but NOT for zero-downtime password rotation. CNPG
    issues a single `ALTER ROLE ... PASSWORD ...` atomically; running pods lose
    DB auth as soon as the reconcile fires, before their `DATABASE_PASSWORD` env
    is updated. CNPG does NOT trigger app-pod rolling restarts.

    Therefore: rotate the password ourselves, in a controlled order, with
    `kubectl rollout restart` providing the propagation window.

    ## Operational Hazards (READ FIRST)

    1. NEVER `sops set` `allchat-secrets`. See `secret-rotation.md` Operational
       Hazards #1.
    2. NEVER `kubectl get secret -o yaml`. See `secret-rotation.md` Operational
       Hazards #2.
    3. The CNPG `allchat-cluster-secret` is the SUPERUSER bootstrap secret —
       DO NOT rotate it as part of this runbook. The app user
       (`allchat_user`) is separate.

    ## Pre-flight

    ```bash
    # Confirm context
    kubectl config current-context  # must be "default"

    # Confirm CNPG primary
    kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster
    PRIMARY=$(kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster,cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')
    echo "Primary: $PRIMARY"

    # Confirm allchat_user exists
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat -c "\du allchat_user"
    ```

    ## Rotation Sequence (7 Steps)

    ### Step 1 — Generate the new password

    ```bash
    NEW_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -d '/=+')
    echo "New password length: ${#NEW_PASSWORD}"
    # Save NEW_PASSWORD in a transient secrets manager (1Password, Bitwarden) BEFORE
    # proceeding. Lose it now and you'll be running the recovery procedure.
    ```

    ### Step 2 — Change the password on the existing user (preferred)

    Two options. Recommended (single user, atomic password change):
    ```bash
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "ALTER ROLE allchat_user PASSWORD '$NEW_PASSWORD';"
    ```

    Alternative (dual-window with new user — only if you need overlapping access):
    ```bash
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "CREATE ROLE allchat_user_new LOGIN PASSWORD '$NEW_PASSWORD';"
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "GRANT ALL PRIVILEGES ON DATABASE allchat TO allchat_user_new;"
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "GRANT ALL ON SCHEMA public TO allchat_user_new;"
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO allchat_user_new;"
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO allchat_user_new;"
    ```
    Use the alternative only if you've confirmed every consumer reads
    `DATABASE_USER` from the secret (so swapping the user name in the secret will
    propagate). The simple `ALTER ROLE` is the default path.

    ### Step 3 — Patch allchat-secrets (NEVER sops set)

    ```bash
    ENCODED_PASSWORD=$(echo -n "$NEW_PASSWORD" | base64)
    kubectl patch secret allchat-secrets -n allchat \
      --type='json' \
      -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${ENCODED_PASSWORD}\"}]"

    # Verify (key only, never value):
    kubectl get secret allchat-secrets -n allchat \
      -o jsonpath='{.data.database-password}' | base64 -d | wc -c
    # Expected: 43-44 chars (32-byte base64 minus padding)
    ```

    ### Step 4 — Wait 60 seconds

    Allow `secretKeyRef` propagation to all kubelets. K8s caches secrets at the
    node level; new pod starts use the new value immediately, but already-running
    pods don't re-read until restart.

    ### Step 5 — Rolling restart of every service that mounts `database-password`

    ```bash
    for d in auth-service token-refresh-service twitch-eventsub-listener api-gateway overlay-manager share-service message-processor source-manager kick-listener tiktok-listener twitch-listener; do
      kubectl rollout restart deployment/$d -n allchat
    done

    for d in auth-service token-refresh-service twitch-eventsub-listener api-gateway overlay-manager share-service message-processor source-manager kick-listener tiktok-listener twitch-listener; do
      kubectl rollout status deployment/$d -n allchat --timeout=5m
    done
    ```

    Note: `youtube-listener-innertube` and `discord-listener` typically do not
    use the DB; verify by `kubectl describe deploy/$NAME | grep -A 1 database-password`
    before adding to the list.

    ### Step 6 — Verify connectivity

    ```bash
    kubectl exec -n allchat deploy/auth-service -- wget -qO- http://localhost:8081/health/ready
    # Expected: HTTP 200 with body indicating DB+Redis healthy

    # Sample login flow (replace with actual test credentials)
    # Confirm no `pq: password authentication failed for user "allchat_user"` in logs:
    kubectl logs -n allchat deploy/auth-service --tail=100 | grep -i "auth.*fail\|password.*fail" || echo "No auth failures observed"
    ```

    ### Step 7 — Cleanup (alternative path only)

    If Step 2 used the dual-window alternative, drop the old user:
    ```bash
    kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
      -c "DROP ROLE IF EXISTS allchat_user_old;"  # adjust name
    ```
    For the recommended `ALTER ROLE` path, no cleanup is needed.

    ## Rollback

    1. **Detect:** services log `pq: password authentication failed` after Step 5.
       Pods may CrashLoopBackOff if readiness probe depends on DB.
    2. **Restore:**
       ```bash
       # Re-issue the OLD password — you saved it before Step 1, right?
       OLD_PASSWORD="<value from password manager>"
       kubectl exec -n allchat $PRIMARY -- psql -U postgres allchat \
         -c "ALTER ROLE allchat_user PASSWORD '$OLD_PASSWORD';"

       # Patch secret back:
       OLD_ENCODED=$(echo -n "$OLD_PASSWORD" | base64)
       kubectl patch secret allchat-secrets -n allchat \
         --type='json' \
         -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${OLD_ENCODED}\"}]"

       # Rolling restart again
       for d in auth-service token-refresh-service ...; do
         kubectl rollout restart deployment/$d -n allchat
       done
       ```
    3. **Validate:** auth-service `/health/ready` returns 200; sample DB query.

    ## Schedule

    Recommended cadence: every 90 days OR upon staff change with DB credential
    knowledge. Phase 14 ships the mechanism; the FIRST rotation per D-18 is the
    operator running this runbook against the leaked password.
    ```

    Step 2 — Validate the file exists and has ≥100 lines:
    ```
    test -f /home/moersener/Hobby/all-chat/docs/runbooks/db-password-rotation.md
    wc -l /home/moersener/Hobby/all-chat/docs/runbooks/db-password-rotation.md
    ```
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && test -f docs/runbooks/db-password-rotation.md && [ "$(wc -l < docs/runbooks/db-password-rotation.md)" -ge 100 ] && grep -q "ALTER ROLE allchat_user PASSWORD" docs/runbooks/db-password-rotation.md && grep -q "kubectl patch secret allchat-secrets" docs/runbooks/db-password-rotation.md && grep -q "rollout restart" docs/runbooks/db-password-rotation.md && grep -q "CNPG.*ManagedRoles\|ManagedRoles" docs/runbooks/db-password-rotation.md && ! grep -q "kubectl get secret.*-o yaml" docs/runbooks/db-password-rotation.md</automated>
  </verify>
  <acceptance_criteria>
    - `test -f docs/runbooks/db-password-rotation.md`
    - `[ $(wc -l < docs/runbooks/db-password-rotation.md) -ge 100 ]`
    - `grep -q "ALTER ROLE allchat_user PASSWORD" docs/runbooks/db-password-rotation.md`
    - `grep -q "kubectl patch secret allchat-secrets" docs/runbooks/db-password-rotation.md`
    - `grep -q "rollout restart" docs/runbooks/db-password-rotation.md` (Step 5 explicit)
    - `grep -q "ManagedRoles\|CNPG ManagedRoles" docs/runbooks/db-password-rotation.md` (D-13/D-14 rationale documented)
    - `! grep -q "kubectl get secret.*-o yaml" docs/runbooks/db-password-rotation.md`
    - `grep -q "Rollback\|rollback" docs/runbooks/db-password-rotation.md`
    - `grep -q "ALTER ROLE.*PASSWORD\|password authentication failed" docs/runbooks/db-password-rotation.md` (rollback example present)
  </acceptance_criteria>
  <done>db-password-rotation.md exists, ≥100 lines, follows RESEARCH.md §2 fallback runbook verbatim. CNPG ManagedRoles unsuitability documented. Rollback path provided.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Operator → cluster | The runbook is the operator's manual interface to rotation; correctness here is paramount |
| Doc → secrets | Doc must NOT include literal example secret values; only commands that GENERATE secrets locally |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-08-01 | Information Disclosure | Doc accidentally includes a literal example key/password | mitigate | Acceptance criterion: no `secret:` literal nor `password:` literal in either runbook (only `${NEW_KEY}` placeholders). Reviewer checks before merge. |
| T-14-08-02 | Tampering | Operator copy-pastes `kubectl get secret -o yaml` thinking it's safe | mitigate | The runbook EXPLICITLY forbids `-o yaml` and uses `-o jsonpath='{.data}' \| jq 'keys'` everywhere; acceptance criterion enforces absence of the leaky form |
| T-14-08-03 | Tampering | Operator runs `sops set` to add the new key | mitigate | "Operational Hazards (READ FIRST)" section opens with the SOPS drift warning; every Step 2 explicitly says "kubectl patch (NEVER sops set)" |
| T-14-08-04 | Repudiation | Operator forgets to wait T+24h before retiring V<n>, breaking active user sessions | mitigate | Step 5 of JWT_SECRET rotation has a dedicated "Wait T+24h" subsection with the configmap verification command; rollback section explicitly notes "users get 401" as the detection signal |
| T-14-08-05 | Denial of Service | DB password rotation: pod restarts during rolling restart hit a node where secret hasn't propagated yet | mitigate | Step 4 includes a 60-second pre-restart wait; this matches RESEARCH.md Pitfall 6 |
| T-14-08-06 | Denial of Service | Rolling restart starts BEFORE `kubectl patch` propagates → pods come up with old password and immediately restart | mitigate | Sequence is patch (Step 3) → wait 60s (Step 4) → rollout restart (Step 5); enforced by Pitfall-6 RESEARCH.md citation |
| T-14-08-07 | Repudiation | Operator loses the new password between generation (Step 1) and persistence (Step 3) → unrecoverable cluster state | mitigate | Step 1 says "Save NEW_PASSWORD in a transient secrets manager BEFORE proceeding" with a callout. Rollback Step 2 also depends on retaining the OLD password — the runbook is explicit. |
</threat_model>

<verification>
- Both runbook files exist; line counts ≥250 (secret-rotation) and ≥100 (db-password).
- All grep acceptance criteria pass.
- No `kubectl get secret -o yaml` patterns appear anywhere in either doc.
- All four rotation classes covered (TOKEN, JWT, SERVICE_JWT, DB password).
- Each rotation class has an explicit Rollback section.
- SOPS drift hazard explicitly cited at the top of each doc.
</verification>

<success_criteria>
- `docs/runbooks/secret-rotation.md` (≥250 lines) covers TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET with end-to-end procedures + rollback.
- `docs/runbooks/db-password-rotation.md` (≥100 lines) covers the CNPG-fallback DB password rotation per D-14 + rollback.
- Every K8s Secret edit uses `kubectl patch`; `sops set` is explicitly forbidden.
- The leaky `kubectl get secret -o yaml` pattern is NEVER suggested.
- D-09 retire timeline (T+max(token_TTL)) documented per JWT class.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-08-SUMMARY.md` documenting:
- Both runbook files committed.
- Decision: NO live rotation executed in Phase 14 — per D-18, rotation is the operator's manual first-run after Phase 14 ships. The leaked keys remain valid during this development per D-19; mitigation is execution speed.
- Note: SOPS drift reconciliation is a deferred follow-up phase (out of scope for Phase 14).
- Note: the actual execution of these runbooks against the leaked production keys is a HUMAN-UAT item that should be scheduled immediately after Phase 14 deploys.
</output>
