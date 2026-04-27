---
phase: 14-secret-rotation-infrastructure
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - shared/encryption/versioned.go
  - shared/encryption/versioned_test.go
  - shared/encryption/testdata/golden_v1.bin
  - shared/encryption/testdata/golden_legacy.bin
  - shared/crypto/crypto.go
  - services/auth-service/cmd/token-encryption-backfill/main.go
autonomous: true
decisions_addressed:
  - D-01
  - D-02
  - D-04
  - D-05
  - D-20
must_haves:
  truths:
    - "shared/encryption exposes MultiKeyEncryptor that prepends a 1-byte kid to new ciphertext"
    - "Decrypt auto-detects versioned vs legacy format and falls back to legacy AESEncryptor on AEAD failure"
    - "Unified chain reads TOKEN_ENCRYPTION_KEY (legacy) AND YOUTUBE_TOKEN_ENCRYPTION_KEY (legacy) for decryption"
    - "shared/crypto package is removed; its sole caller (token-encryption-backfill) uses shared/encryption directly"
    - "Golden ciphertexts (legacy + V1) commit as testdata so format regressions are caught"
  artifacts:
    - path: "shared/encryption/versioned.go"
      provides: "MultiKeyEncryptor, KidByte, KeyEntry, NewMultiKeyEncryptorFromEnv, NewMultiKeyEncryptor, EncryptString, DecryptString, CurrentKid"
      min_lines: 150
    - path: "shared/encryption/versioned_test.go"
      provides: "TestMultiKey*, TestLegacyBackcompat, TestFalsePositive, TestUnifiedChain, TestGolden"
      min_lines: 200
    - path: "shared/encryption/testdata/golden_v1.bin"
      provides: "Fixed test vector — base64 of [0x01||nonce||ct||tag] for known plaintext+key+nonce"
    - path: "shared/encryption/testdata/golden_legacy.bin"
      provides: "Fixed test vector — base64 of [nonce||ct||tag] (kid-less) for same plaintext+key+nonce"
  key_links:
    - from: "shared/encryption/versioned.go"
      to: "shared/encryption/encryption.go"
      via: "wraps existing AESEncryptor as the per-key primitive (calls EncryptString/DecryptString internally with kid byte stripped/added)"
      pattern: "AESEncryptor"
    - from: "services/auth-service/cmd/token-encryption-backfill/main.go"
      to: "shared/encryption"
      via: "imports encryption package (replaces shared/crypto)"
      pattern: "encryption.MultiKeyEncryptor"
---

<objective>
Build the versioned multi-key AES-GCM primitive in `shared/encryption` and remove the duplicate `shared/crypto` package by migrating its only caller. This is the cryptographic foundation for the entire phase — every other plan depends on `MultiKeyEncryptor` existing.

Purpose: Implements decisions D-01 (kid-byte ciphertext format), D-02 (multi-key env-driven chain), D-04 (unify TOKEN_ENCRYPTION_KEY + YOUTUBE_TOKEN_ENCRYPTION_KEY), and D-05 (legacy backwards-compat with AEAD-fail fallback). D-20 honored: All four workstreams (encryption versioning in 14-01/04/06, JWT rotation in 14-02/05, gap-fill in 14-03/05, DB password in 14-08) ship in this single phase across 8 plans / 3 waves; this plan is the phase anchor (Wave 1, primary shared library).

Output:
- `shared/encryption/versioned.go` — `MultiKeyEncryptor` type, env loader, encrypt/decrypt with kid dispatch.
- `shared/encryption/versioned_test.go` — golden ciphertexts, false-positive kid recovery, unified chain decoding.
- `shared/crypto/crypto.go` deleted; `services/auth-service/cmd/token-encryption-backfill/main.go` uses `shared/encryption` instead.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md
@.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md
@.planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md
@.planning/phases/14-secret-rotation-infrastructure/14-VALIDATION.md
@shared/encryption/encryption.go
@shared/crypto/crypto.go
@services/auth-service/cmd/token-encryption-backfill/main.go

<interfaces>
<!-- The existing AESEncryptor primitive that MultiKeyEncryptor wraps. -->

From shared/encryption/encryption.go:
```go
type AESEncryptor struct { gcm cipher.AEAD; nonceSize int }
func NewAESEncryptor(key []byte) (*AESEncryptor, error)  // expects 16/24/32-byte key
func ParseKey(key string) ([]byte, error)                 // base64-or-raw input
func (e *AESEncryptor) EncryptString(plaintext string) (string, error)   // returns base64(nonce||ct||tag)
func (e *AESEncryptor) DecryptString(ciphertext string) (string, error)  // expects base64(nonce||ct||tag)
func (e *AESEncryptor) Encrypt(plaintext string) (string, error)         // alias of EncryptString
func (e *AESEncryptor) Decrypt(ciphertext string) (string, error)        // alias of DecryptString
var ErrEmptyKey, ErrInvalidKeyBytes error
```

From shared/crypto/crypto.go (DOOMED — this plan deletes it):
```go
type StringCipher interface { Encrypt(string) (string, error); Decrypt(string) (string, error) }
func NewAESGCMCipher(key string) (StringCipher, error)
```
The `*encryption.AESEncryptor` and `*encryption.MultiKeyEncryptor` already satisfy this interface contract via the `Encrypt`/`Decrypt` aliases.

From services/auth-service/cmd/token-encryption-backfill/main.go (caller to migrate):
```go
import "github.com/caesar/all-chat/shared/crypto"      // ← REMOVE
type backfillRunner struct {
    pool   *pgxpool.Pool
    cipher crypto.StringCipher                          // ← change to *encryption.MultiKeyEncryptor
    dryRun bool
}
// In main(): cipher, err := crypto.NewAESGCMCipher(os.Getenv("TOKEN_ENCRYPTION_KEY"))
//        →   cipher, err := encryption.NewMultiKeyEncryptorFromEnv()
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create MultiKeyEncryptor primitive (versioned.go) and golden test scaffolding</name>
  <files>shared/encryption/versioned.go, shared/encryption/versioned_test.go, shared/encryption/testdata/golden_v1.bin, shared/encryption/testdata/golden_legacy.bin</files>
  <read_first>
    - shared/encryption/encryption.go (the AESEncryptor primitive being wrapped — full file)
    - shared/crypto/crypto.go (the duplicate this plan obviates — full file)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §1 "Proposed Go API Surface for Versioned Encryption" (lines 107–161) and §5 "Disambiguation Logic" (lines 449–465)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "shared/encryption/versioned.go (NEW...)" section (lines 38–137) for the exact API skeleton
    - .planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md decisions D-01, D-02, D-04, D-05
    - .planning/phases/14-secret-rotation-infrastructure/14-VALIDATION.md table rows for D-01/D-02, D-04, D-05 (lines 44–48)
  </read_first>
  <behavior>
    - Test 1 (TestMultiKeyEncryptor_RoundTripV1): construct chain with one V1 key; Encrypt then Decrypt round-trip yields original plaintext; first decoded byte == 0x01.
    - Test 2 (TestMultiKeyEncryptor_DecryptOldKid): chain with V1 + V2 keys; ciphertext written under V1 still decrypts after V2 added; CurrentKid()==0x02; new writes prefix 0x02.
    - Test 3 (TestMultiKeyEncryptor_LegacyBackcompat): chain with V1 + legacy AESEncryptor; an `AESEncryptor.EncryptString(...)` blob from the legacy key (no kid prefix) decrypts via fallback path.
    - Test 4 (TestMultiKeyEncryptor_FalsePositiveKid): hand-craft a legacy blob whose decoded[0]==0x01 (use a deterministic seeded nonce in test); MultiKeyEncryptor.DecryptString tries versioned path, AEAD authentication fails, code retries with legacy key → succeeds with the original plaintext.
    - Test 5 (TestMultiKeyEncryptor_UnifiedChain): construct chain with V1 + two legacy keys (legacyToken from TOKEN_ENCRYPTION_KEY, legacyYoutube from YOUTUBE_TOKEN_ENCRYPTION_KEY); legacy ciphertexts produced by either legacy key decrypt successfully.
    - Test 6 (TestMultiKeyEncryptor_GoldenV1): read `testdata/golden_v1.bin` (committed fixture: base64 of `[0x01||12 zero-byte nonce||ct||tag]` for plaintext "test-token-value" with key 32-byte all-zero); decrypt with the matching V1 key → "test-token-value".
    - Test 7 (TestMultiKeyEncryptor_GoldenLegacy): read `testdata/golden_legacy.bin` (committed fixture: base64 of `[12 zero-byte nonce||ct||tag]` for same plaintext + key); decrypt via legacy fallback → "test-token-value".
    - Test 8 (TestNewMultiKeyEncryptorFromEnv): set env `TOKEN_ENCRYPTION_KEY=<32-byte-base64>`, `TOKEN_ENCRYPTION_KEY_V1=<different-32-byte-base64>`; call constructor; verify CurrentKid()==0x01, verify Encrypt/Decrypt of fresh plaintext round-trips, verify a legacy AESEncryptor blob (built from TOKEN_ENCRYPTION_KEY value) decrypts via fallback.
    - Test 9 (TestNewMultiKeyEncryptorFromEnv_RequiresAtLeastOneKey): unset all keys; constructor returns error matching "no encryption keys configured".
    - Test 10 (TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy): set `TOKEN_ENCRYPTION_KEY_V1=<key>`, `YOUTUBE_TOKEN_ENCRYPTION_KEY=<other-key>` (no `TOKEN_ENCRYPTION_KEY`); constructor builds chain that can decrypt blobs from the YouTube legacy key (D-04 unification).
    - Test 11 (TestKidRangeReserved): constructor rejects attempts to register kid 0x00 (reserved as LegacyKid) or kid > 0x7F (reserved range per CONTEXT.md specifics).
  </behavior>
  <action>
    Step 1 — Generate the golden test vectors as committed fixtures.

    Write a small Go file under `shared/encryption/testdata/gen/main.go` (or use `go generate` directive) that produces the golden ciphertexts. **However, since testdata is committed and stable, simpler: hand-compute and commit the bytes directly.** Use this approach instead:

    Create `shared/encryption/testdata/golden_v1.bin` and `shared/encryption/testdata/golden_legacy.bin` programmatically inside the test (write a `TestMain` that, on first run, regenerates the goldens if missing — guarded by a `-update` flag) OR hand-compute once. Recommended pattern (matches Go convention):

    Add a `versioned_golden_gen_test.go` that runs only when `-update` flag is passed:
    ```go
    var update = flag.Bool("update", false, "regenerate golden testdata files")
    func TestMain(m *testing.M) { flag.Parse(); if *update { regenerateGoldens() }; os.Exit(m.Run()) }
    ```
    The committed `golden_v1.bin` contains the base64 string (one line) representing `[0x01||12 zero-bytes||AES-GCM seal of "test-token-value" with 32-byte zero key]`. Similarly `golden_legacy.bin` contains base64 of `[12 zero-bytes||same ciphertext||tag]` (no kid).

    Step 2 — Create `shared/encryption/versioned.go` with the API surface from PATTERNS.md lines 91–127:

    ```go
    // Copyright (C) 2026 caesarakalaeii  (copy AGPL header from encryption.go)
    package encryption

    import (
        "encoding/base64"
        "errors"
        "fmt"
        "os"
        "strconv"
    )

    // KidByte is a 1-byte key identifier prepended to versioned ciphertext.
    // 0x00 is reserved as "legacy / kid-less" and never written by EncryptString.
    // 0x01..0x7F are reserved for forward use; planners must allocate sequentially.
    type KidByte = byte

    const (
        LegacyKid KidByte = 0x00
        MaxKid    KidByte = 0x7F
    )

    var (
        ErrNoEncryptionKeys   = errors.New("no encryption keys configured: set TOKEN_ENCRYPTION_KEY_V1 (and optional TOKEN_ENCRYPTION_KEY for legacy fallback)")
        ErrReservedKid        = errors.New("kid 0x00 reserved as legacy; valid range 0x01..0x7F")
        ErrUnknownKid         = errors.New("ciphertext kid byte not registered")
    )

    // KeyEntry maps a KidByte to its AES-GCM cipher.
    type KeyEntry struct {
        Kid    KidByte
        Cipher *AESEncryptor
    }

    // MultiKeyEncryptor encrypts with the latest registered key and decrypts with any
    // registered key plus an optional legacy (kid-less) fallback chain.
    // Thread-safe; the registered keys map is immutable after construction.
    type MultiKeyEncryptor struct {
        latest        *KeyEntry
        byKid         map[KidByte]*AESEncryptor
        legacyKeys    []*AESEncryptor // ordered: TOKEN_ENCRYPTION_KEY first, then YOUTUBE_TOKEN_ENCRYPTION_KEY (D-04)
    }

    // NewMultiKeyEncryptor constructs from explicit entries (used by tests and the sweeper).
    // entries must be non-empty; entries[len-1] is treated as the latest (write) key.
    // legacyKeys may be empty (e.g., kick/tiktok new columns where no legacy data exists).
    func NewMultiKeyEncryptor(entries []KeyEntry, legacyKeys []*AESEncryptor) (*MultiKeyEncryptor, error) {
        if len(entries) == 0 {
            return nil, ErrNoEncryptionKeys
        }
        byKid := make(map[KidByte]*AESEncryptor, len(entries))
        for i := range entries {
            kid := entries[i].Kid
            if kid == LegacyKid || kid > MaxKid {
                return nil, fmt.Errorf("%w: got 0x%02x", ErrReservedKid, kid)
            }
            if _, dup := byKid[kid]; dup {
                return nil, fmt.Errorf("duplicate kid 0x%02x", kid)
            }
            byKid[kid] = entries[i].Cipher
        }
        latest := &entries[len(entries)-1]
        return &MultiKeyEncryptor{latest: latest, byKid: byKid, legacyKeys: legacyKeys}, nil
    }

    // NewMultiKeyEncryptorFromEnv reads:
    //   TOKEN_ENCRYPTION_KEY_V1, _V2, ...     → versioned chain (kid 0x01, 0x02, ...)
    //   TOKEN_ENCRYPTION_KEY                   → legacy fallback (kid-less)
    //   YOUTUBE_TOKEN_ENCRYPTION_KEY           → second legacy fallback (per D-04)
    // Returns ErrNoEncryptionKeys if no V<n> is set.
    func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error) {
        var entries []KeyEntry
        for n := 1; n <= int(MaxKid); n++ {
            envName := "TOKEN_ENCRYPTION_KEY_V" + strconv.Itoa(n)
            v := os.Getenv(envName)
            if v == "" {
                if n == 1 {
                    // V1 is mandatory
                    break
                }
                break
            }
            parsed, err := ParseKey(v)
            if err != nil { return nil, fmt.Errorf("parse %s: %w", envName, err) }
            cipher, err := NewAESEncryptor(parsed)
            if err != nil { return nil, fmt.Errorf("create cipher %s: %w", envName, err) }
            entries = append(entries, KeyEntry{Kid: KidByte(n), Cipher: cipher})
        }
        if len(entries) == 0 {
            return nil, ErrNoEncryptionKeys
        }
        var legacyKeys []*AESEncryptor
        if v := os.Getenv("TOKEN_ENCRYPTION_KEY"); v != "" {
            parsed, err := ParseKey(v)
            if err != nil { return nil, fmt.Errorf("parse TOKEN_ENCRYPTION_KEY: %w", err) }
            cipher, err := NewAESEncryptor(parsed)
            if err != nil { return nil, fmt.Errorf("create cipher TOKEN_ENCRYPTION_KEY: %w", err) }
            legacyKeys = append(legacyKeys, cipher)
        }
        if v := os.Getenv("YOUTUBE_TOKEN_ENCRYPTION_KEY"); v != "" {
            parsed, err := ParseKey(v)
            if err != nil { return nil, fmt.Errorf("parse YOUTUBE_TOKEN_ENCRYPTION_KEY: %w", err) }
            cipher, err := NewAESEncryptor(parsed)
            if err != nil { return nil, fmt.Errorf("create cipher YOUTUBE_TOKEN_ENCRYPTION_KEY: %w", err) }
            legacyKeys = append(legacyKeys, cipher)
        }
        return NewMultiKeyEncryptor(entries, legacyKeys)
    }

    // CurrentKid returns the KidByte used for new writes.
    func (m *MultiKeyEncryptor) CurrentKid() KidByte { return m.latest.Kid }

    // EncryptString encrypts plaintext using the latest key.
    // Wire format: base64( [kid(1B)] [nonce(12B)] [ct] [tag(16B)] ).
    func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error) {
        // Use the underlying AESEncryptor to produce the legacy blob (nonce||ct||tag),
        // base64-decode it, prepend the kid byte, base64-encode the result.
        legacyBlob, err := m.latest.Cipher.EncryptString(plaintext)
        if err != nil { return "", err }
        rawLegacy, err := base64.StdEncoding.DecodeString(legacyBlob)
        if err != nil { return "", fmt.Errorf("re-decode for kid prefix: %w", err) }
        out := make([]byte, 0, len(rawLegacy)+1)
        out = append(out, m.latest.Kid)
        out = append(out, rawLegacy...)
        return base64.StdEncoding.EncodeToString(out), nil
    }

    // DecryptString auto-detects format. Versioned path is tried first when blob[0]
    // matches a registered kid AND len >= 1+12+16. On AEAD authentication failure
    // (false-positive kid byte on a legacy blob), retries with each legacy key.
    func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error) {
        decoded, err := base64.StdEncoding.DecodeString(ciphertext)
        if err != nil { return "", fmt.Errorf("decode: %w", err) }
        // Try versioned path first.
        if len(decoded) >= 1+12+16 {
            kid := decoded[0]
            if cipher, ok := m.byKid[kid]; ok {
                // Reconstitute legacy-shaped blob (nonce||ct||tag) for the underlying AESEncryptor.
                legacyShaped := base64.StdEncoding.EncodeToString(decoded[1:])
                if pt, err := cipher.DecryptString(legacyShaped); err == nil {
                    return pt, nil
                }
                // Fall through to legacy fallback (false-positive kid case).
            }
        }
        // Legacy fallback: try each legacy key in order.
        for _, lk := range m.legacyKeys {
            if pt, err := lk.DecryptString(ciphertext); err == nil {
                return pt, nil
            }
        }
        return "", fmt.Errorf("decrypt: no key in chain (versioned + %d legacy) authenticated the ciphertext", len(m.legacyKeys))
    }

    // Encrypt is a StringCipher-compatible alias for EncryptString.
    func (m *MultiKeyEncryptor) Encrypt(s string) (string, error) { return m.EncryptString(s) }
    // Decrypt is a StringCipher-compatible alias for DecryptString.
    func (m *MultiKeyEncryptor) Decrypt(s string) (string, error) { return m.DecryptString(s) }
    ```

    Step 3 — Write `shared/encryption/versioned_test.go` covering all 11 behaviors above. Use `t.Setenv` for env-driven tests so the tests are isolated. For TestMultiKeyEncryptor_FalsePositiveKid, do NOT mock anything; instead, generate a legitimate AESEncryptor blob in a loop until `decoded[0] == 0x01` (probability 1/256 per attempt, will hit within ~1000 iterations) — this proves real-world false-positive recovery works. Use `require.Eventually` style or loop bounded at 5000 attempts.

    For TestMultiKeyEncryptor_GoldenV1 and TestMultiKeyEncryptor_GoldenLegacy: read the testdata files via `os.ReadFile`. Strip trailing newline.

    Step 4 — Generate the golden testdata files. Add a `TestMain` with a `-update` flag in the same test file. When run as `go test -update`, regenerate the goldens. Commit the generated files.

    Compute the deterministic ciphertexts in `regenerateGoldens()`:
    - Plaintext: "test-token-value"
    - Key: 32 bytes of 0x00
    - Nonce: 12 bytes of 0x00 (NOT random — for test determinism only)
    - Use `cipher.NewGCM(aes.NewCipher(zeroKey))`, `gcm.Seal(zeroNonce, zeroNonce, []byte(plaintext), nil)` → produces `nonce||ct||tag`.
    - For legacy: `base64.StdEncoding.EncodeToString(seal_output)` → write to `golden_legacy.bin`
    - For V1: `prepend(0x01, seal_output)` then base64 → write to `golden_v1.bin`
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go test ./shared/encryption/... -run 'TestMultiKey|TestNewMultiKey|TestKidRange|TestGolden|TestUnifiedChain|TestLegacyBackcompat|TestFalsePositive' -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "type MultiKeyEncryptor struct" shared/encryption/versioned.go`
    - `grep -q "func NewMultiKeyEncryptorFromEnv" shared/encryption/versioned.go`
    - `grep -q "TOKEN_ENCRYPTION_KEY_V" shared/encryption/versioned.go`
    - `grep -q "YOUTUBE_TOKEN_ENCRYPTION_KEY" shared/encryption/versioned.go` (D-04 unified chain)
    - `grep -q "func (m \*MultiKeyEncryptor) CurrentKid" shared/encryption/versioned.go`
    - `grep -q "false-positive\|legacyKeys\|fallback" shared/encryption/versioned.go` (false-positive recovery code path present)
    - `test -f shared/encryption/testdata/golden_v1.bin && test -f shared/encryption/testdata/golden_legacy.bin`
    - `grep -q "TestMultiKeyEncryptor_FalsePositiveKid\|TestFalsePositive" shared/encryption/versioned_test.go`
    - `grep -q "TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy\|TestUnifiedChain" shared/encryption/versioned_test.go`
    - `grep -q "TestMultiKeyEncryptor_GoldenV1\|TestGolden" shared/encryption/versioned_test.go`
    - `cd /home/moersener/Hobby/all-chat && go test ./shared/encryption/... -count=1` exits 0
  </acceptance_criteria>
  <done>All 11 listed test behaviors pass under `go test -count=1`. Golden testdata committed. No use of `any` type. Public API matches PATTERNS.md skeleton exactly.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Migrate token-encryption-backfill off shared/crypto and delete shared/crypto package</name>
  <files>services/auth-service/cmd/token-encryption-backfill/main.go, shared/crypto/crypto.go</files>
  <read_first>
    - services/auth-service/cmd/token-encryption-backfill/main.go (full file — sole caller of shared/crypto)
    - shared/crypto/crypto.go (full file — duplicate to delete; verify no other importers via grep)
    - shared/encryption/versioned.go (created in Task 1)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §1 "Recommendation: Unify Under shared/encryption" (lines 98–105) and §3 "shared/crypto import sites" (lines 302–307)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "shared/crypto/crypto.go (DELETE/deprecate)" section (lines 140–148)
  </read_first>
  <action>
    Step 1 — Verify shared/crypto has exactly one caller:
    ```
    grep -rn '"github.com/caesar/all-chat/shared/crypto"' /home/moersener/Hobby/all-chat/ --include='*.go'
    ```
    Expected output: one line — `services/auth-service/cmd/token-encryption-backfill/main.go`. If MORE than one file is found, STOP and notify the orchestrator (out of scope).

    Step 2 — Edit `services/auth-service/cmd/token-encryption-backfill/main.go`:
    - Replace import `"github.com/caesar/all-chat/shared/crypto"` with `"github.com/caesar/all-chat/shared/encryption"`.
    - Change `cipher crypto.StringCipher` field on `backfillRunner` to `cipher *encryption.AESEncryptor`. (NOTE: Keep this binary on the SINGLE-key `AESEncryptor`, NOT `MultiKeyEncryptor` — this binary is the LEGACY backfill tool from Phase 13's TOKEN_ENCRYPTION_KEY rollout. Phase 14 introduces a NEW binary `cmd/key-rotator` in plan 14-06; the old `token-encryption-backfill` is preserved as-is for historical reproducibility and compiled with the unified package.)
    - In `main()`, replace:
      ```go
      cipher, err := crypto.NewAESGCMCipher(os.Getenv("TOKEN_ENCRYPTION_KEY"))
      ```
      with:
      ```go
      key, err := encryption.ParseKey(os.Getenv("TOKEN_ENCRYPTION_KEY"))
      if err != nil { log.Fatalf("parse TOKEN_ENCRYPTION_KEY: %v", err) }
      cipher, err := encryption.NewAESEncryptor(key)
      ```

    Step 3 — Verify the binary still compiles:
    ```
    cd /home/moersener/Hobby/all-chat/services/auth-service && go build ./cmd/token-encryption-backfill/...
    ```

    Step 4 — Delete `shared/crypto/crypto.go`. Use `git rm shared/crypto/crypto.go`. The directory `shared/crypto/` should be empty afterwards; if there's a `crypto.go.test` or `go.mod` quirk, also remove the directory.

    Step 5 — Verify no remaining references to `shared/crypto`:
    ```
    grep -rn 'shared/crypto' /home/moersener/Hobby/all-chat/ --include='*.go'
    ```
    Expected: zero matches. If any remain, fix or notify.

    Step 6 — Run module-wide build:
    ```
    cd /home/moersener/Hobby/all-chat && go build ./...
    ```
    Expected: clean build, no missing-import errors.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./... && test ! -f shared/crypto/crypto.go && ! grep -rqn 'shared/crypto' --include='*.go' .</automated>
  </verify>
  <acceptance_criteria>
    - `test ! -f shared/crypto/crypto.go` (file deleted)
    - `grep -rn 'shared/crypto' /home/moersener/Hobby/all-chat/ --include='*.go'` returns zero matches
    - `grep -q "encryption.NewAESEncryptor\|encryption.ParseKey" services/auth-service/cmd/token-encryption-backfill/main.go`
    - `! grep -q '"github.com/caesar/all-chat/shared/crypto"' services/auth-service/cmd/token-encryption-backfill/main.go`
    - `cd /home/moersener/Hobby/all-chat/services/auth-service && go build ./cmd/token-encryption-backfill/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go build ./...` exits 0
  </acceptance_criteria>
  <done>shared/crypto package removed. token-encryption-backfill compiles against shared/encryption directly. No regressions in module-wide build.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| K8s Secret → process env | TOKEN_ENCRYPTION_KEY_V<n> values flow from `allchat-secrets` into pod env at start; the encryption package reads them via `os.Getenv` |
| Plaintext PII → DB | OAuth access/refresh tokens (Twitch, YouTube, ElevenLabs API keys) cross from process memory into PostgreSQL via `MultiKeyEncryptor.EncryptString` |
| DB → plaintext PII | Reverse direction; `MultiKeyEncryptor.DecryptString` reconstitutes secrets in process memory |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-01-01 | Tampering | `MultiKeyEncryptor.DecryptString` false-positive kid (legacy ciphertext whose first byte coincidentally equals a registered kid) | mitigate | TestMultiKeyEncryptor_FalsePositiveKid asserts that AEAD authentication failure on the versioned path falls through to each legacy key; brute-force-find a real legacy blob with `decoded[0]==0x01` rather than mocking |
| T-14-01-02 | Tampering | Kid namespace exhaustion / collision | mitigate | `MaxKid=0x7F` constant + constructor returns `ErrReservedKid` for kid 0x00 or > 0x7F; comments document the kid registry as monotonically allocated |
| T-14-01-03 | Information Disclosure | Test fixtures leaking real keys | accept | golden testdata uses 32-byte all-zero key + 12-byte all-zero nonce + plaintext "test-token-value" — none of which are or could be production values |
| T-14-01-04 | Repudiation | Wire format regression silently breaking decoding of historical ciphertexts | mitigate | Committed golden files (`golden_v1.bin`, `golden_legacy.bin`) lock the wire format; any byte-level change fails `TestGolden*` in CI |
| T-14-01-05 | Denial of Service | `NewMultiKeyEncryptorFromEnv` panicking on malformed env values | mitigate | Constructor wraps `ParseKey`/`NewAESEncryptor` errors with `fmt.Errorf("parse %s: %w", envName, err)` and returns; never panics |
| T-14-01-06 | Tampering | `shared/crypto` package retains stale duplicate primitive that diverges from `shared/encryption` | mitigate | Task 2 deletes `shared/crypto/crypto.go` outright; module-wide build verifies no orphan importers remain |
</threat_model>

<verification>
- `go test ./shared/encryption/... -count=1` — all 11 test behaviors green.
- `go build ./...` from repo root — module-wide compile clean after `shared/crypto` removal.
- `grep -rn 'shared/crypto' --include='*.go'` from repo root — zero matches.
</verification>

<success_criteria>
- `MultiKeyEncryptor` exists in `shared/encryption/versioned.go` with the API surface from PATTERNS.md.
- Golden testdata committed; golden tests are deterministic and reproducible.
- Legacy false-positive recovery proven by a real-data test (no mock).
- `shared/crypto` is deleted; `services/auth-service/cmd/token-encryption-backfill/main.go` uses `shared/encryption` directly.
- `go test ./shared/encryption/... -count=1 -race` exits 0.
- `go build ./...` exits 0.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-01-SUMMARY.md` documenting:
- Final API surface of `MultiKeyEncryptor`.
- Golden ciphertext bytes (in hex, for forensic reference).
- Confirmed: `shared/crypto` had exactly one caller, now migrated.
- Any deviations from PATTERNS.md skeleton (with rationale).
</output>
