---
phase: quick
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - services/discord-listener/relay/poster.go
  - services/discord-listener/relay/manager.go
  - services/discord-listener/relay/repository.go
  - services/discord-listener/relay/poster_test.go
autonomous: true
requirements: [WEBHOOK-RELAY]
must_haves:
  truths:
    - "Relayed messages appear in Discord with the sender's avatar and display name"
    - "Webhook username shows platform origin (e.g. 'alice [Twitch]')"
    - "Rate limit retry and silent 403/404 drop behavior is preserved"
    - "Old channel-based relay configs without webhook URL are skipped gracefully"
  artifacts:
    - path: "services/discord-listener/relay/poster.go"
      provides: "WebhookPoster implementing DiscordPoster interface"
      contains: "webhookPoster"
    - path: "services/discord-listener/relay/repository.go"
      provides: "relayConfig with WebhookURL field"
      contains: "WebhookURL"
    - path: "services/discord-listener/relay/manager.go"
      provides: "relayMessage parsing display_name and avatar_url, webhook-aware posting"
      contains: "DisplayName"
  key_links:
    - from: "services/discord-listener/relay/manager.go"
      to: "services/discord-listener/relay/poster.go"
      via: "DiscordPoster.Post call with webhook params"
      pattern: "poster\\.Post"
    - from: "services/discord-listener/relay/repository.go"
      to: "overlay_chat_sources.config JSONB"
      via: "SQL query extracting relay_webhook_url"
      pattern: "relay_webhook_url"
---

<objective>
Switch Discord relay from bot-token channel messages to webhook-based sending so relayed messages display the original sender's avatar, display name, and platform badge in Discord.

Purpose: Relayed messages currently appear as the bot user. Webhooks allow per-message identity (avatar + username), making cross-platform chat feel native in Discord.
Output: Updated poster, manager, repository, and tests in services/discord-listener/relay/
</objective>

<execution_context>
@/home/moersener/.claude/get-shit-done/workflows/execute-plan.md
@/home/moersener/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@services/discord-listener/relay/poster.go
@services/discord-listener/relay/manager.go
@services/discord-listener/relay/repository.go
@services/discord-listener/relay/poster_test.go
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Replace channel poster with webhook poster and update DiscordPoster interface</name>
  <files>services/discord-listener/relay/poster.go, services/discord-listener/relay/poster_test.go</files>
  <behavior>
    - Test: WebhookPoster sends POST to correct webhook URL path with username, avatar_url, content in JSON body
    - Test: WebhookPoster formats username as "{display_name} [{Platform}]" (title-cased platform)
    - Test: WebhookPoster falls back to username when display_name is empty
    - Test: 429 rate limit triggers single retry with Retry-After sleep (same as current behavior)
    - Test: 403/404 silently dropped (nil error returned)
    - Test: 204 No Content treated as success (webhook default response)
    - Test: formatWebhookUsername("alice", "twitch") returns "alice [Twitch]"
    - Test: formatWebhookUsername("", "twitch") with empty display name uses username fallback
  </behavior>
  <action>
1. Change the `DiscordPoster` interface signature to:
   ```go
   type DiscordPoster interface {
       Post(ctx context.Context, webhookURL string, msg RelayPayload) error
   }
   ```
   Where `RelayPayload` is a new exported struct:
   ```go
   type RelayPayload struct {
       Content     string
       Username    string  // pre-formatted: "alice [Twitch]"
       AvatarURL   string  // may be empty
   }
   ```

2. Replace `httpPoster` with `webhookPoster`:
   - Constructor: `NewWebhookPoster(client *http.Client, logger *zap.Logger) DiscordPoster`
   - No bot token needed (webhook token is in the URL)
   - POST body JSON: `{"content": "...", "username": "...", "avatar_url": "..."}` (omit avatar_url if empty)
   - URL is the full webhook URL passed per-call (no baseURL needed, but keep baseURL override for tests)
   - Accept 200 and 204 as success (not 201 like channels)
   - Keep identical 429 retry-once logic and 403/404 silent drop

3. Add `formatWebhookUsername(displayName, platform string) string` function:
   - Returns `"{displayName} [{Platform}]"` with platform title-cased (first letter upper)
   - If displayName is empty, caller should pass username instead (handled in manager)

4. Remove `formatRelayContent` (no longer needed — content is just the message text, identity is in webhook fields).

5. Remove old `httpPoster`, `NewHTTPPoster`, `discordMessageBody`, `platformEmoji` — all replaced.

6. Update poster_test.go: Replace all existing tests with new tests for webhookPoster behavior (see behavior block). Use httptest.NewServer to capture webhook POST requests and verify JSON body contains username, avatar_url, content fields.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/services/discord-listener && go test ./relay/ -run "TestWebhook|TestFormat" -v</automated>
  </verify>
  <done>webhookPoster implements DiscordPoster, POSTs to webhook URLs with identity fields, all tests pass</done>
</task>

<task type="auto">
  <name>Task 2: Update repository, relayMessage, and manager to use webhook URL and sender identity</name>
  <files>services/discord-listener/relay/repository.go, services/discord-listener/relay/manager.go</files>
  <action>
1. **repository.go** — Update `relayConfig` struct:
   ```go
   type relayConfig struct {
       OverlayID  string
       WebhookURL string
   }
   ```
   Update SQL query to extract `relay_webhook_url` from JSONB config instead of `relay_channel_id`:
   ```sql
   SELECT ocs.overlay_id,
          ocs.config->>'relay_webhook_url' AS webhook_url
   FROM overlay_chat_sources ocs
   JOIN overlays o ON o.id = ocs.overlay_id
   WHERE ocs.platform = 'discord'
     AND (ocs.config->>'relay_enabled')::boolean = true
     AND ocs.config->>'relay_webhook_url' IS NOT NULL
     AND o.is_active = true
   ```
   Update `rows.Scan` to scan into `cfg.OverlayID` and `cfg.WebhookURL`.

2. **manager.go** — Update `relayMessage` struct to parse additional user fields:
   ```go
   type relayMessage struct {
       Platform  string `json:"platform"`
       OverlayID string `json:"overlay_id"`
       User      struct {
           Username    string `json:"username"`
           DisplayName string `json:"display_name"`
           AvatarURL   string `json:"avatar_url"`
       } `json:"user"`
       Message struct {
           Text string `json:"text"`
       } `json:"message"`
   }
   ```

3. **manager.go** — Update `activeConf` map type from `map[string]string` to `map[string]string` (still overlay_id -> webhook_url, just different value semantics). Update `SyncRelayConfigs` to use `cfg.WebhookURL` instead of `cfg.RelayChannelID`. Update `drainOverlay` signature and call: pass `webhookURL` instead of `relayChannelID`.

4. **manager.go** — Update `drainOverlay` message handling:
   ```go
   displayName := rm.User.DisplayName
   if displayName == "" {
       displayName = rm.User.Username
   }
   username := formatWebhookUsername(displayName, rm.Platform)
   payload := RelayPayload{
       Content:   rm.Message.Text,
       Username:  username,
       AvatarURL: rm.User.AvatarURL,
   }
   if err := m.poster.Post(ctx, webhookURL, payload); err != nil { ... }
   ```

5. **manager.go** — Update `HandleMessage` to accept the new fields and construct `RelayPayload`. Change signature to:
   ```go
   func (m *Manager) HandleMessage(ctx context.Context, platform, username, displayName, avatarURL, text, overlayID, webhookURL string) error
   ```

6. Update all log messages to reference "webhook_url" instead of "relay_channel_id" or "channel_id".
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/services/discord-listener && go build ./... && go test ./relay/ -v</automated>
  </verify>
  <done>Repository queries relay_webhook_url from JSONB config, manager passes display_name/avatar_url/platform to webhook poster, all code compiles and tests pass</done>
</task>

</tasks>

<verification>
- `cd /home/moersener/Hobby/all-chat/services/discord-listener && go build ./...` compiles without errors
- `cd /home/moersener/Hobby/all-chat/services/discord-listener && go test ./relay/ -v` all tests pass
- `go vet ./relay/` reports no issues
- No references to `relay_channel_id` remain in relay package (replaced by webhook_url)
- No references to `httpPoster` or `NewHTTPPoster` remain (replaced by webhookPoster)
</verification>

<success_criteria>
- DiscordPoster interface accepts webhook URL + RelayPayload (username, avatar_url, content)
- webhookPoster POSTs to Discord webhook endpoint with sender identity fields
- relayMessage parses display_name and avatar_url from Redis Pub/Sub messages
- Webhook username formatted as "{display_name} [{Platform}]"
- Repository reads relay_webhook_url from JSONB config column
- Rate limit retry and silent error drop behavior preserved
- All tests pass, code compiles cleanly
</success_criteria>

<output>
After completion, create `.planning/quick/260325-ljj-discord-relay-webhook-based-sending-with/260325-ljj-SUMMARY.md`
</output>
