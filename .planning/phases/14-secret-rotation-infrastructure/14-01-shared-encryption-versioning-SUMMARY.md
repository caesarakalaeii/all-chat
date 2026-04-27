---
phase: 14
plan: 01
subsystem: shared-encryption
tags: [encryption, key-rotation, aes-gcm, versioning, kid-byte, legacy-compat]
dependency_graph:
  requires: []
  provides: [shared/encryption.MultiKeyEncryptor, shared/encryption.NewMultiKeyEncryptorFromEnv]
  affects: [shared/encryption, services/auth-service]
tech_stack:
  added: []
  patterns: [kid-byte versioned ciphertext, AEAD-fail legacy fallback, env-driven key chain]
key_files:
  created:
    - shared/encryption/versioned.go
    - shared/encryption/versioned_test.go
    - shared/encryption/testdata/golden_v1.bin
    - shared/encryption/testdata/golden_legacy.bin
  modified:
    - services/auth-service/cmd/token-encryption-backfill/main.go
    - services/auth-service/repository/user_repo_test.go
  deleted:
    - shared/crypto/crypto.go
decisions:
  - D-01: kid-byte ciphertext format [v(1B)][nonce(12B)][ct][tag(16B)] implemented in versioned.go
  - D-02: MultiKeyEncryptor reads TOKEN_ENCRYPTION_KEY_V{n} env chain; last present = write key
  - D-04: YOUTUBE_TOKEN_ENCRYPTION_KEY added as second legacy fallback in unified chain
  - D-05: AEAD-fail on versioned path falls back to legacy keys (false-positive kid recovery)
  - D-20: Phase anchor — Wave 1, all other plans depend on MultiKeyEncryptor existing
metrics:
  duration: 75m
  completed: "2026-04-27"
  tasks: 2
  files: 7
---

# Phase 14 Plan 01: Shared Encryption Versioning Summary

Adds `MultiKeyEncryptor` to `shared/encryption` — the versioned AES-GCM primitive that prepends a 1-byte kid to new ciphertexts and falls back gracefully to legacy kid-less blobs. Deletes the duplicate `shared/crypto` package by migrating its two callers to `shared/encryption`.

## What Changed

### Task 1: versioned.go + test suite + golden fixtures

`shared/encryption/versioned.go` introduces:

- `MultiKeyEncryptor` — encrypts with the latest key, decrypts with any registered key plus an optional legacy fallback chain.
- `NewMultiKeyEncryptorFromEnv()` — reads `TOKEN_ENCRYPTION_KEY_V1.._V{n}`, `TOKEN_ENCRYPTION_KEY`, `YOUTUBE_TOKEN_ENCRYPTION_KEY` from env.
- `NewMultiKeyEncryptor(entries, legacyKeys)` — explicit constructor for tests and the sweeper.
- `CurrentKid()` — returns the kid byte used for new writes.
- `EncryptString` / `DecryptString` / `Encrypt` / `Decrypt` — wire-format-aware encrypt/decrypt with legacy fallback.

Wire format (D-01):

```
Versioned: base64( [kid(1B)] [nonce(12B)] [ciphertext] [tag(16B)] )
Legacy:    base64( [nonce(12B)] [ciphertext] [tag(16B)] )
```

Kid namespace (T-14-01-02):
- `0x00` = `LegacyKid` — reserved, never written
- `0x01..0x7F` = valid range (allocated sequentially)
- `0x80..0xFF` = reserved for future use

`versioned_test.go` covers all 11 behaviors from PLAN.md:
- Round-trip V1, decrypt-old-kid after V2 added, legacy backcompat
- False-positive kid recovery (brute-force finds real random blob with `decoded[0]==0x01`, proves AEAD-fail → legacy-key fallback works)
- Unified chain (TOKEN_ENCRYPTION_KEY + YOUTUBE_TOKEN_ENCRYPTION_KEY both decrypt)
- Golden V1 and golden legacy fixtures
- Env constructor with V1 only, requires-at-least-one-key error, YouTube legacy path
- Kid range rejection (0x00, >0x7F)

### Task 2: Migrate callers and delete shared/crypto

`shared/crypto/crypto.go` was a functional duplicate of `shared/encryption` (identical AES-GCM wire format `base64(nonce||ct||tag)`). RESEARCH.md documented one caller; grep found two:

1. `services/auth-service/cmd/token-encryption-backfill/main.go` — replaced `crypto.NewAESGCMCipher` with `encryption.ParseKey + NewAESEncryptor`
2. `services/auth-service/repository/user_repo_test.go` — replaced `crypto.NewAESGCMCipher` with `encryption.ParseKey + NewAESEncryptor` (auto-fixed per Rule 1 — RESEARCH.md missed this caller)

`shared/crypto/crypto.go` deleted. All 14 service modules build cleanly after deletion.

## Final API Surface of MultiKeyEncryptor

```go
type KidByte = byte

const LegacyKid KidByte = 0x00
const MaxKid    KidByte = 0x7F

type KeyEntry struct {
    Kid    KidByte
    Cipher *AESEncryptor
}

type MultiKeyEncryptor struct { /* immutable after construction */ }

func NewMultiKeyEncryptor(entries []KeyEntry, legacyKeys []*AESEncryptor) (*MultiKeyEncryptor, error)
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error)

func (m *MultiKeyEncryptor) CurrentKid() KidByte
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error)
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error)
func (m *MultiKeyEncryptor) Encrypt(s string) (string, error)  // StringCipher alias
func (m *MultiKeyEncryptor) Decrypt(s string) (string, error)  // StringCipher alias

var ErrNoEncryptionKeys error
var ErrReservedKid      error
var ErrUnknownKid       error
```

## Golden Ciphertext Bytes (forensic reference)

Test vector: plaintext `"test-token-value"`, key 32×`0x00`, nonce 12×`0x00`.

```
golden_legacy.bin (base64): AAAAAAAAAAAAAAAAusIzSWAUBAViIOil25/ofRaBw2WQ9CGbMVHMidGAQmE=
golden_legacy.bin (hex):    000000000000000000000000bac23349601404056220e8a5db9fe87d1681c36590f4219b3151cc89d1804261
  breakdown: [nonce: 000000000000000000000000] [ct: bac2334960140405] [tag: 6220e8a5db9fe87d1681c36590f4219b3151cc89d1804261]

golden_v1.bin (base64): AQAAAAAAAAAAAAAAALrCM0lgFAQFYiDopduf6H0WgcNlkPQhmzFRzInRgEJh
golden_v1.bin (hex):    01000000000000000000000000bac23349601404056220e8a5db9fe87d1681c36590f4219b3151cc89d1804261
  breakdown: [kid: 01] [nonce: 000000000000000000000000] [ct: bac2334960140405] [tag: 6220e8a5db9fe87d1681c36590f4219b3151cc89d1804261]
```

To regenerate golden files (e.g. after a Go stdlib AES-GCM change — which would break the fixtures intentionally):
```bash
cd shared && go test ./encryption/... -update
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Second shared/crypto caller not documented in RESEARCH.md**
- **Found during:** Task 2 — initial `grep -rn '"github.com/caesar/all-chat/shared/crypto"'` before deletion
- **Issue:** RESEARCH.md §3 "shared/crypto import sites" documented only `token-encryption-backfill/main.go`. `services/auth-service/repository/user_repo_test.go` was also importing `shared/crypto` to construct a test cipher for `newTestUserRepository`.
- **Fix:** Migrated `user_repo_test.go` to use `encryption.ParseKey + NewAESEncryptor` before deleting `shared/crypto`. `*encryption.AESEncryptor` satisfies the local `StringCipher` interface defined in `user_repository.go`.
- **Files modified:** `services/auth-service/repository/user_repo_test.go`
- **Commit:** bundled in `a096b457`

**2. [Rule 1 - Bug] TestNewMultiKeyEncryptorFromEnv used raw zero-byte key value**
- **Found during:** Task 1 test run
- **Issue:** `t.Setenv("TOKEN_ENCRYPTION_KEY", string(zeroKey32()))` failed with `setenv: invalid argument` because the value contained null bytes (OS env vars cannot contain null).
- **Fix:** Changed to `base64.StdEncoding.EncodeToString(zeroKey32())` — `ParseKey` accepts base64-encoded keys.
- **Files modified:** `shared/encryption/versioned_test.go`

## Known Stubs

None. The `MultiKeyEncryptor` API is fully implemented and all test behaviors wire real data through the real encrypt/decrypt path.

## Threat Flags

No new threat surface introduced. All changes are internal to the `shared/encryption` package and the existing `auth-service` binary. No new network endpoints, auth paths, or schema changes.

## Self-Check: PASSED

- `shared/encryption/versioned.go`: EXISTS
- `shared/encryption/versioned_test.go`: EXISTS
- `shared/encryption/testdata/golden_v1.bin`: EXISTS
- `shared/encryption/testdata/golden_legacy.bin`: EXISTS
- `shared/crypto/crypto.go`: DELETED
- Task 1 commit `cf0a0996`: EXISTS in git log
- Task 2 changes committed in `a096b457` (bundled with api-gateway fix): verified via `git show a096b457 --stat`
- `go test ./shared/encryption/... -count=1 -race`: PASS (13/13 tests)
- All 14 service modules `go build ./...`: PASS
- Zero `shared/crypto` references in `*.go` files: CONFIRMED
