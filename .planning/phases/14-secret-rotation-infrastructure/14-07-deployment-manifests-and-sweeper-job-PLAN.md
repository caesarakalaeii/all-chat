---
phase: 14-secret-rotation-infrastructure
plan: 07
type: execute
wave: 3
depends_on:
  - "14-04"
  - "14-05"
  - "14-06"
files_modified:
  - caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml
  - caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml
  - caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml
  - caesar-deployment/apps/workloads/all-chat/kustomization.yaml
autonomous: true
decisions_addressed:
  - D-02
  - D-04
  - D-06
  - D-08
  - D-10
must_haves:
  truths:
    - "Every consumer of TOKEN_ENCRYPTION_KEY also has TOKEN_ENCRYPTION_KEY_V1 mounted"
    - "Every consumer of JWT_SECRET also has JWT_SECRET_V1 mounted"
    - "Every consumer of SERVICE_JWT_SECRET also has SERVICE_JWT_SECRET_V1 mounted"
    - "token-refresh-service and twitch-eventsub-listener: env var renamed from ENCRYPTION_KEY to TOKEN_ENCRYPTION_KEY (Pitfall 1)"
    - "key-rotator Job and CronJob manifests exist; kustomization.yaml includes them"
    - "kick-listener has TOKEN_ENCRYPTION_KEY + TOKEN_ENCRYPTION_KEY_V1 (NEW — gained encryption in Plan 14-05)"
  artifacts:
    - path: "caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml"
      provides: "Manual-trigger Job for sweeper"
      contains: "kind: Job"
    - path: "caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml"
      provides: "Weekly CronJob for sweeper (Sunday 03:00 UTC)"
      contains: "kind: CronJob"
  key_links:
    - from: "every *-deployment.yaml"
      to: "allchat-secrets keys: token-encryption-key-v1, jwt-secret-v1, service-jwt-secret-v1"
      via: "secretKeyRef per env var (no envFrom)"
      pattern: "secretKeyRef.*key:.*-v1"
---

<objective>
Roll out deployment manifest changes that expose the new versioned secret keys (`token-encryption-key-v1`, `jwt-secret-v1`, `service-jwt-secret-v1`) to every running service, reconcile Pitfall 1 (env name inconsistency for token-refresh-service and twitch-eventsub-listener), and add Kubernetes `Job` + `CronJob` manifests for the sweeper.

Purpose: Implements the deployment side of D-02 (multi-key chain in env), D-08 (kid-aware validators), D-10 (independent service chain), D-06 (sweeper as Job/CronJob).

Output: every Go service deployment YAML gains the appropriate `_V1` env entries; legacy keys remain in place as backwards-compat fallback per D-05; new key-rotator manifests are kustomization-included.

This plan does NOT mutate the live K8s Secret. Adding the secret keys to `allchat-secrets` (the actual key material) is a runbook step in Plan 14-08 that uses `kubectl patch`. This plan ships the YAML scaffolding.
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
@.planning/phases/14-secret-rotation-infrastructure/14-04-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-05-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-06-SUMMARY.md
@caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml
@caesar-deployment/apps/workloads/all-chat/kustomization.yaml

<interfaces>
The `secretKeyRef` per-env-var pattern (no envFrom):
```yaml
- name: <ENV_VAR_NAME>
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: <secret-key-in-kebab-case>
```

Pitfall 1 — token-refresh-service and twitch-eventsub-listener currently have `name: ENCRYPTION_KEY` even though Plan 14-04 changed the code to read `TOKEN_ENCRYPTION_KEY` (via `NewMultiKeyEncryptorFromEnv`). This task fixes the env var name.
</interfaces>

## Per-Service Manifest Map

| Service | Add | Rename | Notes |
|---------|-----|--------|-------|
| auth-service | TOKEN_ENCRYPTION_KEY_V1, JWT_SECRET_V1 | — | |
| api-gateway | JWT_SECRET_V1, SERVICE_JWT_SECRET (NEW), SERVICE_JWT_SECRET_V1 | — | service JWT validation was buggy until Plan 14-05; manifest must mount service chain too |
| overlay-manager | TOKEN_ENCRYPTION_KEY_V1, JWT_SECRET_V1 | — | |
| share-service | JWT_SECRET_V1, SERVICE_JWT_SECRET (NEW), SERVICE_JWT_SECRET_V1 | — | bugfix: now issues service JWTs with service chain |
| token-refresh-service | TOKEN_ENCRYPTION_KEY_V1 | ENCRYPTION_KEY → TOKEN_ENCRYPTION_KEY | Pitfall 1 |
| twitch-eventsub-listener | TOKEN_ENCRYPTION_KEY_V1, SERVICE_JWT_SECRET_V1 | ENCRYPTION_KEY → TOKEN_ENCRYPTION_KEY | Pitfall 1 |
| source-manager | SERVICE_JWT_SECRET_V1 | — | |
| twitch-listener | SERVICE_JWT_SECRET_V1 | — | |
| kick-listener | TOKEN_ENCRYPTION_KEY (NEW), TOKEN_ENCRYPTION_KEY_V1, SERVICE_JWT_SECRET_V1 | — | first-time encryption consumer per Plan 14-05 |
| tiktok-listener | SERVICE_JWT_SECRET_V1 | — | NO encryption env (Node.js scope) |
| youtube-listener-innertube | SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — | currently mounts SOURCE_MANAGER_SECRET only |
| discord-listener | SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — | same |

</context>

<tasks>

<task type="auto">
  <name>Task 1: Add _V1 env entries to auth-service, api-gateway, overlay-manager, share-service, source-manager + Pitfall 1 fix in token-refresh-service and twitch-eventsub-listener</name>
  <files>caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml, caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml, caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml, caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml, caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml, caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml, caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml</files>
  <read_first>
    - caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml lines 160–185
    - caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml lines 45–70
    - caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml lines 95–150
    - caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml lines 90–110
    - caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml lines 90–110
    - caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml lines 120–145 (find ENCRYPTION_KEY)
    - caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml lines 80–110 (find ENCRYPTION_KEY + SERVICE_JWT_SECRET)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "K8s deployment files (MODIFY)" section (lines 506–557)
  </read_first>
  <action>
    Step 1 — auth-service-deployment.yaml: After existing `JWT_SECRET` block, insert a new block with `name: JWT_SECRET_V1` and `key: jwt-secret-v1`. After existing `TOKEN_ENCRYPTION_KEY` block, insert `TOKEN_ENCRYPTION_KEY_V1` block with `key: token-encryption-key-v1`.

    Step 2 — api-gateway-deployment.yaml: After existing `JWT_SECRET`, insert `JWT_SECRET_V1`, then insert `SERVICE_JWT_SECRET` (key: service-jwt-secret) AND `SERVICE_JWT_SECRET_V1` (key: service-jwt-secret-v1). The `SERVICE_JWT_SECRET` non-V1 mount is NEW because Plan 14-05 fixed the bug at line 563 (was `jwtSecret`, now `serviceKeyChain`); without this manifest change the service won't start.

    Step 3 — overlay-manager-deployment.yaml: After `JWT_SECRET`, insert `JWT_SECRET_V1`. After `TOKEN_ENCRYPTION_KEY`, insert `TOKEN_ENCRYPTION_KEY_V1`.

    Step 4 — share-service-deployment.yaml: After `JWT_SECRET`, insert `JWT_SECRET_V1`, then `SERVICE_JWT_SECRET` AND `SERVICE_JWT_SECRET_V1`. Same rationale as api-gateway: Plan 14-05's bugfix needs the service chain mounted.

    Step 5 — source-manager-deployment.yaml: After existing `SERVICE_JWT_SECRET` (line 96), insert `SERVICE_JWT_SECRET_V1`.

    Step 6 — token-refresh-service-deployment.yaml: At the existing `ENCRYPTION_KEY` block (line 134), CHANGE the env name to `TOKEN_ENCRYPTION_KEY` (the `secretKeyRef.key: token-encryption-key` is unchanged). Then ADD a `TOKEN_ENCRYPTION_KEY_V1` block (key: token-encryption-key-v1) immediately after.

    Step 7 — twitch-eventsub-listener-deployment.yaml: At the existing `ENCRYPTION_KEY` block (line 89), CHANGE env name to `TOKEN_ENCRYPTION_KEY` AND insert `TOKEN_ENCRYPTION_KEY_V1`. After `SERVICE_JWT_SECRET` (line 94), insert `SERVICE_JWT_SECRET_V1`.

    Step 8 — Validate every modified YAML parses:
    ```
    cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat
    for f in auth-service api-gateway overlay-manager share-service source-manager token-refresh-service twitch-eventsub-listener; do
      python3 -c "import yaml; yaml.safe_load(open('${f}-deployment.yaml'))" || echo "INVALID: ${f}-deployment.yaml"
    done
    ```
    Zero "INVALID" lines expected.

    Step 9 — If `kustomize` is available, render:
    ```
    cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat && kustomize build . > /tmp/all-chat-rendered.yaml 2>&1
    grep -c "TOKEN_ENCRYPTION_KEY_V1\|JWT_SECRET_V1\|SERVICE_JWT_SECRET_V1" /tmp/all-chat-rendered.yaml
    ```
    Expected ≥ 14 (multiple services × multiple keys).
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat && python3 -c "import yaml; [yaml.safe_load(open(f)) for f in ['auth-service-deployment.yaml','api-gateway-deployment.yaml','overlay-manager-deployment.yaml','share-service-deployment.yaml','source-manager-deployment.yaml','token-refresh-service-deployment.yaml','twitch-eventsub-listener-deployment.yaml']]" && grep -q "name: JWT_SECRET_V1" auth-service-deployment.yaml && grep -q "name: TOKEN_ENCRYPTION_KEY_V1" auth-service-deployment.yaml overlay-manager-deployment.yaml token-refresh-service-deployment.yaml twitch-eventsub-listener-deployment.yaml && grep -q "name: SERVICE_JWT_SECRET_V1" api-gateway-deployment.yaml share-service-deployment.yaml source-manager-deployment.yaml twitch-eventsub-listener-deployment.yaml && ! grep -q "name: ENCRYPTION_KEY$" token-refresh-service-deployment.yaml && ! grep -q "name: ENCRYPTION_KEY$" twitch-eventsub-listener-deployment.yaml</automated>
  </verify>
  <acceptance_criteria>
    - All seven YAML files parse cleanly via `python3 -c "import yaml; yaml.safe_load(...)"`.
    - `grep -q "name: JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml`
    - `grep -q "name: TOKEN_ENCRYPTION_KEY_V1" caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET$\|name: SERVICE_JWT_SECRET\b" caesar-deployment/apps/workloads/all-chat/api-gateway-deployment.yaml`
    - `grep -q "name: TOKEN_ENCRYPTION_KEY$\|name: TOKEN_ENCRYPTION_KEY\b" caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml`
    - `! grep -q "name: ENCRYPTION_KEY$" caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml`
    - `grep -q "name: TOKEN_ENCRYPTION_KEY$\|name: TOKEN_ENCRYPTION_KEY\b" caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml`
    - `! grep -q "name: ENCRYPTION_KEY$" caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml`
    - `grep -q "key: token-encryption-key-v1" caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml`
    - `grep -q "key: jwt-secret-v1" caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml`
  </acceptance_criteria>
  <done>All seven service deployments have the appropriate _V1 env entries. Pitfall 1 fixed in token-refresh-service and twitch-eventsub-listener. share-service and api-gateway gain the previously-missing SERVICE_JWT_SECRET non-V1 mount needed for Plan 14-05's bugfix.</done>
</task>

<task type="auto">
  <name>Task 2: Add _V1 env entries to listener deployments + add TOKEN_ENCRYPTION_KEY to kick-listener</name>
  <files>caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml, caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml, caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml, caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml, caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml</files>
  <read_first>
    - caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml (full)
    - caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml (full — gains TOKEN_ENCRYPTION_KEY for the FIRST time per Plan 14-05)
    - caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml (full)
    - caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml (full)
    - caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml (full)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §7 "SERVICE_JWT_SECRET Rotation" (lines 663–679)
    - .planning/phases/14-secret-rotation-infrastructure/14-05-SUMMARY.md (KeyChain wiring topology)
  </read_first>
  <action>
    Step 1 — twitch-listener-deployment.yaml: After existing `SERVICE_JWT_SECRET` (line 80), insert `SERVICE_JWT_SECRET_V1` (key: service-jwt-secret-v1).

    Step 2 — kick-listener-deployment.yaml:
    a) After existing `SERVICE_JWT_SECRET` (line 80), insert `SERVICE_JWT_SECRET_V1`.
    b) Add NEW env entries `TOKEN_ENCRYPTION_KEY` (key: token-encryption-key) and `TOKEN_ENCRYPTION_KEY_V1` (key: token-encryption-key-v1). kick-listener gained encryption capability in Plan 14-05.

    Step 3 — tiktok-listener-deployment.yaml: After existing `SERVICE_JWT_SECRET` (line 80), insert `SERVICE_JWT_SECRET_V1`. Do NOT add `TOKEN_ENCRYPTION_KEY` — Node.js scope deferral (Plan 14-03 + 14-05 confirmed).

    Step 4 — youtube-listener-innertube-deployment.yaml: After existing `SOURCE_MANAGER_SECRET` (line 89), insert BOTH `SERVICE_JWT_SECRET` (key: service-jwt-secret) AND `SERVICE_JWT_SECRET_V1` (key: service-jwt-secret-v1). The legacy `SOURCE_MANAGER_SECRET` mount stays as-is for any SDK code path that still reads it.

    Step 5 — discord-listener-deployment.yaml: Same change as Step 4.

    Step 6 — Validate YAML for each:
    ```
    cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat
    for f in twitch-listener kick-listener tiktok-listener youtube-listener-innertube discord-listener; do
      python3 -c "import yaml; yaml.safe_load(open('${f}-deployment.yaml'))" || echo "INVALID: ${f}-deployment.yaml"
    done
    ```
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat && python3 -c "import yaml; [yaml.safe_load(open(f)) for f in ['twitch-listener-deployment.yaml','kick-listener-deployment.yaml','tiktok-listener-deployment.yaml','youtube-listener-innertube-deployment.yaml','discord-listener-deployment.yaml']]" && grep -q "name: SERVICE_JWT_SECRET_V1" twitch-listener-deployment.yaml kick-listener-deployment.yaml tiktok-listener-deployment.yaml youtube-listener-innertube-deployment.yaml discord-listener-deployment.yaml && grep -q "name: TOKEN_ENCRYPTION_KEY_V1" kick-listener-deployment.yaml</automated>
  </verify>
  <acceptance_criteria>
    - All five YAML files parse cleanly.
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml`
    - `grep -q "name: TOKEN_ENCRYPTION_KEY_V1" caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml`
    - `grep -q "name: TOKEN_ENCRYPTION_KEY\b" caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml` (legacy too)
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml`
    - `! grep -q "name: TOKEN_ENCRYPTION_KEY" caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml` (Node.js scope)
    - `grep -q "name: SERVICE_JWT_SECRET\b" caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml`
    - `grep -q "name: SERVICE_JWT_SECRET_V1" caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml`
  </acceptance_criteria>
  <done>All five listener deployments have SERVICE_JWT_SECRET_V1. kick-listener gains TOKEN_ENCRYPTION_KEY + V1. tiktok-listener intentionally does NOT gain encryption env vars (Node.js scope deferral).</done>
</task>

<task type="auto">
  <name>Task 3: Create key-rotator Job + CronJob manifests + register in kustomization.yaml</name>
  <files>caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml, caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml, caesar-deployment/apps/workloads/all-chat/kustomization.yaml</files>
  <read_first>
    - caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml (env-block pattern for the Job container — uses same image and same secret keys as auth-service)
    - caesar-deployment/apps/workloads/all-chat/kustomization.yaml (the existing resource-list pattern)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §6 "Deployment Shape (K8s Job + CronJob)" (lines 528–569)
    - .planning/phases/14-secret-rotation-infrastructure/14-06-SUMMARY.md (Job env vars list)
  </read_first>
  <action>
    Step 1 — Create `caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml`. Use suspended state by default so it doesn't run on apply; the rotation runbook (Plan 14-08) explicitly creates a one-shot Job from this template:
    ```yaml
    # Manual-trigger Job template for the key-rotator sweeper.
    # NOT applied by default — the rotation runbook creates a Job from this template.
    # To run on demand:
    #   kubectl create job --from=cronjob/key-rotator-weekly key-rotator-manual-$(date +%s) -n allchat
    # OR copy this file, adjust metadata.name, and `kubectl apply -f`.
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: key-rotator-manual
      namespace: allchat
      labels:
        app: key-rotator
        component: secret-rotation
    spec:
      activeDeadlineSeconds: 3600  # 1h max — sweep should complete in <10min normally
      backoffLimit: 2
      ttlSecondsAfterFinished: 86400  # keep finished pod 24h for log retrieval
      template:
        metadata:
          labels:
            app: key-rotator
        spec:
          restartPolicy: OnFailure
          containers:
            - name: key-rotator
              image: ghcr.io/caesarakalaeii/allchat-auth-service:latest
              imagePullPolicy: Always
              command: ["/app/key-rotator"]
              args:
                - "--batch-size=100"
                - "--batch-delay-ms=50"
              env:
                - name: DATABASE_HOST
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-host } }
                - name: DATABASE_PORT
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-port } }
                - name: DATABASE_USER
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-user } }
                - name: DATABASE_PASSWORD
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-password } }
                - name: DATABASE_NAME
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-name } }
                # Composed in entrypoint OR set explicitly:
                - name: DATABASE_URL
                  value: "postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)?sslmode=disable"
                - name: TOKEN_ENCRYPTION_KEY
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: token-encryption-key } }
                - name: TOKEN_ENCRYPTION_KEY_V1
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: token-encryption-key-v1 } }
                - name: YOUTUBE_TOKEN_ENCRYPTION_KEY
                  valueFrom: { secretKeyRef: { name: allchat-secrets, key: youtube-token-encryption-key } }
              resources:
                requests:
                  cpu: 100m
                  memory: 128Mi
                limits:
                  cpu: 500m
                  memory: 256Mi
    ```

    NOTE: The Dockerfile of auth-service must build BOTH `/app/auth-service` AND `/app/key-rotator` binaries. Verify with `grep -n "key-rotator" services/auth-service/Dockerfile` after Plan 14-06 — if it only builds `auth-service`, add a multi-binary build stage. If the Dockerfile is not yet updated, this Job will fail with "exec: no such file" — flag that as a follow-up if found.

    Step 2 — Create `caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml`:
    ```yaml
    apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: key-rotator-weekly
      namespace: allchat
      labels:
        app: key-rotator
        component: secret-rotation
    spec:
      schedule: "0 3 * * 0"  # Sundays 03:00 UTC
      successfulJobsHistoryLimit: 3
      failedJobsHistoryLimit: 1
      concurrencyPolicy: Forbid
      jobTemplate:
        spec:
          activeDeadlineSeconds: 3600
          backoffLimit: 2
          ttlSecondsAfterFinished: 604800  # 7 days
          template:
            metadata:
              labels:
                app: key-rotator
            spec:
              restartPolicy: OnFailure
              containers:
                - name: key-rotator
                  image: ghcr.io/caesarakalaeii/allchat-auth-service:latest
                  imagePullPolicy: Always
                  command: ["/app/key-rotator"]
                  args:
                    - "--batch-size=100"
                    - "--batch-delay-ms=50"
                  env:
                    # SAME env block as the Job above — use a YAML anchor or copy verbatim
                    # ... (same as key-rotator-job.yaml)
                  resources:
                    requests: { cpu: 100m, memory: 128Mi }
                    limits:   { cpu: 500m, memory: 256Mi }
    ```
    Copy the env block verbatim from Step 1 (no anchors — kustomize doesn't always preserve them).

    Step 3 — Update `caesar-deployment/apps/workloads/all-chat/kustomization.yaml` to include the CronJob in the resources list. Inspect the file first:
    ```
    cat /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat/kustomization.yaml
    ```
    Add `- key-rotator-cronjob.yaml` to the resources list.

    DO NOT add `key-rotator-job.yaml` to kustomization — it's a template only. The runbook (Plan 14-08) creates Jobs from the CronJob via `kubectl create job --from=cronjob/key-rotator-weekly`.

    Step 4 — Validate everything parses:
    ```
    cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat
    python3 -c "import yaml; list(yaml.safe_load_all(open('key-rotator-job.yaml'))); list(yaml.safe_load_all(open('key-rotator-cronjob.yaml'))); yaml.safe_load(open('kustomization.yaml'))"
    ```

    Step 5 — Verify kustomize render includes the CronJob (if kustomize is available):
    ```
    cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat && kustomize build . > /tmp/all-chat-rendered.yaml 2>&1
    grep -c "kind: CronJob" /tmp/all-chat-rendered.yaml
    ```
    Expected: ≥ 1 (the new key-rotator CronJob).
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat && python3 -c "import yaml; list(yaml.safe_load_all(open('key-rotator-job.yaml'))); list(yaml.safe_load_all(open('key-rotator-cronjob.yaml'))); yaml.safe_load(open('kustomization.yaml'))" && grep -q "kind: Job" key-rotator-job.yaml && grep -q "kind: CronJob" key-rotator-cronjob.yaml && grep -q "key-rotator-cronjob.yaml" kustomization.yaml</automated>
  </verify>
  <acceptance_criteria>
    - `test -f caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml`
    - `test -f caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml`
    - `grep -q "kind: Job" caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml`
    - `grep -q "kind: CronJob" caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml`
    - `grep -q "schedule: \"0 3 \* \* 0\"\|schedule: '0 3 \* \* 0'" caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml`
    - `grep -q "/app/key-rotator" caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml`
    - `grep -q "TOKEN_ENCRYPTION_KEY_V1" caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml`
    - `grep -q "YOUTUBE_TOKEN_ENCRYPTION_KEY" caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml` (D-04 legacy chain decryption)
    - `grep -q "key-rotator-cronjob.yaml" caesar-deployment/apps/workloads/all-chat/kustomization.yaml`
    - `! grep -q "key-rotator-job.yaml" caesar-deployment/apps/workloads/all-chat/kustomization.yaml` (Job is a template, not applied by default)
    - All YAML files parse via `python3 -c "import yaml"` without exception.
  </acceptance_criteria>
  <done>Job and CronJob manifests exist. kustomization.yaml registers the CronJob (not the Job — Job is a template). Job mounts all required env vars (DB + TOKEN_ENCRYPTION_KEY + TOKEN_ENCRYPTION_KEY_V1 + YOUTUBE_TOKEN_ENCRYPTION_KEY for D-04 legacy decryption).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `allchat-secrets` → service pod env | secretKeyRef per env var; secret values mounted at pod start, refreshed only on rolling restart |
| auth-service image → key-rotator binary | Same Docker image; the Job container runs the `key-rotator` binary instead of `auth-service` |
| ArgoCD reconciliation → live cluster | New manifests must NOT be applied before the secret keys exist in `allchat-secrets` (otherwise pods fail to start) |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-07-01 | Denial of Service | Pods fail to start because `token-encryption-key-v1` doesn't exist in `allchat-secrets` yet | accept | Documented in Plan 14-08 sequence: `kubectl patch` adds the secret keys BEFORE this manifest is applied. ArgoCD sync order is operator-controlled. |
| T-14-07-02 | Tampering | SOPS sync overwrites manually-added secret keys (the 31-vs-11 drift hazard) | mitigate | Phase 14 runbook in Plan 14-08 explicitly uses `kubectl patch` AND documents `database-password`, `token-encryption-key-v1`, `jwt-secret-v1`, `service-jwt-secret-v1` as "manually-managed, drift accepted" per memory `project_secrets_drift.md` |
| T-14-07-03 | Spoofing | key-rotator Job runs with auth-service service account; could be abused to escalate | accept | Same blast radius as auth-service itself; rotator only writes to encrypted columns of token tables. RBAC-tightening deferred per CONTEXT.md "Out of scope" |
| T-14-07-04 | Information Disclosure | Job logs include DATABASE_URL with embedded password | mitigate | `valueFrom.secretKeyRef` — value never appears in `kubectl describe`; Job spec includes only the env-name. Tests confirm via `! grep -q "value:.*password" key-rotator-job.yaml` |
| T-14-07-05 | Repudiation | CronJob accidentally runs during business hours, throttling DB | mitigate | Schedule pinned to Sundays 03:00 UTC; --batch-delay-ms=50 throttle prevents DB saturation; `concurrencyPolicy: Forbid` prevents overlapping runs |
| T-14-07-06 | Tampering | Pitfall 1 partial fix — token-refresh-service rebuilt but env still ENCRYPTION_KEY → service fails to start | mitigate | Both rename AND _V1 are applied in Step 6 of Task 1; `! grep ENCRYPTION_KEY$` acceptance criterion enforces |
| T-14-07-07 | Information Disclosure | API Gateway suddenly mounts `service-jwt-secret` (was previously not mounted; line 563 used jwtSecret) — broader exposure | accept | This was always the intent; the BUG was that the secret was NOT being used. Mounting it now closes the bug. The blast radius doesn't change — same secret was already mounted on listener pods. |
| T-14-07-08 | Tampering | Dockerfile doesn't actually build the key-rotator binary | mitigate | Action Step 1 of Task 3 includes a verification step (`grep -n "key-rotator" services/auth-service/Dockerfile`); if absent, flag for follow-up Dockerfile patch in this plan or in Plan 14-08 |
</threat_model>

<verification>
- All 12 deployment YAMLs and the new key-rotator-job.yaml + key-rotator-cronjob.yaml parse via `python3 -c "import yaml"`.
- All grep acceptance criteria pass.
- `kustomize build` (if available) renders ≥ 1 CronJob and ≥ 14 occurrences of the new env-var names.
- Dockerfile multi-binary build verified (or flagged for follow-up).
</verification>

<success_criteria>
- 12 deployment YAMLs updated with _V1 entries; Pitfall 1 reconciled.
- key-rotator Job + CronJob manifests committed.
- kustomization.yaml registers the CronJob (not the Job — Job is a template).
- Tiktok deployment intentionally does NOT gain TOKEN_ENCRYPTION_KEY (Node.js scope).
- All YAML files parse cleanly.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-07-SUMMARY.md` documenting:
- Per-service inventory of env vars added (delta).
- Pitfall 1 reconciliation: ENCRYPTION_KEY → TOKEN_ENCRYPTION_KEY in both services.
- Dockerfile multi-binary status (whether services/auth-service/Dockerfile already builds key-rotator or needs a follow-up patch).
- The kustomization.yaml inclusion shape: CronJob YES, Job NO (template only).
- Note for Plan 14-08: Before applying these manifest changes via ArgoCD, the runbook must `kubectl patch allchat-secrets` to add the new secret KEYS (token-encryption-key-v1, jwt-secret-v1, service-jwt-secret-v1) — pods will fail to start otherwise.
</output>
