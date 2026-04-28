---
phase: 14-secret-rotation-infrastructure
plan: "08"
subsystem: docs/runbooks
tags:
  - runbook
  - secret-rotation
  - kubernetes
  - cnpg
  - documentation
dependency_graph:
  requires:
    - 14-06   # key-rotator binary + sweeper flags
    - 14-07   # deployment manifests, V1 env entries, Job/CronJob
  provides:
    - docs/runbooks/secret-rotation.md
    - docs/runbooks/db-password-rotation.md
  affects: []
tech_stack:
  added: []
  patterns:
    - "kubectl patch for all K8s Secret edits (never sops set — SOPS drift hazard)"
    - "kubectl create job --from=cronjob/key-rotator-weekly for manual sweeper runs"
    - "kubectl --context default throughout (per reference_k8s_context.md)"
key_files:
  created:
    - docs/runbooks/secret-rotation.md
    - docs/runbooks/db-password-rotation.md
  modified: []
decisions:
  - "D-13/D-14: CNPG ManagedRoles verdict (FAIL — no dual-password window); manual ALTER ROLE + kubectl patch is canonical path; documented in db-password-rotation.md §Why a Manual Runbook"
  - "D-15: kubectl patch not sops set — every K8s Secret edit in both runbooks uses kubectl patch; sops set is explicitly forbidden with SOPS drift hazard explanation"
  - "D-18: NO live rotation executed in Phase 14 — mechanism shipped; first rotation is operator manual execution after Phase 14 deploys; documented as 'first post-Phase-14' trigger in both runbooks' When-to-Use tables"
  - "D-19: leaked keys remain valid during Phase 14 development; the FIRST rotation IS the leaked-key remediation; highlighted in secret-rotation.md When-to-Use and db-password-rotation.md cadence section"
  - "D-11: TTS overlay JWTs explicitly excluded from secret-rotation.md (Hazard 4 section)"
  - "SOPS drift reconciliation deferred: runbooks explicitly state 31-vs-11 drift continues; kubectl patch is the only safe path until reconciliation phase ships"
metrics:
  duration_seconds: 811
  completed_date: "2026-04-27"
  task_count: 2
  file_count: 2
---

# Phase 14 Plan 08: Rotation Runbook Summary

**One-liner:** Operator-ready rotation playbooks for TOKEN\_ENCRYPTION\_KEY/JWT\_SECRET/SERVICE\_JWT\_SECRET (660 lines) and CNPG DB password (379 lines) with concrete kubectl commands, T+TTL wait guidance, rollback steps, and explicit SOPS drift + `-o yaml` leak prohibitions.

## What Was Built

### `docs/runbooks/secret-rotation.md` (660 lines)

Covers all three cryptographic secret classes introduced or extended in Phase 14:

- **Decision matrix ("When to Use")** — proactive rotation, leak response, retired-kid deadlines, and first post-Phase-14 leaked-key rotation (D-18)
- **Operational Hazards (READ FIRST)** — SOPS 31-vs-11 drift, `-o yaml` leak prohibition (safe jsonpath pattern shown), JWT TTL windows, TTS overlay JWT exclusion (D-11)
- **Pre-flight checklist** — context verification, key inspection without values, pod health, sweeper state, current kid identification
- **Section 1: TOKEN\_ENCRYPTION\_KEY** — 6-step rotation (generate → kubectl patch → manifest update + rollout → sweeper dry-run + live run → wait-for-0-rows → retire legacy); consumer table (5 services + key-rotator Job); rollback with detection signals
- **Section 2: JWT\_SECRET** — 6-step rotation with mandatory T+24h wait (user JWT TTL) before retiring V<n>; consumer table (4 services); kid header verification; rollback
- **Section 3: SERVICE\_JWT\_SECRET** — identical structure, T+15 min wait; SOURCE\_MANAGER\_SECRET legacy alias retirement explained; consumer table (9 services); rollback
- **Section 4: Sweeper Operations** — manual run via `kubectl create job --from=cronjob/key-rotator-weekly`, dry-run pattern, all flags table, telemetry jq filter, Pitfall 3 and Pitfall 5 documented
- **Section 5: Verification reference** — per-service health checks
- **Appendices** — cluster context, SOPS drift reconciliation deferred scope, related documentation links to all 14-01..14-07 SUMMARYs

### `docs/runbooks/db-password-rotation.md` (379 lines)

CNPG fallback runbook per decisions D-13/D-14:

- **Decision rationale** — why CNPG ManagedRoles fails (atomic ALTER ROLE, no rolling restart trigger, criterion table)
- **Operational Hazards** — sops set prohibited, `-o yaml` prohibited, superuser secret excluded, old password must be saved before Step 1
- **Pre-flight** — context, key inspection, CNPG primary identification, allchat\_user existence check, pod health
- **7-step rotation** — generate password, ALTER ROLE (Option A recommended / Option B dual-user), kubectl patch allchat-secrets, 60s kubelet propagation wait, rolling restart of 11 DB-consuming services, connectivity verification, cleanup + SOPS drift register documentation
- **Rollback** — detect via `pq: password authentication failed`, restore via ALTER ROLE to old password + kubectl patch revert + rolling restart
- **Failure modes table** — Step 2-3 race, kubelet cache staleness, connection pool exhaustion, PodDisruptionBudget blocking
- **Cadence** — 90-day / staff-change trigger; first run IS the leaked-password remediation (D-18/D-19)

## Key Constraints Satisfied

| Constraint | Status |
|-----------|--------|
| Every K8s Secret edit uses `kubectl patch` | Both runbooks: every mutation step uses `kubectl patch secret allchat-secrets ... --type=json` |
| `sops set` / `sops edit` explicitly forbidden | Both runbooks: Hazard 1 opens with the SOPS drift warning and prohibition |
| No `kubectl get secret -o yaml` | Neither file contains this pattern; safe `-o jsonpath='{.data}' \| jq 'keys'` shown instead |
| SOPS 31-vs-11 drift hazard cited | Both runbooks reference `project_secrets_drift.md` and the 31-vs-11 count |
| `kubectl --context default` throughout | All kubectl commands use `--context default` per `reference_k8s_context.md` |
| T+24h wait for JWT\_SECRET | Section 2 Step 6 with configmap verification command |
| T+15m wait for SERVICE\_JWT\_SECRET | Section 3 Step 5 |
| TTS overlay JWT excluded | Hazard 4 in secret-rotation.md |
| DB password: 7-step CNPG fallback from RESEARCH.md §2 | db-password-rotation.md Rotation Sequence follows the verbatim procedure |
| Rollback section per rotation type | All four rotation types have explicit rollback with detect/restore/validate |

## Decisions Documented

- **D-13/D-14:** CNPG `ManagedRoles` verdict (FAIL) — atomic `ALTER ROLE` provides no dual-password overlap; running pods must be restarted manually. Documented in "Why a Manual Runbook" section.
- **D-15:** `kubectl patch` is the only safe K8s Secret edit path while SOPS drift persists.
- **D-18:** Phase 14 ships the mechanism; the actual rotation is the operator's first post-Phase-14 run. The "first post-Phase-14 rotation" trigger is the top entry in both runbooks' When-to-Use tables.
- **D-19:** Leaked keys remain valid during Phase 14 development. Mitigation is execution speed, not architecture. Both runbooks highlight that the FIRST rotation IS the leaked-key remediation.

## Deviations from Plan

None — plan executed exactly as written. Both files created, all acceptance criteria satisfied, all must\_haves confirmed.

## Known Stubs

None — both runbooks are complete operational procedures. No placeholder commands or TODO sections.

## Threat Flags

None — these are documentation files only. No new network endpoints, auth paths, or trust-boundary changes. Threat model items T-14-08-01 through T-14-08-07 addressed:

- T-14-08-01: No literal key/password values in either runbook — only `$NEW_KEY`, `$NEW_PASSWORD`, `$OLD_PASSWORD` variable references
- T-14-08-02: Safe `-o jsonpath` pattern shown; `-o yaml` prohibition documented
- T-14-08-03: `sops set` prohibition in Hazard 1 of both runbooks
- T-14-08-04: T+24h wait section documented with configmap verification command
- T-14-08-05/06: Step 4 (60s wait) before Step 5 (rolling restart) mitigates kubelet cache race

## Self-Check: PASSED

- `docs/runbooks/secret-rotation.md` — FOUND (660 lines ≥ 250)
- `docs/runbooks/db-password-rotation.md` — FOUND (379 lines ≥ 100)
- Commit `14394d81` — FOUND (secret-rotation.md)
- Commit `39020358` — FOUND (db-password-rotation.md)
- `grep -q "kubectl patch" docs/runbooks/secret-rotation.md` — PASS
- `grep -q "kubectl create job --from=cronjob/key-rotator-weekly" docs/runbooks/secret-rotation.md` — PASS
- `! grep -q "kubectl get secret.*-o yaml" docs/runbooks/secret-rotation.md` — PASS
- `grep -q "kubectl patch secret allchat-secrets" docs/runbooks/db-password-rotation.md` — PASS
- `! grep -q "kubectl get secret.*-o yaml" docs/runbooks/db-password-rotation.md` — PASS
- `grep -q "ALTER ROLE allchat_user PASSWORD" docs/runbooks/db-password-rotation.md` — PASS
- `grep -q "ManagedRoles" docs/runbooks/db-password-rotation.md` — PASS
- Combined plan verify block — PASS
