# Domain Pitfalls: InnerTube YouTube Listener

**Domain:** Adding InnerTube-based YouTube listener as drop-in replacement
**Researched:** 2026-02-21

## Executive Summary

Adding InnerTube as a drop-in replacement for the official YouTube API listener introduces **invisible contract risks** that won't be caught by compilation but **will break production overlays**. The primary danger is schema drift between InnerTube's response format and the existing RawChatMessage contract that downstream services (message-processor, API gateway, overlay UI) depend on. Secondary risks include InnerTube's undocumented instability, faster deletion events exposing race conditions, and rate limiting differences.

**Critical insight:** This isn't about building a YouTube listener—it's about maintaining **behavioral equivalence** with an existing system while swapping the data source. Any divergence in JSON schema, deletion semantics, timestamp formats, or stream discovery logic will manifest as production bugs that integration tests won't catch.

---

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

### Pitfall 1: RawChatMessage Schema Drift

**What goes wrong:** InnerTube returns data in different field names, types, or nested structures than the official API. Listener publishes to Redis Streams with slightly different schema. Message-processor deserializes successfully (JSON is flexible) but downstream logic breaks due to missing/mistyped fields.

**Why it happens:**
- InnerTube uses internal naming conventions (e.g., `authorExternalChannelId` vs `authorDetails.channelId`)
- Type mismatches: InnerTube may use strings where official API uses integers (e.g., `amountMicros` as string vs int64)
- Nested structure differences: InnerTube often nests metadata deeper (e.g., `runs[0].text` vs direct `messageText`)
- Optional fields: InnerTube may omit fields that official API always includes (e.g., `profileImageUrl` for deleted accounts)

**Consequences:**
- **Silent data loss**: Message-processor normalizer extracts null/empty values, overlays show blank usernames or missing badges
- **Type assertion panics**: Code expecting int64 receives string, crashes message-processor worker goroutine
- **Emote enrichment failure**: Text extraction differences cause emote positions to misalign, breaking emoji/emote rendering
- **Avatar broken images**: URL format differences (YouTube vs YouTube CDN) cause 404s in overlay UI
- **Badge misclassification**: InnerTube boolean flag names differ (`isChatOwner` vs `isOwner`), users lose moderator/member badges

**Prevention:**

1. **Contract tests (Phase 2 - Testing):**
   ```go
   // services/youtube-innertube-listener/api/contract_test.go
   func TestInnerTubeContractMatchesOfficialAPI(t *testing.T) {
       // Golden file: official API RawChatMessage JSON
       officialJSON := loadGoldenFile("testdata/official_raw_message.json")

       // Parse InnerTube response → RawChatMessage
       innertubeJSON := parseInnerTubeMessage(loadInnerTubeResponse("testdata/innertube_response.json"))

       // CRITICAL: Field-by-field equality check
       assert.JSONEq(t, officialJSON, innertubeJSON)

       // Type validation: message_id must be string UUID
       assert.Regexp(t, uuidRegex, innertubeMsg.MessageID)

       // Type validation: timestamp must parse as time.Time
       assert.NotZero(t, innertubeMsg.Timestamp)

       // Tags map schema validation
       requiredTags := []string{"display_name", "channel_id", "is_verified", "is_owner", "is_sponsor", "is_moderator"}
       for _, tag := range requiredTags {
           assert.Contains(t, innertubeMsg.Tags, tag)
           assert.IsType(t, "", innertubeMsg.Tags[tag]) // All tags must be strings
       }
   }
   ```

2. **Schema validation library (Phase 1 - Implementation):**
   ```go
   // shared/validation/schema.go
   func ValidateRawChatMessage(msg *models.RawChatMessage) error {
       // JSON schema validation against canonical definition
       // Catches: missing required fields, wrong types, invalid enum values
       return jsonschema.Validate(msg, rawChatMessageSchema)
   }
   ```

3. **Cross-listener comparison tests (Phase 3 - Validation):**
   - Run both listeners against same live stream simultaneously
   - Compare RawChatMessage outputs for identical chat messages
   - Flag any field-level differences (tool: `cmp.Diff`)

**Detection:**
- Unit test failures in contract tests (before deployment)
- Integration test failures: message-processor normalizer returns errors (CI pipeline)
- Production: Sentry errors with "unexpected type for field X" (requires monitoring)
- Production: Overlay UI shows blank/broken user info (requires E2E tests)

**Phase assignment:** **Phase 1 (Core Implementation)** - Schema validation must be built-in from day 1. Contract tests in Phase 2.

---

### Pitfall 2: Deletion Event Semantic Mismatch

**What goes wrong:** InnerTube deletion events use different identifiers, timing, or batching semantics than official API. Message-processor expects specific deletion schema, receives incompatible format, fails to delete messages from overlay buffers.

**Why it happens:**
- **Official API schema (existing):**
  ```json
  {
    "event_type": "message_deletion",
    "event_data": {
      "deletion_type": "single",
      "target_msg_id": "CjoKGkNNbnYwb0xvdHZrQ0ZZRkhBUW9k..."  // YouTube message ID
    }
  }
  ```

- **InnerTube schema (likely different):**
  ```json
  {
    "markChatItemAsDeletedAction": {
      "deletedStateMessage": {...},
      "targetItemId": "CjoKGkNNbnYwb0xv..."  // Same? Or internal ID?
    }
  }
  ```

- **Key differences:**
  1. **ID format**: InnerTube may use internal chat item IDs that don't match YouTube API message IDs stored in message-processor registry
  2. **Batch deletions**: Official API `UserBannedDetails` includes `bannedUserDetails.channelId` for batch deletion. InnerTube may send individual `markChatItemAsDeletedAction` events per message (no batch indicator)
  3. **Timing**: InnerTube receives deletion events faster (WebSocket-like continuations) than official API (2-5 second polling), exposing race conditions where deletion arrives before original message

**Consequences:**
- **Orphaned messages in overlay**: Deleted messages remain visible because message-processor can't find them in registry (ID mismatch)
- **Race condition**: Deletion event processed before original message, overlay shows "[deleted]" placeholder with no context
- **Batch deletion failure**: User ban on official API deletes 50 messages instantly. InnerTube sends 50 individual deletion events, overwhelming message-processor and causing replay lag

**Prevention:**

1. **Deletion contract test (Phase 2):**
   ```go
   func TestDeletionEventSchema(t *testing.T) {
       tests := []struct{
           name string
           innertubeResp []byte
           expectedRawMsg *models.RawChatMessage
       }{
           {
               name: "single_message_deletion",
               innertubeResp: loadTestData("deletion_single.json"),
               expectedRawMsg: &models.RawChatMessage{
                   EventType: "message_deletion",
                   EventData: map[string]interface{}{
                       "deletion_type": "single",
                       "target_msg_id": "CjoKGk...", // CRITICAL: Must match original message_id format
                   },
               },
           },
           {
               name: "user_ban_batch_deletion",
               innertubeResp: loadTestData("deletion_batch.json"),
               expectedRawMsg: &models.RawChatMessage{
                   EventType: "message_deletion",
                   EventData: map[string]interface{}{
                       "deletion_type": "batch",
                       "target_user_id": "UCxxxxx",
                       "target_username": "spammer123",
                   },
               },
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               rawMsg := parseInnerTubeDeletion(tt.innertubeResp)
               assert.Equal(t, tt.expectedRawMsg.EventType, rawMsg.EventType)
               assert.Equal(t, tt.expectedRawMsg.EventData, rawMsg.EventData)
           })
       }
   }
   ```

2. **Message ID tracking strategy (Phase 1):**
   - **Store mapping**: When parsing InnerTube message, store both `innertubeItemId` and generated `messageID` (UUID) in Redis hash
   - **Deletion lookup**: When deletion arrives with `targetItemId`, look up corresponding `messageID` from Redis hash
   - **Fallback**: If lookup fails, log warning but don't crash (InnerTube may send stale deletions)

3. **Race condition buffer (Phase 1):**
   ```go
   // If deletion arrives before message, buffer it for 10 seconds
   type DeletionBuffer struct {
       deletions map[string]time.Time // targetItemId → timestamp
       mu sync.RWMutex
   }

   func (b *DeletionBuffer) CheckPending(itemId string) bool {
       b.mu.RLock()
       defer b.mu.RUnlock()
       _, exists := b.deletions[itemId]
       return exists
   }
   ```

4. **Batch deletion detection (Phase 1):**
   - If 5+ deletion events arrive within 100ms with same `moderator_id`, synthesize batch deletion event
   - Prevents overwhelming downstream with individual events

**Detection:**
- Unit test: Deletion events fail schema validation
- Integration test: Message-processor logs "target message not found in registry"
- Production: Overlay shows deleted messages (visual inspection required)
- Production: Sentry alert "high deletion event rate" (may indicate individual events instead of batches)

**Phase assignment:** **Phase 1 (Core Implementation)** - ID mapping and race condition buffer required from day 1. Batch detection optimization can be Phase 2.

---

### Pitfall 3: InnerTube API Instability (Breaking Changes Without Notice)

**What goes wrong:** InnerTube is an undocumented internal API. YouTube changes field names, response structure, or endpoint behavior without warning. Listener stops working mid-stream, no messages delivered to overlays.

**Why it happens:**
- InnerTube is used by YouTube's web client, not designed for external consumption
- YouTube pushes frontend updates globally, InnerTube schema changes deploy simultaneously
- No versioning, no changelog, no deprecation warnings
- Schema documented by reverse engineering (libraries like YTLiveChat, chat-downloader)

**Consequences:**
- **Field renamed**: `authorExternalChannelId` → `author.id`, parser returns empty `userID`, overlay shows "Unknown User"
- **Nested structure changed**: `messageRendererBase.message.runs[0].text` → `messageText`, text extraction fails, empty messages
- **Continuation token format change**: Existing continuations invalidate, connection drops, reconnection fails (stuck in retry loop)
- **New required header**: InnerTube starts requiring `X-Goog-Visitor-Id`, requests return 400, listener crashes
- **No forward warning**: Issue discovered by users reporting "overlay stopped working," no GitHub issue, no community alert

**Prevention:**

1. **Schema version detection (Phase 1):**
   ```go
   func (p *InnerTubeParser) DetectSchemaVersion(resp []byte) (string, error) {
       // Check for known field patterns to detect schema changes
       var raw map[string]interface{}
       json.Unmarshal(resp, &raw)

       // Version markers (reverse-engineered)
       if _, ok := raw["continuationContents"]; ok {
           if _, ok := raw["continuationContents"].(map[string]interface{})["liveChatContinuation"]; ok {
               return "2024-schema", nil
           }
       }

       // Unknown schema
       return "", fmt.Errorf("unrecognized InnerTube schema")
   }
   ```

2. **Graceful degradation (Phase 1):**
   - If critical field missing, log error but don't crash
   - Publish partial message with `metadata: {"schema_degraded": true}`
   - Alert monitoring system but keep service running

3. **Canary deployment strategy (Phase 3 - Production):**
   - Deploy InnerTube listener to 10% of users first
   - Monitor error rates, message delivery success rate
   - Automatic rollback if error rate > 5%

4. **Schema snapshot testing (Phase 2):**
   ```go
   func TestInnerTubeSchemaStability(t *testing.T) {
       // Golden file: Known-good InnerTube response
       goldenResp := loadGoldenFile("testdata/innertube_response_2026-02-21.json")

       // Parse with current parser
       msgs, err := parser.Parse(goldenResp)
       require.NoError(t, err)

       // Snapshot test: Compare structure
       golden.Assert(t, msgs) // Fails if schema changed
   }
   ```

5. **Community monitoring (Ongoing):**
   - Subscribe to GitHub issues: LuanRT/YouTube.js, Agash/YTLiveChat, xenova/chat-downloader
   - Join Discord servers for InnerTube library maintainers
   - Set up GitHub watch notifications for new issues mentioning "breaking change" or "stopped working"

**Detection:**
- Pre-production: Schema snapshot tests fail (CI pipeline)
- Production: Sentry alert "unknown InnerTube schema version"
- Production: CloudWatch metric "youtube_innertube_parse_errors" > threshold
- Production: User reports "overlay not updating"

**Phase assignment:** **Phase 1 (Core Implementation)** - Version detection and graceful degradation. **Phase 2** - Snapshot tests. **Phase 3** - Canary deployment.

---

### Pitfall 4: Stream Discovery Edge Cases (Multiple Concurrent Streams, Premieres)

**What goes wrong:** InnerTube stream discovery returns different results than official API `search.list`. Listener connects to wrong stream, misses live chat, or connects to premiere chat instead of live chat.

**Why it happens:**
- **Official API behavior (existing):**
  - `search.list` with `eventType=live` returns only currently-live streams
  - Premieres explicitly excluded (different `eventType=upcoming`)
  - Returns array, listener picks first match

- **InnerTube behavior (likely different):**
  - Channel page parsing may show both premiere countdowns and live streams
  - "Live" badge detection may flag premieres as live (timing race condition)
  - Multiple concurrent streams: InnerTube may return all, including unlisted streams

**Consequences:**
- **Wrong stream selected**: Channel has 2 concurrent streams (main + test). InnerTube returns both, listener picks test stream, main stream chat ignored
- **Premiere false positive**: Listener connects to premiere 1 hour early, chat is pre-show only, misses actual live chat when premiere starts
- **Unlisted stream exposed**: InnerTube returns unlisted/private stream not visible via official API, listener connects, user confused why overlay shows unexpected stream

**Prevention:**

1. **Stream filter contract (Phase 1):**
   ```go
   func (d *StreamDiscovery) FilterLiveStreams(streams []InnerTubeStream) []InnerTubeStream {
       var live []InnerTubeStream
       for _, s := range streams {
           // CRITICAL: Must match official API behavior
           if !s.IsLive {
               continue // Skip premieres, scheduled, offline
           }
           if s.IsUnlisted {
               continue // Skip unlisted streams (official API doesn't return these)
           }
           if s.IsPrivate {
               continue
           }
           live = append(live, s)
       }
       return live
   }
   ```

2. **Premiere detection (Phase 1):**
   ```go
   func (s *InnerTubeStream) IsPremiere() bool {
       // InnerTube premiere markers (reverse-engineered)
       return s.LiveStreamability != nil &&
              s.LiveStreamability.LiveStreamabilityRenderer != nil &&
              s.LiveStreamability.LiveStreamabilityRenderer.BroadcastId != "" &&
              s.UpcomingEventData != nil // Has scheduled start time
   }
   ```

3. **Multiple stream handling (Phase 1):**
   - If multiple live streams detected, log warning
   - Sort by viewer count descending (primary stream likely has more viewers)
   - Fallback: Sort by start time descending (most recently started)
   - Configuration flag: `PREFER_STREAM_ID` to force specific stream

4. **Cross-validation with official API (Phase 2):**
   - During testing, run both discovery methods simultaneously
   - Compare results: InnerTube streams vs official API search results
   - Flag mismatches for investigation

**Detection:**
- Unit test: Stream filter removes premieres correctly
- Integration test: Mock InnerTube response with multiple streams, verify selection logic
- Production: User reports "overlay showing wrong stream" (requires user feedback loop)
- Production: Metrics "stream_discovery_mismatch" (requires dual-discovery comparison)

**Phase assignment:** **Phase 1 (Core Implementation)** - Stream filtering logic. **Phase 2** - Cross-validation tests.

---

## Moderate Pitfalls

### Pitfall 5: Timestamp Format Inconsistency

**What goes wrong:** InnerTube timestamps use different format (Unix milliseconds vs RFC3339), message-processor fails to parse, messages rejected.

**Prevention:**
- Normalize all timestamps to `time.Time` in parser (Phase 1)
- Contract test validates timestamp parsing (Phase 2)

---

### Pitfall 6: Rate Limiting and IP Blocking Differences

**What goes wrong:** InnerTube enforces different rate limits than official API. Aggressive continuation polling triggers IP block, listener stops receiving messages.

**Why it happens:**
- Official API uses quota system (1,009,000 units/day), InnerTube uses IP-based rate limiting
- InnerTube default polling interval is 1000ms (vs 2000-5000ms for official API)
- Too-fast polling triggers YouTube's anti-bot detection

**Prevention:**
- Respect InnerTube continuation `timeoutMs` field (typically 3000-8000ms) (Phase 1)
- Exponential backoff on 429/403 responses (Phase 1)
- Configuration: `MIN_POLLING_INTERVAL_MS=2000` override (Phase 1)

**Detection:**
- Production: HTTP 429 errors logged
- Production: Sentry alert "InnerTube rate limit exceeded"

**Phase assignment:** **Phase 1 (Core Implementation)**

---

### Pitfall 7: Continuation Token Lifecycle Management

**What goes wrong:** Continuation tokens expire or invalidate, listener loses chat history, reconnection starts from "now" instead of last known position.

**Why it happens:**
- InnerTube continuation tokens have shorter TTL than expected (5-10 minutes idle)
- Token invalidation on stream end not detected, reconnection fails silently

**Prevention:**
- Continuation expiry detection: Retry with fresh token on invalidation (Phase 1)
- Connection gating: Stop polling immediately when stream ends (Phase 1)
- Fast resume: Store last continuation token in Redis with TTL (Phase 1)

**Phase assignment:** **Phase 1 (Core Implementation)**

---

### Pitfall 8: Badge URL Format Differences

**What goes wrong:** InnerTube badge image URLs use different CDN or format, overlay fails to load badge images.

**Prevention:**
- Badge URL normalization in parser (Phase 1)
- Fallback: Use SVG placeholders from existing youtube-normalizer (Phase 1)
- Contract test: Verify badge URLs return 200 (Phase 2)

**Phase assignment:** **Phase 1 (Core Implementation)**

---

## Minor Pitfalls

### Pitfall 9: Super Chat Amount Parsing (Micros vs Dollars)

**What goes wrong:** InnerTube returns Super Chat amounts in dollars (float), official API uses micros (int64). Amount mismatch breaks tier classification.

**Prevention:**
- Normalize all amounts to micros in parser (Phase 1)
- Contract test validates amount format (Phase 2)

---

### Pitfall 10: User Avatar URL Caching

**What goes wrong:** InnerTube avatar URLs include cache-busting query params, message deduplication treats same user as different user.

**Prevention:**
- Strip query params from avatar URLs before publishing (Phase 1)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| **Phase 1: Core Implementation** | Schema drift (Pitfall 1) | Build-in JSON schema validation, graceful degradation on missing fields |
| **Phase 1: Core Implementation** | Deletion ID mismatch (Pitfall 2) | Implement InnerTube item ID → message ID mapping in Redis |
| **Phase 1: Core Implementation** | Rate limiting (Pitfall 6) | Respect `timeoutMs`, exponential backoff, configurable min interval |
| **Phase 2: Contract Testing** | Integration breaks (Pitfall 1) | Golden file tests, cross-listener comparison, schema snapshot tests |
| **Phase 2: Contract Testing** | Deletion race conditions (Pitfall 2) | Dual-listener deletion event comparison, buffer timing tests |
| **Phase 3: Production Validation** | InnerTube breaking change (Pitfall 3) | Canary deployment (10% traffic), automatic rollback on error spike |
| **Phase 3: Production Validation** | Stream discovery mismatch (Pitfall 4) | Dual-discovery comparison metrics, user feedback loop |
| **Ongoing: Monitoring** | Silent schema changes (Pitfall 3) | Subscribe to InnerTube library GitHub issues, community Discord monitoring |

---

## Testing Strategy for Drop-In Compatibility

### Golden Replay Testing (Phase 2)

**Purpose:** Ensure InnerTube listener produces **byte-for-byte identical** RawChatMessage output as official API listener for same chat messages.

**Implementation:**
```go
// services/youtube-innertube-listener/contract/golden_replay_test.go

func TestGoldenReplay(t *testing.T) {
    // 1. Load golden file: Official API RawChatMessage JSON (pre-recorded from production)
    officialGolden := loadGoldenFile("testdata/official_raw_message_golden.json")

    // 2. Load corresponding InnerTube response (captured from real stream)
    innertubeResp := loadTestData("testdata/innertube_response_for_same_message.json")

    // 3. Parse InnerTube response → RawChatMessage
    parser := NewInnerTubeParser()
    innertubeMsg, err := parser.ParseChatMessage(innertubeResp)
    require.NoError(t, err)

    // 4. Serialize to JSON
    innertubeJSON, _ := json.Marshal(innertubeMsg)

    // 5. CRITICAL: Deep equality check
    assert.JSONEq(t, string(officialGolden), string(innertubeJSON),
        "InnerTube RawChatMessage must match official API golden file exactly")

    // 6. Field-level assertions (fail-fast debugging)
    var officialMsg models.RawChatMessage
    json.Unmarshal(officialGolden, &officialMsg)

    assert.Equal(t, officialMsg.Platform, innertubeMsg.Platform)
    assert.Equal(t, officialMsg.ChannelID, innertubeMsg.ChannelID)
    assert.Equal(t, officialMsg.UserID, innertubeMsg.UserID)
    assert.Equal(t, officialMsg.Username, innertubeMsg.Username)
    assert.Equal(t, officialMsg.Text, innertubeMsg.Text)
    assert.Equal(t, officialMsg.Tags, innertubeMsg.Tags) // CRITICAL: Tags map must match exactly
}
```

**Test data collection:**
1. Run official API listener in production, capture 100 diverse messages to `testdata/official_golden/`
2. For each golden message, capture corresponding InnerTube API response to `testdata/innertube_raw/`
3. Golden replay test iterates all pairs, verifies InnerTube parser produces identical RawChatMessage

**Coverage targets:**
- Regular chat messages (50%)
- Super Chat events (10%)
- Membership events (10%)
- Deletions (single + batch) (10%)
- Moderator messages (10%)
- Verified users (5%)
- Edge cases: empty text, emoji-only, very long messages (5%)

---

### Dual-Listener Validation (Phase 3)

**Purpose:** Run both listeners simultaneously in production canary, compare message delivery in real-time.

**Implementation:**
```go
// services/youtube-listener-validator/main.go (new validation service)

func main() {
    // Subscribe to Redis Streams from both listeners
    officialStream := redis.XRead("chat:raw:official")
    innertubeStream := redis.XRead("chat:raw:innertube")

    for {
        officialMsg := <-officialStream
        innertubeMsg := <-innertubeStream

        // Compare messages (allow 5-second timestamp drift)
        if !messagesEquivalent(officialMsg, innertubeMsg) {
            // Log mismatch
            logger.Error("Message mismatch detected",
                zap.String("official_id", officialMsg.MessageID),
                zap.String("innertube_id", innertubeMsg.MessageID),
                zap.Any("diff", cmp.Diff(officialMsg, innertubeMsg)))

            // Alert PagerDuty if mismatch rate > 1%
            mismatchRate.Inc()
            if mismatchRate.Rate() > 0.01 {
                alerting.Trigger("innertube_contract_violation")
            }
        }
    }
}
```

**Deployment strategy:**
1. Phase 3.1: Deploy validator in staging (1 week)
2. Phase 3.2: Deploy validator to production canary (10% of channels)
3. Phase 3.3: If mismatch rate < 0.1%, proceed to 50% rollout
4. Phase 3.4: If mismatch rate < 0.01%, complete rollout to 100%

---

## Sources

### InnerTube Stability and Common Issues
- [GitHub - LuanRT/YouTube.js](https://github.com/LuanRT/YouTube.js) - Most popular InnerTube JavaScript wrapper
- [GitHub - Agash/YTLiveChat](https://github.com/Agash/YTLiveChat) - .NET InnerTube live chat library with stability warnings
- [xenova/chat-downloader YouTube implementation](https://github.com/xenova/chat-downloader/blob/master/chat_downloader/sites/youtube.py) - Production-grade InnerTube parser with deletion handling
- [InnerTube Schema Instability Discussion](https://news.ycombinator.com/item?id=31021611) - Community discussion on YouTube.js reliability
- [Extract YouTube Transcripts Using Innertube API (2025)](https://medium.com/@aqib-2/extract-youtube-transcripts-using-innertube-api-2025-javascript-guide-dc417b762f49) - Recent breaking changes in transcript API

### Deletion Events and Chat Actions
- [YouTube API samples - Retract Message issue](https://github.com/youtube/api-samples/issues/263) - Official API deletion behavior
- [YouTube Live Chat Messages API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages) - Official deletion schema
- [LiveChatMessages: delete API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete) - Official deletion endpoint

### Rate Limiting and IP Blocking
- [Can YouTube IP Ban You? 2026 Guide](https://multilogin.com/blog/youtube-ip-ban/) - IP-based rate limiting behavior
- [YouTube rate limiting discussion](https://github.com/jdepoix/youtube-transcript-api/issues/511) - Community reports of rate limiting
- [API Rate Limiting Overview](https://getstream.io/glossary/api-rate-limiting/) - General rate limiting concepts

### Contract Testing Strategies
- [Stop Breaking My API: Contract Testing with Pact](https://medium.com/@mohsenny/stop-breaking-my-api-a-practical-guide-to-contract-testing-with-pact-33858d113386)
- [API Contract Testing Guide](https://testfully.io/blog/api-contract-testing/)
- [Golden Files Testing for Go](https://github.com/sebdah/goldie) - Snapshot testing library
- [API Evolution with Contract Testing at eBay](https://innovation.ebayinc.com/stories/api-evolution-with-confidence-a-case-study-of-contract-testing-adoption-at-ebay/)
- [JSON Schema Diff Validator](https://www.npmjs.com/package/json-schema-diff-validator) - Backward compatibility checking

### Schema Validation
- [Contract Testing vs Schema Testing](https://pactflow.io/blog/contract-testing-using-json-schemas-and-open-api-part-1/)
- [Unit Testing Backward Compatibility of Message Format](https://medium.com/javarevisited/unit-testing-backward-compatibility-of-message-format-ada50916a453)
- [Avoiding Production Disasters: Versioning and Backward Compatibility Testing](https://medium.com/qualitynexus/api-versioning-and-backward-compatibility-complete-testing-guide-for-quality-engineers-669d46d204d7)

---

## Confidence Assessment

| Finding Type | Confidence | Reasoning |
|-------------|------------|-----------|
| **Schema drift risk** | HIGH | Verified by examining existing RawChatMessage contract + InnerTube library codebases showing structural differences |
| **Deletion semantics** | HIGH | Confirmed by official API parser code (parser.go lines 98-119) + InnerTube chat-downloader implementation |
| **InnerTube instability** | MEDIUM | Based on community reports (GitHub issues, HN discussion) + library maintainer warnings; no official changelog exists |
| **Stream discovery differences** | MEDIUM | Inferred from InnerTube behavior patterns + official API documentation; requires validation testing |
| **Rate limiting behavior** | MEDIUM | Community-reported IP blocks + InnerTube library documentation warnings |
| **Contract testing strategy** | HIGH | Standard industry practice (Pact, golden files, schema validation) applicable to this use case |

**Overall assessment:** HIGH confidence in identification of critical pitfalls (schema drift, deletion semantics). MEDIUM confidence in InnerTube-specific behavior patterns due to undocumented API. All recommendations are actionable and phase-appropriate.
