# Phase 11: Contract Validation - Research

**Researched:** 2026-02-21
**Domain:** Contract testing and behavioral equivalence validation for drop-in service replacement
**Confidence:** HIGH

## Summary

Phase 11 validates that the InnerTube YouTube listener is a true drop-in replacement for the official YouTube listener by proving behavioral equivalence through comprehensive contract testing. This requires three distinct testing strategies: (1) schema validation with golden files to verify RawChatMessage output format, (2) dual-listener integration testing with live streams to prove <0.1% mismatch over 24 hours, and (3) lifecycle behavior tests to validate connection management and offline detection.

The challenge is comparing two services that produce semantically identical but not byte-identical output (different message IDs, potential message reordering) while running against live YouTube streams in production-like conditions. This requires content-based message correlation, tolerance for timing variations, and robust mismatch investigation workflows.

**Primary recommendation:** Use testify/suite for lifecycle hooks, goldie/v2 for golden file management, nsf/jsondiff for semantic JSON comparison, testcontainers-go for Redis integration tests, and Kubernetes Jobs for the 24-hour dual-listener validation test with structured artifact collection on mismatches.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Test Data Collection:**
- **Live streams only** — All tests run against real-time YouTube live streams, not pre-recorded fixtures
- **Golden files captured from official listener** — Run official youtube-listener, save its RawChatMessage output as ground truth
- **Stream variety:** Test both extremes
  - High-volume chat (>100 messages/sec) — stress test parsing under load
  - Low-activity streams (<5 messages/min) — validate edge cases like empty continuations
- **Data volume:** Sample 100+ messages from 5-10 different streams (not 100 distinct streams)

**Comparison Methodology:**
- **Field-by-field semantic match** — Compare parsed JSON objects field by field, not byte-for-byte string comparison
- **Allow internal IDs to differ** — InnerTube and official API may use different message ID schemes; compare content, not IDs
- **Allow message reordering within time window** — Messages within ~1 second can arrive out of order due to network timing
- **Mismatch calculation:** Both missing messages AND field differences count toward the <0.1% threshold
  - Missing in one listener but not the other = mismatch
  - Present in both but fields differ (excluding allowlisted IDs) = mismatch

**Dual-Listener Orchestration:**
- **Parallel execution** — Official and InnerTube listeners both connect to same live stream simultaneously
- **Message matching by content** — Correlate messages by comparing text/author, robust to ID differences
- **Separate Kubernetes pods** — Deploy each listener in isolated pods for realistic production-like testing
- **Redis Streams production path** — Both listeners publish to Redis, comparator consumes from production pipeline

**Failure Investigation:**
- **Artifacts captured on mismatch:**
  - Raw JSON from both listeners (full RawChatMessage objects)
  - Surrounding context (±5 messages before/after mismatch)
  - InnerTube API response (raw continuation payload)
  - Timestamps and latency metrics (when each listener received message)
- **Diff format:** JSON diff using git-style unified diff format (familiar to developers)
- **Monitoring strategy:** Continuous monitoring with final report after 24 hours (let test run to completion)

### Claude's Discretion

- Reproduction workflow for debugging mismatches (replay, harness, or re-run)
- Exact tolerance value for timestamp window reordering
- Lifecycle test implementation details (connection gating, offline detection)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core Testing Libraries

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| testify | v1.10+ | Test assertions and suite lifecycle | Most popular Go testing library (44k+ stars), provides rich assertions and test suite organization |
| goldie/v2 | v2.5.3+ | Golden file management | Standard library for golden file testing in Go (1.8k+ stars), supports -update flag and diff output |
| nsf/jsondiff | latest | Semantic JSON comparison | Provides field-by-field diff with support for ignoring fields, human-readable output (380+ stars) |
| testcontainers-go | v0.37+ | Redis/PostgreSQL containers | Industry standard for Docker-based integration tests (4k+ stars), ensures clean test isolation |
| go-redis/v9 | v9.7+ | Redis client for test harness | Already used in production, proven compatibility with Redis Streams |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go-cmp | v0.6+ | Struct comparison with ignore options | Alternative to jsondiff for struct-level comparison, useful for lifecycle tests |
| UUID | v1.6+ | Deterministic UUID generation for tests | When testing UUID fields (use SetRand for reproducible UUIDs) |
| kwk/golden | v0.5+ | UUID-agnostic golden file testing | If golden files need automatic UUID normalization (not needed - we ignore IDs) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| goldie/v2 | xorcare/golden | xorcare has similar API but less mature (fewer stars, less documentation) |
| nsf/jsondiff | yudai/gojsondiff | yudai provides more detailed diff types but harder to ignore specific fields |
| testcontainers-go | Manual Docker commands | Testcontainers handles cleanup automatically, prevents orphaned containers |

**Installation:**
```bash
# Core libraries
go get github.com/stretchr/testify/v2@latest
go get github.com/sebdah/goldie/v2@latest
go get github.com/nsf/jsondiff@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/redis@latest

# Supporting
go get github.com/google/go-cmp/cmp@latest
go get github.com/google/uuid@latest
```

## Architecture Patterns

### Recommended Project Structure
```
test/
├── contract/                           # All Phase 11 contract tests
│   ├── schema/                         # TEST-01: Schema validation
│   │   ├── golden/                     # Golden files from official listener
│   │   │   ├── text_message_001.json   # Regular chat message
│   │   │   ├── super_chat_001.json     # Super Chat event
│   │   │   ├── member_joined_001.json  # Membership event
│   │   │   └── ...                     # 100+ samples from 5-10 streams
│   │   ├── schema_test.go              # testify suite for golden file validation
│   │   └── README.md                   # Instructions for capturing golden files
│   ├── dual-listener/                  # TEST-02: 24-hour integration test
│   │   ├── main.go                     # Test harness (runs as Kubernetes Job)
│   │   ├── comparator.go               # Message correlation and diff logic
│   │   ├── artifacts.go                # Mismatch artifact collection
│   │   ├── manifests/                  # Kubernetes Job + ConfigMaps
│   │   │   ├── job.yaml                # Dual-listener test Job
│   │   │   ├── redis.yaml              # Isolated Redis for test
│   │   │   └── secrets.yaml            # Test stream configuration
│   │   └── README.md                   # Running the 24-hour test
│   ├── lifecycle/                      # TEST-03, TEST-04: Lifecycle tests
│   │   ├── connection_test.go          # Connection gating (TEST-03)
│   │   ├── offline_test.go             # Stream offline detection (TEST-04)
│   │   └── testcontainers_suite.go     # Shared testcontainers setup
│   └── deletion/                       # DEL-01, DEL-02: Deletion events
│       ├── single_deletion_test.go     # Single message deletion (DEL-01)
│       ├── deletion_emission_test.go   # Deletion event emission (DEL-02)
│       └── fixtures/                   # InnerTube API responses with deletions
├── shared/
│   ├── message_matcher.go              # Content-based message correlation
│   ├── time_window.go                  # Time window tolerance logic
│   └── uuid_normalizer.go              # UUID allowlist for comparisons
└── README.md                           # Contract testing overview
```

### Pattern 1: Golden File Validation with goldie/v2

**What:** Capture RawChatMessage JSON output from official listener as "golden files", then validate InnerTube output matches field-by-field.

**When to use:** TEST-01 (schema validation), verifying message format compliance across 100+ samples.

**Example:**
```go
// Source: https://pkg.go.dev/github.com/sebdah/goldie/v2
// Adapted for RawChatMessage schema validation

package schema

import (
    "encoding/json"
    "testing"
    "github.com/sebdah/goldie/v2"
    "github.com/stretchr/testify/suite"
)

type SchemaTestSuite struct {
    suite.Suite
    g *goldie.Goldie
}

func (s *SchemaTestSuite) SetupSuite() {
    // Initialize goldie with custom options
    s.g = goldie.New(
        s.T(),
        goldie.WithFixtureDir("golden"),
        goldie.WithNameSuffix(".json"),
        goldie.WithDiffEngine(goldie.ColoredDiff), // Git-style colored diff
    )
}

func (s *SchemaTestSuite) TestTextMessage() {
    // Parse InnerTube message (test data from live stream)
    innerTubeMsg := parseInnerTubeMessage(/* ... */)

    // Normalize fields that are allowed to differ
    normalized := normalizeForComparison(innerTubeMsg)

    // Marshal to JSON
    actual, err := json.MarshalIndent(normalized, "", "  ")
    s.NoError(err)

    // Compare with golden file (captured from official listener)
    s.g.Assert(s.T(), "text_message_001", actual)
}

// Run with -update flag to regenerate golden files:
// go test ./test/contract/schema -update
func TestSchemaValidation(t *testing.T) {
    suite.Run(t, new(SchemaTestSuite))
}

// normalizeForComparison removes fields allowed to differ per user constraints
func normalizeForComparison(msg *RawChatMessage) *RawChatMessage {
    normalized := *msg
    // Allow message_id to differ (InnerTube vs official may use different ID schemes)
    normalized.MessageID = "<normalized>"
    // Allow timestamp microsecond precision differences
    normalized.Timestamp = normalized.Timestamp.Truncate(time.Second)
    return &normalized
}
```

### Pattern 2: Content-Based Message Correlation

**What:** Match messages between official and InnerTube listeners by comparing username+text+timestamp instead of relying on message IDs.

**When to use:** Dual-listener integration test (TEST-02), handling message reordering and ID differences.

**Example:**
```go
// Source: Derived from distributed systems testing patterns
// https://github.com/asatarin/testing-distributed-systems

package shared

import (
    "crypto/sha256"
    "fmt"
    "time"
)

// MessageFingerprint creates content-based identifier for matching
type MessageFingerprint struct {
    Username  string
    Text      string
    Timestamp time.Time // Truncated to 1-second precision
}

func (f MessageFingerprint) Hash() string {
    // Create deterministic hash from content
    data := fmt.Sprintf("%s|%s|%d", f.Username, f.Text, f.Timestamp.Unix())
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash[:8]) // First 8 bytes for readability
}

// MatchMessages correlates messages from two listeners using content fingerprints
func MatchMessages(official, innertube []*RawChatMessage, timeWindow time.Duration) MatchResult {
    officialMap := make(map[string]*RawChatMessage)
    innertubeMap := make(map[string]*RawChatMessage)

    // Build fingerprint maps
    for _, msg := range official {
        fp := MessageFingerprint{
            Username:  msg.Username,
            Text:      msg.Text,
            Timestamp: msg.Timestamp.Truncate(timeWindow),
        }
        officialMap[fp.Hash()] = msg
    }

    for _, msg := range innertube {
        fp := MessageFingerprint{
            Username:  msg.Username,
            Text:      msg.Text,
            Timestamp: msg.Timestamp.Truncate(timeWindow),
        }
        innertubeMap[fp.Hash()] = msg
    }

    // Find mismatches
    var matched, missingInInnerTube, missingInOfficial, contentMismatches int

    for hash, officialMsg := range officialMap {
        if innertubeMsg, exists := innertubeMap[hash]; exists {
            // Matched by fingerprint, check for field differences
            if !compareFields(officialMsg, innertubeMsg) {
                contentMismatches++
            } else {
                matched++
            }
        } else {
            missingInInnerTube++
        }
    }

    for hash := range innertubeMap {
        if _, exists := officialMap[hash]; !exists {
            missingInOfficial++
        }
    }

    return MatchResult{
        Matched:              matched,
        MissingInInnerTube:   missingInInnerTube,
        MissingInOfficial:    missingInOfficial,
        ContentMismatches:    contentMismatches,
        TotalMismatches:      missingInInnerTube + missingInOfficial + contentMismatches,
    }
}

// compareFields performs semantic field comparison excluding allowlisted IDs
func compareFields(official, innertube *RawChatMessage) bool {
    // Use nsf/jsondiff for semantic comparison
    // Ignore: message_id, tags.youtube_message_id (InnerTube may differ)
    return comparePlatform(official, innertube) &&
           compareUsername(official, innertube) &&
           compareText(official, innertube) &&
           compareTimestamp(official, innertube, time.Second) && // 1s tolerance
           compareTags(official, innertube)
}
```

### Pattern 3: Dual-Listener Test Harness with Kubernetes Jobs

**What:** Deploy official and InnerTube listeners as separate pods, consume from Redis Streams, run comparison for 24 hours, collect artifacts on mismatches.

**When to use:** TEST-02 (24-hour integration test), production-like validation.

**Example:**
```yaml
# Source: Kubernetes Job patterns for long-running tests
# test/contract/dual-listener/manifests/job.yaml

apiVersion: batch/v1
kind: Job
metadata:
  name: contract-validation-dual-listener
  namespace: allchat-test
spec:
  backoffLimit: 0  # Don't retry - let test run to completion
  ttlSecondsAfterFinished: 86400  # Keep artifacts for 24 hours
  template:
    spec:
      restartPolicy: Never
      initContainers:
      # Start official YouTube listener
      - name: official-listener
        image: allchat-youtube-listener:latest
        env:
        - name: REDIS_HOST
          value: "redis-test"
        - name: DATABASE_HOST
          value: "postgres-test"
        # ... other env vars

      # Start InnerTube listener in parallel
      - name: innertube-listener
        image: allchat-youtube-listener-innertube:latest
        env:
        - name: REDIS_HOST
          value: "redis-test"
        # ... other env vars

      containers:
      # Main test harness - runs for 24 hours
      - name: comparator
        image: contract-test-harness:latest
        env:
        - name: TEST_DURATION
          value: "24h"
        - name: REDIS_HOST
          value: "redis-test"
        - name: MISMATCH_THRESHOLD
          value: "0.001"  # <0.1%
        - name: TIME_WINDOW_TOLERANCE
          value: "1s"
        volumeMounts:
        - name: artifacts
          mountPath: /artifacts
        command:
        - /bin/contract-test
        - --duration=24h
        - --stream-id=$(STREAM_ID)
        - --threshold=0.1%
        - --artifacts=/artifacts

      volumes:
      - name: artifacts
        persistentVolumeClaim:
          claimName: contract-test-artifacts
---
# PVC for storing mismatch artifacts
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: contract-test-artifacts
  namespace: allchat-test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

**Go test harness (dual-listener/main.go):**
```go
// Source: Long-running integration test patterns
// https://go.dev/blog/testing-time

package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"
    "github.com/redis/go-redis/v9"
)

type TestHarness struct {
    redisClient      *redis.Client
    duration         time.Duration
    streamID         string
    threshold        float64  // 0.001 = 0.1%
    timeWindow       time.Duration
    artifactDir      string
    officialMsgs     []*RawChatMessage
    innertubeMsgs    []*RawChatMessage
    mismatchCount    int
    totalMessages    int
}

func (h *TestHarness) Run(ctx context.Context) error {
    startTime := time.Now()
    endTime := startTime.Add(h.duration)

    // Consume from both Redis Streams in parallel
    errCh := make(chan error, 2)

    go h.consumeStream(ctx, "chat:raw:official", &h.officialMsgs, errCh)
    go h.consumeStream(ctx, "chat:raw:innertube", &h.innertubeMsgs, errCh)

    // Periodic comparison every 5 minutes
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()

        case err := <-errCh:
            if err != nil {
                return fmt.Errorf("stream consumption error: %w", err)
            }

        case <-ticker.C:
            // Perform incremental comparison
            result := MatchMessages(h.officialMsgs, h.innertubeMsgs, h.timeWindow)
            h.mismatchCount = result.TotalMismatches
            h.totalMessages = len(h.officialMsgs) + len(h.innertubeMsgs)

            mismatchRate := float64(h.mismatchCount) / float64(h.totalMessages)

            fmt.Printf("[%s] Progress: %d messages, %.4f%% mismatches\n",
                time.Now().Format(time.RFC3339),
                h.totalMessages,
                mismatchRate*100,
            )

            // Collect artifacts on mismatch
            if result.TotalMismatches > 0 {
                h.collectArtifacts(result)
            }

        case <-time.After(time.Until(endTime)):
            // Final comparison after 24 hours
            return h.finalReport()
        }
    }
}

func (h *TestHarness) collectArtifacts(result MatchResult) {
    timestamp := time.Now().Format("20060102-150405")
    artifactPath := fmt.Sprintf("%s/mismatch-%s.json", h.artifactDir, timestamp)

    artifact := map[string]interface{}{
        "timestamp":             timestamp,
        "mismatches":            result,
        "official_messages":     h.officialMsgs[max(0, len(h.officialMsgs)-10):], // Last 10
        "innertube_messages":    h.innertubeMsgs[max(0, len(h.innertubeMsgs)-10):],
    }

    data, _ := json.MarshalIndent(artifact, "", "  ")
    os.WriteFile(artifactPath, data, 0644)
}

func (h *TestHarness) finalReport() error {
    result := MatchMessages(h.officialMsgs, h.innertubeMsgs, h.timeWindow)
    mismatchRate := float64(result.TotalMismatches) / float64(h.totalMessages)

    report := fmt.Sprintf(`
CONTRACT VALIDATION REPORT
Duration: %s
Total Messages: %d
Matched: %d
Mismatches: %d (%.4f%%)
- Missing in InnerTube: %d
- Missing in Official: %d
- Content Mismatches: %d

RESULT: %s (threshold: %.4f%%)
`,
        h.duration,
        h.totalMessages,
        result.Matched,
        result.TotalMismatches,
        mismatchRate*100,
        result.MissingInInnerTube,
        result.MissingInOfficial,
        result.ContentMismatches,
        func() string {
            if mismatchRate < h.threshold {
                return "PASS"
            }
            return "FAIL"
        }(),
        h.threshold*100,
    )

    fmt.Println(report)
    os.WriteFile(fmt.Sprintf("%s/final-report.txt", h.artifactDir), []byte(report), 0644)

    if mismatchRate >= h.threshold {
        return fmt.Errorf("mismatch rate %.4f%% exceeds threshold %.4f%%",
            mismatchRate*100, h.threshold*100)
    }

    return nil
}
```

### Pattern 4: Testcontainers for Lifecycle Tests

**What:** Use testcontainers-go to spin up Redis/PostgreSQL for isolated lifecycle behavior tests.

**When to use:** TEST-03 (connection gating), TEST-04 (offline detection), DEL-01/DEL-02 (deletion events).

**Example:**
```go
// Source: https://golang.testcontainers.org/modules/redis/
// Adapted for YouTube listener lifecycle testing

package lifecycle

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/suite"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

type LifecycleTestSuite struct {
    suite.Suite
    ctx           context.Context
    redisContainer *redis.RedisContainer
    redisClient   *redis.Client
}

// SetupSuite runs once before all tests - spin up containers
func (s *LifecycleTestSuite) SetupSuite() {
    s.ctx = context.Background()

    // Start Redis container
    var err error
    s.redisContainer, err = redis.Run(
        s.ctx,
        "redis:7-alpine",
        redis.WithLogLevel(redis.LogLevelVerbose),
    )
    s.Require().NoError(err)

    // Get connection string
    endpoint, err := s.redisContainer.Endpoint(s.ctx, "")
    s.Require().NoError(err)

    // Create client
    s.redisClient = redis.NewClient(&redis.Options{
        Addr: endpoint,
    })
}

// TearDownSuite runs once after all tests - cleanup containers
func (s *LifecycleTestSuite) TearDownSuite() {
    if s.redisClient != nil {
        s.redisClient.Close()
    }
    if s.redisContainer != nil {
        s.redisContainer.Terminate(s.ctx)
    }
}

// TEST-03: Connection gating - service should not start polling without monitor command
func (s *LifecycleTestSuite) TestConnectionGating() {
    // Start InnerTube listener without sending monitor command
    listener := NewInnerTubeListener(/* ... */)

    ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
    defer cancel()

    go listener.Start(ctx)

    // Wait 5 seconds
    time.Sleep(5 * time.Second)

    // Verify no messages published to Redis Streams
    count, err := s.redisClient.XLen(ctx, "chat:raw").Result()
    s.NoError(err)
    s.Equal(int64(0), count, "No messages should be published without monitor command")
}

// TEST-04: Stream offline detection - service should stop polling when stream ends
func (s *LifecycleTestSuite) TestStreamOfflineDetection() {
    // Mock InnerTube client that returns offline response after 10 seconds
    mockClient := &MockInnerTubeClient{
        responses: []InnerTubeResponse{
            {Messages: []ChatMessage{/* ... */}}, // First poll: messages
            {Messages: []ChatMessage{/* ... */}}, // Second poll: messages
            {Offline: true},                      // Third poll: stream offline
        },
    }

    listener := NewInnerTubeListenerWithClient(mockClient, /* ... */)

    ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
    defer cancel()

    // Send monitor command
    listener.Monitor("test_stream_id", "UCxxxxxx")

    // Wait for offline detection
    err := listener.WaitForStop(ctx)
    s.NoError(err, "Listener should stop gracefully on offline detection")

    // Verify listener stopped polling
    s.False(listener.IsActive(), "Listener should not be active after offline detection")
}

func TestLifecycleBehaviors(t *testing.T) {
    suite.Run(t, new(LifecycleTestSuite))
}
```

### Anti-Patterns to Avoid

- **Anti-pattern: Sleep-based eventually consistent testing** — Using `time.Sleep()` to wait for messages makes tests flaky. Instead, use context timeouts and active polling with exponential backoff.

- **Anti-pattern: Byte-for-byte string comparison** — Comparing marshaled JSON strings fails due to field ordering. Use semantic JSON diff (jsondiff) or struct comparison (go-cmp).

- **Anti-pattern: Shared test state across goroutines** — Long-running tests with parallel goroutines need proper synchronization. Use channels or sync.Mutex for shared state.

- **Anti-pattern: Ignoring mismatch artifacts** — Not collecting surrounding context makes debugging impossible. Always capture ±5 messages, timestamps, and raw API responses.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Golden file management | Custom file read/write + diff | goldie/v2 | Handles -update flag, diff engines, fixture directory conventions automatically |
| Semantic JSON comparison | Recursive struct comparison | nsf/jsondiff | Handles nested objects, arrays, type mismatches, provides human-readable diff |
| Docker container lifecycle | Manual docker run/stop | testcontainers-go | Automatic cleanup, parallel test isolation, handles port allocation |
| Message correlation | Manual hash maps | Content-based fingerprinting library | Content fingerprinting is deceptively complex (timezone handling, Unicode normalization, case sensitivity) |
| Time window tolerance | Manual timestamp comparison | Truncate + comparison logic | Edge cases: DST transitions, leap seconds, timezone variations |

**Key insight:** Contract testing involves comparing complex nested JSON with tolerance for specific fields (IDs, timestamps). Manual comparison code becomes unmaintainable as schemas evolve. Libraries provide battle-tested edge case handling (null values, missing fields, type coercion).

## Common Pitfalls

### Pitfall 1: Message Reordering False Positives

**What goes wrong:** Messages arrive out of order within ~1 second window due to network timing, causing false mismatch reports.

**Why it happens:** InnerTube polling and official API polling are not synchronized - network latency variations mean messages can arrive in different orders.

**How to avoid:**
1. Group messages by time window (1-second buckets)
2. Sort messages within each bucket by content fingerprint
3. Compare buckets, not individual message sequences

**Warning signs:** High mismatch rates (>1%) with no actual content differences, "Missing in InnerTube" and "Missing in Official" counts roughly equal.

### Pitfall 2: UUID Field Comparison

**What goes wrong:** Every message has a different UUID (message_id), causing 100% mismatch rate if not excluded.

**Why it happens:** UUIDs are generated per listener, not deterministic from content.

**How to avoid:**
1. Maintain allowlist of fields to exclude: `message_id`, `tags.youtube_message_id`
2. Use `normalizeForComparison()` function before comparison
3. Document why each field is excluded (reference user constraints)

**Warning signs:** 100% mismatch rate with only message_id differences in diff output.

### Pitfall 3: Golden File Staleness

**What goes wrong:** Schema changes in official listener (new fields added) cause golden file tests to fail even though InnerTube output is correct.

**Why it happens:** Golden files are snapshots from a specific point in time. YouTube API schema evolves (new badges, new event types).

**How to avoid:**
1. Regenerate golden files monthly (scheduled job)
2. Use semantic comparison (ignore extra fields not in golden file)
3. Version golden files by YouTube API schema version

**Warning signs:** Sudden test failures across all golden files after official listener deployment.

### Pitfall 4: Race Conditions in Dual-Listener Test

**What goes wrong:** Test harness reads from Redis Streams while listeners are still publishing, causing incomplete message sets.

**Why it happens:** XREAD without blocking may miss messages published between reads.

**How to avoid:**
1. Use XREADGROUP with consumer groups (guaranteed delivery)
2. Set BLOCK timeout to ensure blocking reads
3. Track high-water mark (last processed ID) for each stream

**Warning signs:** Message counts differ between runs, intermittent "Missing in X" mismatches.

### Pitfall 5: Kubernetes Job Failure Handling

**What goes wrong:** Job restarts on transient errors (network blip), invalidating 24-hour test data.

**Why it happens:** Default Kubernetes Job backoffLimit causes retries.

**How to avoid:**
1. Set `backoffLimit: 0` (no retries)
2. Use init containers for setup, main container for test
3. Persist artifacts to PVC before exit (even on failure)

**Warning signs:** Job shows multiple pod restarts, test duration resets to 0.

## Code Examples

### Example 1: Capturing Golden Files from Official Listener

```go
// Script: test/contract/schema/capture_golden.go
// Run against live stream with official listener to generate golden files

package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/redis/go-redis/v9"
)

func main() {
    ctx := context.Background()

    // Connect to Redis where official listener publishes
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    goldenDir := "test/contract/schema/golden"
    os.MkdirAll(goldenDir, 0755)

    // Consume from chat:raw stream
    lastID := "0"
    captureCount := 0
    targetCount := 100

    fmt.Printf("Capturing %d messages from official listener...\n", targetCount)

    for captureCount < targetCount {
        // XREAD with BLOCK for new messages
        streams, err := client.XRead(ctx, &redis.XReadArgs{
            Streams: []string{"chat:raw", lastID},
            Count:   10,
            Block:   5 * time.Second,
        }).Result()

        if err == redis.Nil {
            continue // No messages yet
        }
        if err != nil {
            panic(err)
        }

        for _, stream := range streams {
            for _, message := range stream.Messages {
                // Extract RawChatMessage JSON from 'data' field
                dataJSON := message.Values["data"].(string)

                var rawMsg RawChatMessage
                if err := json.Unmarshal([]byte(dataJSON), &rawMsg); err != nil {
                    fmt.Printf("Warning: Failed to parse message: %v\n", err)
                    continue
                }

                // Determine file name based on event type
                filename := fmt.Sprintf("%s_%03d.json", getMessageType(&rawMsg), captureCount+1)
                filepath := filepath.Join(goldenDir, filename)

                // Write pretty-printed JSON
                prettyJSON, _ := json.MarshalIndent(rawMsg, "", "  ")
                if err := os.WriteFile(filepath, prettyJSON, 0644); err != nil {
                    panic(err)
                }

                captureCount++
                fmt.Printf("Captured: %s (%d/%d)\n", filename, captureCount, targetCount)

                lastID = message.ID

                if captureCount >= targetCount {
                    break
                }
            }
        }
    }

    fmt.Printf("\nGolden file capture complete: %d files in %s\n", captureCount, goldenDir)
}

func getMessageType(msg *RawChatMessage) string {
    if msg.EventType != "" {
        return msg.EventType // "super_chat", "member_joined", etc.
    }
    return "text_message" // Regular chat message
}
```

### Example 2: Semantic JSON Diff with nsf/jsondiff

```go
// Source: https://pkg.go.dev/github.com/nsf/jsondiff
// Semantic field-by-field comparison excluding allowlisted fields

package shared

import (
    "encoding/json"
    "fmt"

    "github.com/nsf/jsondiff"
)

// CompareMessages performs semantic JSON comparison between official and InnerTube messages
// Returns (matched bool, diff string, error)
func CompareMessages(official, innertube *RawChatMessage) (bool, string, error) {
    // Normalize both messages (remove allowlisted fields)
    normalizedOfficial := normalizeForComparison(official)
    normalizedInnerTube := normalizeForComparison(innertube)

    // Marshal to JSON
    officialJSON, err := json.Marshal(normalizedOfficial)
    if err != nil {
        return false, "", fmt.Errorf("marshal official: %w", err)
    }

    innertubeJSON, err := json.Marshal(normalizedInnerTube)
    if err != nil {
        return false, "", fmt.Errorf("marshal innertube: %w", err)
    }

    // Perform semantic diff
    opts := jsondiff.DefaultConsoleOptions()
    diff, explanation := jsondiff.Compare(officialJSON, innertubeJSON, &opts)

    // Check if semantically equivalent
    matched := diff == jsondiff.FullMatch || diff == jsondiff.SupersetMatch

    return matched, explanation, nil
}

// normalizeForComparison removes fields allowed to differ per user constraints
func normalizeForComparison(msg *RawChatMessage) *RawChatMessage {
    normalized := *msg

    // Allow message_id to differ (InnerTube vs official may use different ID schemes)
    normalized.MessageID = "<normalized>"

    // Allow timestamp microsecond precision differences (truncate to 1 second)
    normalized.Timestamp = normalized.Timestamp.Truncate(time.Second)

    // Remove YouTube-specific internal IDs from tags
    if normalized.Tags != nil {
        delete(normalized.Tags, "youtube_message_id")
    }

    return &normalized
}
```

### Example 3: Time Window Tolerance for Message Reordering

```go
// Source: Eventually consistent testing patterns
// https://ondrej-popelka.medium.com/testing-eventual-consistent-systems-settle-down-44d80348625e

package shared

import (
    "sort"
    "time"
)

// GroupByTimeWindow groups messages into time buckets for reordering-tolerant comparison
func GroupByTimeWindow(messages []*RawChatMessage, window time.Duration) [][]*RawChatMessage {
    if len(messages) == 0 {
        return nil
    }

    // Sort by timestamp first
    sorted := make([]*RawChatMessage, len(messages))
    copy(sorted, messages)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].Timestamp.Before(sorted[j].Timestamp)
    })

    // Group into buckets
    var buckets [][]*RawChatMessage
    var currentBucket []*RawChatMessage
    var bucketStart time.Time

    for _, msg := range sorted {
        if len(currentBucket) == 0 {
            // Start new bucket
            bucketStart = msg.Timestamp
            currentBucket = []*RawChatMessage{msg}
        } else if msg.Timestamp.Sub(bucketStart) < window {
            // Add to current bucket
            currentBucket = append(currentBucket, msg)
        } else {
            // Close current bucket, start new one
            buckets = append(buckets, currentBucket)
            bucketStart = msg.Timestamp
            currentBucket = []*RawChatMessage{msg}
        }
    }

    // Append final bucket
    if len(currentBucket) > 0 {
        buckets = append(buckets, currentBucket)
    }

    return buckets
}

// CompareBuckets compares time-windowed buckets allowing reordering within each bucket
func CompareBuckets(officialBuckets, innertubeBuckets [][]*RawChatMessage) MatchResult {
    var matched, contentMismatches, missingInInnerTube, missingInOfficial int

    // Compare bucket by bucket
    for i := 0; i < len(officialBuckets) || i < len(innertubeBuckets); i++ {
        var officialBucket, innertubeBucket []*RawChatMessage

        if i < len(officialBuckets) {
            officialBucket = officialBuckets[i]
        }
        if i < len(innertubeBuckets) {
            innertubeBucket = innertubeBuckets[i]
        }

        // Build content fingerprint maps for each bucket
        result := MatchMessages(officialBucket, innertubeBucket, time.Second)

        matched += result.Matched
        contentMismatches += result.ContentMismatches
        missingInInnerTube += result.MissingInInnerTube
        missingInOfficial += result.MissingInOfficial
    }

    return MatchResult{
        Matched:              matched,
        ContentMismatches:    contentMismatches,
        MissingInInnerTube:   missingInInnerTube,
        MissingInOfficial:    missingInOfficial,
        TotalMismatches:      contentMismatches + missingInInnerTube + missingInOfficial,
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual Docker setup | testcontainers-go | 2020+ | Automatic cleanup, parallel test isolation, 5x faster setup |
| String comparison | jsondiff semantic diff | 2018+ | Handles nested JSON, type mismatches, provides human-readable diffs |
| Fixture-based tests | Live stream golden files | 2023+ | Real-world schema validation, catches API evolution |
| Sequential testing | Parallel Ginkgo tests | 2021+ | 6.5x faster test suite execution (from testing-distributed-systems research) |
| Sleep-based waits | Context timeouts + active polling | 2019+ | Eliminates flaky tests from timing issues |

**Deprecated/outdated:**
- **go-golden** (unmaintained) → Use goldie/v2 instead (active maintenance, more features)
- **testify/require in goroutines** → Use testify/assert (require calls t.FailNow which doesn't work in goroutines)
- **-short flag for long tests** → Use -timeout flag with generous limits (24h+ tests need explicit timeout)

## Open Questions

### 1. Exact Time Window Tolerance Value

**What we know:** User constraint specifies "~1 second" window for message reordering tolerance.

**What's unclear:** Exact value (1s? 1.5s? 2s?) and whether it varies by stream characteristics (high vs low volume).

**Recommendation:** Start with 1 second, make configurable via flag (`--time-window=1s`), monitor mismatch artifacts to tune. High-volume streams may need tighter window (500ms), low-volume may tolerate wider (2s).

### 2. Reproduction Workflow for Debugging Mismatches

**What we know:** Need to debug mismatches when they occur, artifacts collected include raw JSON and surrounding context.

**What's unclear:** Preferred workflow - replay from artifacts (requires mock InnerTube client), harness for manual testing, or re-run test with enhanced logging?

**Recommendation:**
- **Primary:** Replay from artifacts using testcontainers + mock clients (fastest, most reproducible)
- **Secondary:** Enhanced logging mode (`--debug`) that dumps every comparison to disk
- **Tertiary:** Re-run subset test on same stream ID if still live

### 3. Handling Stream End During 24-Hour Test

**What we know:** Test runs for 24 hours against live stream, but most streams don't run 24/7.

**What's unclear:** Should test automatically switch to new stream on same channel? Or fail gracefully?

**Recommendation:** Graceful failure with partial report. Document that test requires 24/7 stream (e.g., news channels). Alternative: Chain multiple streams from same channel (requires stream discovery logic in test harness).

## Sources

### Primary (HIGH confidence)

- **goldie/v2** - [https://pkg.go.dev/github.com/sebdah/goldie/v2](https://pkg.go.dev/github.com/sebdah/goldie/v2) - Golden file testing API and patterns
- **nsf/jsondiff** - [https://pkg.go.dev/github.com/nsf/jsondiff](https://pkg.go.dev/github.com/nsf/jsondiff) - Semantic JSON comparison capabilities
- **testify/suite** - [https://pkg.go.dev/github.com/stretchr/testify/suite](https://pkg.go.dev/github.com/stretchr/testify/suite) - Suite lifecycle hooks (BeforeTest, AfterTest, SetupSuite)
- **testcontainers-go/redis** - [https://golang.testcontainers.org/modules/redis/](https://golang.testcontainers.org/modules/redis/) - Redis container integration testing
- **Redis Streams XREADGROUP** - [https://redis.io/docs/latest/commands/xreadgroup/](https://redis.io/docs/latest/commands/xreadgroup/) - Consumer group patterns for guaranteed delivery

### Secondary (MEDIUM confidence)

- **Testing with golden files in Go** - [https://medium.com/soon-london/testing-with-golden-files-in-go-7fccc71c43d3](https://medium.com/soon-london/testing-with-golden-files-in-go-7fccc71c43d3) - Golden file best practices (verified pattern)
- **Testing distributed systems** - [https://github.com/asatarin/testing-distributed-systems](https://github.com/asatarin/testing-distributed-systems) - Message correlation patterns (curated resource list)
- **Testing eventually consistent systems** - [https://ondrej-popelka.medium.com/testing-eventual-consistent-systems-settle-down-44d80348625e](https://ondrej-popelka.medium.com/testing-eventual-consistent-systems-settle-down-44d80348625e) - Time window tolerance patterns
- **Kubernetes controller parallel tests** - [https://kev.fan/posts/04-k8s-ginkgo-parallel-tests/](https://kev.fan/posts/04-k8s-ginkgo-parallel-tests/) - 6.5x speedup with Ginkgo parallelism
- **kwk/golden UUID-agnostic testing** - [https://github.com/kwk/golden](https://github.com/kwk/golden) - UUID normalization approach (alternative pattern)

### Tertiary (LOW confidence - needs validation)

- **Pact Go contract testing** - [https://github.com/pact-foundation/pact-go](https://github.com/pact-foundation/pact-go) - Consumer-driven contract testing (different paradigm, may not apply)
- **Go testing time blog** - [https://go.dev/blog/testing-time](https://go.dev/blog/testing-time) - Mocking time in tests (useful for lifecycle tests but limited for 24h integration test)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries are widely used (1k+ stars), actively maintained, proven in production
- Architecture: HIGH - Patterns validated by existing codebase (testify already used), user constraints are specific
- Golden file patterns: HIGH - Direct documentation from goldie/v2 pkg.go.dev, matches Go stdlib conventions
- Dual-listener test: MEDIUM - Kubernetes Job patterns are standard, but 24-hour duration is atypical (needs monitoring validation)
- Message correlation: MEDIUM - Content-based fingerprinting is common pattern, but reordering tolerance needs tuning
- Pitfalls: HIGH - Derived from existing test code (parser_test.go) and distributed systems testing research

**Research date:** 2026-02-21
**Valid until:** 60 days (stable Go ecosystem, libraries have mature APIs)

**Gaps identified:**
1. No existing dual-listener test pattern in codebase - will need to prototype
2. Time window tolerance value needs empirical tuning (start with 1s, adjust based on mismatch artifacts)
3. 24-hour test monitoring strategy not fully specified (recommend Prometheus metrics + Grafana dashboard)
