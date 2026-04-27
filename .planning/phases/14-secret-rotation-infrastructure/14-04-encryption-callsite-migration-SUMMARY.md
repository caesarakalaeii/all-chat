---
phase: 14
plan: 04
subsystem: encryption-callsites
tags: [encryption, key-rotation, multi-key, aes-gcm, kid-byte, migration]
dependency_graph:
  requires: [shared/encryption.MultiKeyEncryptor (plan 14-01)]
  provides:
    - services/auth-service uses *MultiKeyEncryptor
    - services/overlay-manager uses *MultiKeyEncryptor
    - services/token-refresh-service uses *MultiKeyEncryptor
    - services/twitch-eventsub-listener uses *MultiKeyEncryptor
    - services/youtube-listener uses *MultiKeyEncryptor
  affects:
    - services/auth-service
    - services/overlay-manager
    - services/token-refresh-service
    - services/twitch-eventsub-listener
    - services/youtube-listener
tech_stack:
  added: []
  patterns:
    - MultiKeyEncryptor drop-in replacing AESEncryptor at all call sites
    - env-driven key chain (NewMultiKeyEncryptorFromEnv)
    - unified TOKEN_ENCRYPTION_KEY chain absorbs YOUTUBE_TOKEN_ENCRYPTION_KEY (D-04)
key_files:
  created: []
  modified:
    - services/auth-service/cmd/main.go
    - services/overlay-manager/cmd/main.go
    - services/overlay-manager/models/tts_config.go
    - services/overlay-manager/handlers/tts.go
    - services/token-refresh-service/cmd/main.go
    - services/token-refresh-service/repository/token_repository.go
    - services/twitch-eventsub-listener/cmd/main.go
    - services/twitch-eventsub-listener/channels/manager.go
    - services/youtube-listener/cmd/main.go
    - services/youtube-listener/cmd/token_backfill/main.go
    - services/youtube-listener/oauth/store.go
decisions:
  - D-02: TOKEN_ENCRYPTION_KEY_V1 env chain consumed via NewMultiKeyEncryptorFromEnv at all 5 services
  - D-04: YOUTUBE_TOKEN_ENCRYPTION_KEY is now a legacy fallback inside NewMultiKeyEncryptorFromEnv; youtube-listener needs no special-case code
  - D-05: Legacy kid-less ciphertext transparently decrypted via fallback chain; no upfront DB migration needed
metrics:
  duration: ~25m
  completed: "2026-04-27"
  tasks: 2
  files: 11
---

# Phase 14 Plan 04: Encryption Call-Site Migration Summary

Migrates every Go service that previously held a `*encryption.AESEncryptor` to hold a `*encryption.MultiKeyEncryptor`. The wire-format change is transparent at each call site — public method names (`EncryptString`, `DecryptString`, `Encrypt`, `Decrypt`) are identical. New writes now produce versioned ciphertext with a `kid` prefix; reads transparently handle both versioned and legacy (kid-less) format.

## Services Migrated

| Service | Constructor Before | Constructor After | Field/Parameter Types Changed |
|---------|-------------------|-------------------|-------------------------------|
| auth-service | `ParseKey` + `NewAESEncryptor` | `NewMultiKeyEncryptorFromEnv` | `tokenCipher` is now `*MultiKeyEncryptor` |
| overlay-manager | `ParseKey` + `NewAESEncryptor` | `NewMultiKeyEncryptorFromEnv` | `tokenCipher` is now `*MultiKeyEncryptor` |
| token-refresh-service | `ParseKey` + `NewAESEncryptor` reading `ENCRYPTION_KEY` | `NewMultiKeyEncryptorFromEnv` | `cipher` field in `TokenRepository` → `*MultiKeyEncryptor` |
| twitch-eventsub-listener | `ParseKey` + `NewAESEncryptor` reading `ENCRYPTION_KEY` | `NewMultiKeyEncryptorFromEnv` | `cipher` field in `channels.Manager` → `*MultiKeyEncryptor` |
| youtube-listener | `ParseKey` + `NewAESEncryptor` reading `YOUTUBE_TOKEN_ENCRYPTION_KEY` | `NewMultiKeyEncryptorFromEnv` | `enc` field in `oauth.PostgresTokenStore` → `*MultiKeyEncryptor` |

## StringEncryptor Interface — Unchanged (viewer_auth.go)

`services/auth-service/handlers/viewer_auth.go` defines a `StringEncryptor` interface with `Encrypt(string) (string, error)` and `Decrypt(string) (string, error)`. `*MultiKeyEncryptor` satisfies this interface via the `Encrypt`/`Decrypt` aliases added in Plan 14-01. No changes were required to `viewer_auth.go` and it was NOT modified by this plan.

## TTS jwt.go — Untouched (D-11)

`services/overlay-manager/tts/jwt.go` uses the per-overlay `tts_signing_secret` (Phase 13 design), not the TOKEN_ENCRYPTION_KEY chain. It was NOT modified, per D-11 and the plan constraint.

## D-04 Unification (YOUTUBE_TOKEN_ENCRYPTION_KEY)

`NewMultiKeyEncryptorFromEnv` (Plan 14-01) already reads `YOUTUBE_TOKEN_ENCRYPTION_KEY` as a second legacy fallback after `TOKEN_ENCRYPTION_KEY`. The youtube-listener migration required no special-case YouTube code — the unified chain handles existing YouTube ciphertexts transparently. The log message at startup explicitly names this fallback.

## Pitfall 1 — Hand-off to Plan 14-07

**token-refresh-service** and **twitch-eventsub-listener** previously read env var `ENCRYPTION_KEY` for their cipher. Their code now calls `NewMultiKeyEncryptorFromEnv()` which reads `TOKEN_ENCRYPTION_KEY_V1` (required) and `TOKEN_ENCRYPTION_KEY` (legacy fallback).

**The deployment manifests for these two services still mount the K8s secret `token-encryption-key` under the key `ENCRYPTION_KEY` — this name mismatch means the services WILL fail to start in production until Plan 14-07 ships.**

Plan 14-07 must apply the following manifest renames for both services:

| Service | Current env var name | New env var name required |
|---------|---------------------|--------------------------|
| token-refresh-service | `ENCRYPTION_KEY` → `token-encryption-key` | `TOKEN_ENCRYPTION_KEY` |
| twitch-eventsub-listener | `ENCRYPTION_KEY` → `token-encryption-key` | `TOKEN_ENCRYPTION_KEY` |

Additionally, Plan 14-07 must add `TOKEN_ENCRYPTION_KEY_V1` (mapped to the new versioned K8s secret key) for ALL five services. Until then, these services cannot start with just the plan-14-04 code deployed.

**Production deploy gate:** Do not deploy any of the five services built after Plan 14-04 merges until Plan 14-07 also merges (Wave 3). This is the intended sequencing per T-14-04-01.

## Note for Plan 14-05

`services/auth-service/cmd/main.go` and `services/overlay-manager/cmd/main.go` were modified in this plan but only in the encryption construction block (lines ~129-133 in auth-service, ~138-145 in overlay-manager). Plan 14-05 will edit different blocks in these files (JWT key chain wiring). The files are left in a clean state for that edit.

## Note for Plan 14-06

`services/youtube-listener/cmd/token_backfill/main.go` was updated to use `NewMultiKeyEncryptorFromEnv()` for consistency. The new sweeper from Plan 14-06 supersedes this binary as the primary rotation tool. The old binary remains compiled but is not part of the rotation runbook.

## Pre-existing Test Failures (not caused by this plan)

auth-service `repository` tests fail with `column "is_premium" does not exist` — the testcontainer schema in `user_repo_test.go` is hardcoded at an older migration level that predates the `is_premium` column. This failure predates Plan 14-04. auth-service `handlers` tests also have pre-existing Discord and auth handler failures unrelated to encryption.

All token-refresh-service, twitch-eventsub-listener, overlay-manager, and youtube-listener tests are green.

## Deviations from Plan

None — plan executed exactly as written. The `overlay-manager/handlers/tts.go` comment update (documenting `MultiKeyEncryptor` as the production implementation of `aesCipher`) is a cosmetic improvement within plan scope; the interface itself is unchanged.

## Known Stubs

None. All encryption call sites are fully wired to `*MultiKeyEncryptor`. No placeholder values or mock data flows to production paths.

## Threat Flags

No new threat surface introduced. All changes are internal constructor swaps and field type updates. No new network endpoints, auth paths, file access patterns, or schema changes.

## Self-Check: PASSED

- `services/auth-service/cmd/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/overlay-manager/cmd/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/token-refresh-service/cmd/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/token-refresh-service/repository/token_repository.go`: contains `cipher *encryption.MultiKeyEncryptor` — CONFIRMED
- `services/twitch-eventsub-listener/cmd/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/twitch-eventsub-listener/channels/manager.go`: contains `cipher *encryption.MultiKeyEncryptor` — CONFIRMED
- `services/youtube-listener/cmd/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/youtube-listener/cmd/token_backfill/main.go`: contains `NewMultiKeyEncryptorFromEnv` — CONFIRMED
- `services/youtube-listener/oauth/store.go`: contains `enc *encryption.MultiKeyEncryptor` — CONFIRMED
- `services/auth-service/handlers/viewer_auth.go`: NOT modified — CONFIRMED
- `services/overlay-manager/tts/jwt.go`: NOT modified — CONFIRMED
- Task 1 commit `b7153f63`: EXISTS in git log — CONFIRMED
- Task 2 commit `7634b777`: EXISTS in git log — CONFIRMED
- `go build` all 5 services: PASS
- `go test` overlay-manager, youtube-listener, token-refresh-service, twitch-eventsub-listener: ALL GREEN
- `go test` auth-service: pre-existing failures in repository (is_premium schema) and handlers (Discord/auth); not caused by encryption changes
