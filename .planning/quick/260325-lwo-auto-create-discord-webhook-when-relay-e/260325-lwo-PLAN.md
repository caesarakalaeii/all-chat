---
phase: quick
plan: 260325-lwo
type: execute
wave: 1
depends_on: []
files_modified:
  - services/discord-listener/relay/webhook_provisioner.go
  - services/discord-listener/relay/repository.go
  - services/discord-listener/relay/manager.go
  - services/discord-listener/cmd/main.go
autonomous: true
requirements: [auto-create-discord-webhook]
must_haves:
  truths:
    - "When relay_enabled=true and relay_channel_id is set but relay_webhook_url is NULL, the bot auto-creates a Discord webhook and stores the URL"
    - "Duplicate webhooks are avoided by checking existing webhooks in the channel first"
    - "Webhook creation failure does not block relay sync for other overlays"
  artifacts:
    - path: "services/discord-listener/relay/webhook_provisioner.go"
      provides: "WebhookProvisioner that creates Discord webhooks via REST API"
    - path: "services/discord-listener/relay/repository.go"
      provides: "GetPendingRelayConfigs and StoreWebhookURL methods"
    - path: "services/discord-listener/relay/manager.go"
      provides: "Provisioning integration in SyncRelayConfigs"
    - path: "services/discord-listener/cmd/main.go"
      provides: "WebhookProvisioner wiring with bot token"
  key_links:
    - from: "services/discord-listener/relay/manager.go"
      to: "services/discord-listener/relay/webhook_provisioner.go"
      via: "provisioner.ProvisionPending called in SyncRelayConfigs"
      pattern: "provisioner\\.ProvisionPending"
    - from: "services/discord-listener/relay/webhook_provisioner.go"
      to: "services/discord-listener/relay/repository.go"
      via: "GetPendingRelayConfigs + StoreWebhookURL"
      pattern: "(GetPendingRelayConfigs|StoreWebhookURL)"
---

<objective>
Auto-create Discord webhooks when relay is enabled on an overlay source. Instead of users manually creating webhooks and pasting URLs, the discord-listener bot creates the webhook via Discord API when it detects relay_enabled=true with a relay_channel_id but no relay_webhook_url.

Purpose: Remove manual webhook setup step from relay configuration -- users just toggle relay_enabled and set channel ID.
Output: WebhookProvisioner + repository methods + manager integration + main.go wiring.
</objective>

<execution_context>
@/home/moersener/.claude/get-shit-done/workflows/execute-plan.md
@/home/moersener/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@services/discord-listener/relay/repository.go
@services/discord-listener/relay/manager.go
@services/discord-listener/relay/poster.go
@services/discord-listener/cmd/main.go

<interfaces>
<!-- Existing relay package types the executor needs -->

From services/discord-listener/relay/repository.go:
```go
type relayConfig struct {
	OverlayID  string
	WebhookURL string
}

type RepositoryInterface interface {
	GetRelayConfigs(ctx context.Context) ([]relayConfig, error)
}

type Repository struct {
	db *pgxpool.Pool
}
```

From services/discord-listener/relay/manager.go:
```go
type Manager struct {
	repo        RepositoryInterface
	poster      DiscordPoster
	redisClient *redis.Client
	logger      *zap.Logger
	dbPool      *pgxpool.Pool
	// ...
}

func NewManager(repo RepositoryInterface, poster DiscordPoster, rdb *redis.Client, dbPool *pgxpool.Pool, logger *zap.Logger) *Manager
func (m *Manager) SyncRelayConfigs(ctx context.Context) error
```

From services/discord-listener/cmd/main.go:
```go
botToken := os.Getenv("DISCORD_BOT_TOKEN")
relayRepo := relay.NewRepository(dbPool)
relayPoster := relay.NewWebhookPoster(&http.Client{Timeout: 10 * time.Second}, log)
relayMgr := relay.NewManager(relayRepo, relayPoster, rdb, dbPool, log)
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add WebhookProvisioner and repository methods</name>
  <files>
    services/discord-listener/relay/webhook_provisioner.go
    services/discord-listener/relay/repository.go
  </files>
  <action>
**repository.go changes:**

1. Add a `pendingRelayConfig` struct:
```go
type pendingRelayConfig struct {
    SourceID   string // overlay_chat_sources.id (needed for UPDATE WHERE)
    OverlayID  string
    ChannelID  string // relay_channel_id from JSONB config
}
```

2. Add `GetPendingRelayConfigs(ctx context.Context) ([]pendingRelayConfig, error)` to `RepositoryInterface` and implement on `Repository`. SQL query:
```sql
SELECT ocs.id, ocs.overlay_id, ocs.config->>'relay_channel_id' AS channel_id
FROM overlay_chat_sources ocs
JOIN overlays o ON o.id = ocs.overlay_id
WHERE ocs.platform = 'discord'
  AND (ocs.config->>'relay_enabled')::boolean = true
  AND ocs.config->>'relay_channel_id' IS NOT NULL
  AND (ocs.config->>'relay_webhook_url' IS NULL OR ocs.config->>'relay_webhook_url' = '')
  AND o.is_active = true
```

3. Add `StoreWebhookURL(ctx context.Context, sourceID, webhookURL string) error` to `RepositoryInterface` and implement on `Repository`. SQL:
```sql
UPDATE overlay_chat_sources
SET config = config || jsonb_build_object('relay_webhook_url', $2),
    updated_at = NOW()
WHERE id = $1
```
Also issue `NOTIFY chat_source_changes` so the relay manager's LISTEN/NOTIFY watcher picks up the change immediately.

**webhook_provisioner.go (NEW file):**

Create `WebhookProvisioner` struct with fields: `botToken string`, `httpClient *http.Client`, `repo RepositoryInterface`, `logger *zap.Logger`.

Constructor: `NewWebhookProvisioner(botToken string, httpClient *http.Client, repo RepositoryInterface, logger *zap.Logger) *WebhookProvisioner`

Method `ProvisionPending(ctx context.Context) error`:
1. Call `repo.GetPendingRelayConfigs(ctx)` to get sources needing webhooks
2. For each pending config, call `ensureWebhook(ctx, cfg)`
3. Log errors per-source but do NOT return error -- continue to next source

Method `ensureWebhook(ctx context.Context, cfg pendingRelayConfig) error`:
1. First, GET existing webhooks: `GET https://discord.com/api/v10/channels/{channel_id}/webhooks` with `Authorization: Bot {token}`
2. Parse response as `[]discordWebhook` where `discordWebhook` has fields `ID string json:"id"`, `Token string json:"token"`, `Name string json:"name"`, `ApplicationID *string json:"application_id"` (pointer because it can be null for user-created webhooks)
3. Check if any webhook has `Name == "AllChat Relay"` -- if found, construct URL from that webhook's ID+Token and skip creation
4. If not found, POST to create: `POST https://discord.com/api/v10/channels/{channel_id}/webhooks` with body `{"name": "AllChat Relay"}` and `Authorization: Bot {token}`, `Content-Type: application/json`
5. Parse response as `discordWebhook`, construct URL: `https://discord.com/api/webhooks/{id}/{token}`
6. Call `repo.StoreWebhookURL(ctx, cfg.SourceID, webhookURL)` to persist
7. Log success at Info level with overlay_id and channel_id

Handle HTTP errors gracefully:
- 403: Log warning "bot lacks MANAGE_WEBHOOKS permission in channel {id}" and return error
- 404: Log warning "channel {id} not found" and return error
- 429: Parse Retry-After header, sleep, retry once (same pattern as poster.go doPost)
- Other non-2xx: return wrapped error

Define the `discordWebhook` response struct privately in this file.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/services/discord-listener && go build ./...</automated>
  </verify>
  <done>WebhookProvisioner creates Discord webhooks via API with idempotency (checks existing first), repository has GetPendingRelayConfigs and StoreWebhookURL methods, code compiles cleanly.</done>
</task>

<task type="auto">
  <name>Task 2: Wire provisioner into Manager and main.go</name>
  <files>
    services/discord-listener/relay/manager.go
    services/discord-listener/cmd/main.go
  </files>
  <action>
**manager.go changes:**

1. Add `provisioner *WebhookProvisioner` field to `Manager` struct.

2. Update `NewManager` signature to accept an optional provisioner:
```go
func NewManager(repo RepositoryInterface, poster DiscordPoster, rdb *redis.Client, dbPool *pgxpool.Pool, logger *zap.Logger, provisioner *WebhookProvisioner) *Manager
```
Store provisioner in the struct. If nil, provisioning is simply skipped (backwards compatible for tests).

3. In `SyncRelayConfigs`, BEFORE fetching relay configs, add provisioning step:
```go
// Auto-provision webhooks for sources that need them.
if m.provisioner != nil {
    if err := m.provisioner.ProvisionPending(ctx); err != nil {
        if m.logger != nil {
            m.logger.Warn("Webhook provisioning had errors", zap.Error(err))
        }
    }
}
```
This runs first so that newly-provisioned sources are picked up by the existing GetRelayConfigs query in the same sync cycle.

**cmd/main.go changes:**

1. After creating `relayRepo` and `relayPoster`, create the provisioner:
```go
relayProvisioner := relay.NewWebhookProvisioner(botToken, &http.Client{Timeout: 10 * time.Second}, relayRepo, log)
```

2. Update `relay.NewManager` call to pass the provisioner as the last argument:
```go
relayMgr := relay.NewManager(relayRepo, relayPoster, rdb, dbPool, log, relayProvisioner)
```

**Test file updates (if any exist):**
Check for existing test files in the relay package. If `manager_test.go` exists and calls `NewManager`, add `nil` as the last argument to maintain compilation.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/services/discord-listener && go build ./... && go vet ./...</automated>
  </verify>
  <done>Manager calls provisioner.ProvisionPending before each sync cycle. main.go creates provisioner with bot token and passes it to Manager. All code compiles and passes vet. Existing tests (if any) still compile.</done>
</task>

</tasks>

<verification>
1. `cd services/discord-listener && go build ./...` -- compiles without errors
2. `cd services/discord-listener && go vet ./...` -- no issues
3. Review webhook_provisioner.go: confirms idempotent webhook lookup before creation, graceful error handling per-source
4. Review repository.go: GetPendingRelayConfigs returns sources with relay_enabled + channel_id but no webhook_url; StoreWebhookURL updates JSONB config and issues NOTIFY
5. Review manager.go: provisioner.ProvisionPending called before GetRelayConfigs in SyncRelayConfigs
</verification>

<success_criteria>
- WebhookProvisioner auto-creates Discord webhooks for relay-enabled sources missing a webhook URL
- Existing webhooks named "AllChat Relay" are reused (no duplicates)
- Webhook URL is stored in DB JSONB config and picked up on next sync
- Provisioning errors are logged but do not block other overlays
- No changes required from users beyond setting relay_enabled + relay_channel_id
</success_criteria>

<output>
After completion, create `.planning/quick/260325-lwo-auto-create-discord-webhook-when-relay-e/260325-lwo-SUMMARY.md`
</output>
