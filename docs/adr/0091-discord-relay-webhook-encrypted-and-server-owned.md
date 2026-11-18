# ADR-0091: Discord relay webhook URLs are encrypted at rest and server-owned

**Date**: 2026-11-03
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

A Discord source with relay enabled posts the overlay's chat from other platforms into a
Discord channel. discord-listener provisions an "AllChat Relay" webhook in the chosen
`relay_channel_id` with the shared bot and stores its URL in
`overlay_chat_sources.config.relay_webhook_url`. The URL embeds the webhook token, so it is a
credential: anyone holding it can post into that channel as the webhook.

Every other credential is AES-GCM encrypted with the shared token key (`shared/encryption`,
`TOKEN_ENCRYPTION_KEY_V<n>`), and the privacy policy says credentials are encrypted. The webhook
URL was not:

- It was stored in plaintext in the config JSONB.
- overlay-manager returned the whole config, so every source list (owner and admin) carried
  the URL to the browser, and the GDPR data export (auth-service) shipped it in the file,
  although that export's own rule is that credentials never leave the database.
- `PATCH /overlays/:id/sources/:source_id` replaces the whole config with the client's body,
  and `POST /overlays/:id/sources` stores the client's config. A client could therefore set
  `relay_webhook_url` to any URL, and discord-listener would POST the overlay's chat to it.
- The editor saves its loaded config back, so moving a relay to another channel kept the old
  channel's webhook, and the relay manager never noticed a changed URL for an overlay it was
  already relaying, so it kept posting into the old channel until a restart.
- Poster errors (an unexpected status, a transport failure) quoted the full URL and were
  logged.

## Decision Drivers

- Encrypt it with the mechanism and key every other credential uses; no new secret.
- Existing plaintext rows must keep relaying through the rollout and then be encrypted. SQL
  cannot encrypt, so this happens in Go.
- The relay must not stop working if the deployment has not mounted the key yet.
- Nothing outside discord-listener needs the URL: the editor only sets `relay_enabled` and
  `relay_channel_id`.

## Considered Options

1. **Encrypt in place under the same key, server-owned.** The value in
   `config.relay_webhook_url` becomes the `MultiKeyEncryptor` ciphertext. overlay-manager
   never returns the key and never accepts it from a client.
2. **A separate column or table with an `encryption_version`,** like the OAuth token tables.
   This needs a migration, a second read path and a backfill job for a value only one service
   reads, and it would still have to be hidden from the config the API returns.
3. **Mask the URL in the API response** (for example `https://discord.com/api/webhooks/123/***`)
   instead of removing it. The editor has no use for it, and a masked value written back by
   the editor's save would overwrite the real one.

## Decision Outcome

**Chosen: Option 1.**

### Storage

- discord-listener writes `relay_webhook_url` as `MultiKeyEncryptor` ciphertext (the current
  `TOKEN_ENCRYPTION_KEY_V<n>`). The ciphertext is base64, which has no `:`, so a stored value
  containing `://` is unambiguously a plaintext URL from before this change.
- On every relay sync (every 30 s and on each `chat_source_changes` NOTIFY) discord-listener
  decrypts the stored values. A plaintext value still relays and is rewritten encrypted, and so
  is a value encrypted under a key other than the current one. The key-rotator sweeper does not
  cover this JSONB key, so rotation for it happens here. The rewrite is
  `jsonb_set(...) WHERE id = $1 AND config->>'relay_webhook_url' = <value read>`, so it never
  overwrites a webhook provisioned in the meantime.
- A value that cannot be decrypted, or that is not an `https://discord.com/api/webhooks/` (or
  `discordapp.com`) URL, is skipped and logged by source id. One bad row cannot stop the other
  relays, and a value injected while the config was client-writable is never posted to.
- The key is optional in discord-listener, as it is in the other listeners. Without
  `TOKEN_ENCRYPTION_KEY_V1` the service logs a warning at startup, relays plaintext rows and
  stores new webhook URLs in plaintext; encrypted rows cannot be read until the key is
  mounted. The relay therefore never breaks on a deploy that runs ahead of the manifest, but
  the gap stays open until the key is mounted.
- The URL is never logged. The poster's errors name only the webhook id.

### API

- `models.ChatSource` marshals its config without `relay_webhook_url`, so no overlay-manager
  response (source list, create, admin source list) carries it.
- `POST` and `PATCH` drop a client-supplied `relay_webhook_url`. `PATCH` carries the stored
  value over while `relay_channel_id` is unchanged; a different channel (or turning the relay
  off, which clears the channel) leaves it unset, so discord-listener provisions a webhook in
  the new channel, reusing an existing "AllChat Relay" webhook there.
- The relay manager restarts an overlay's relay when its webhook URL changes, so a moved relay
  stops posting into the old channel.
- The GDPR data export removes the key from each source's config.

### Consequences

- Good: the webhook URL is encrypted at rest under the key every other credential uses, never
  reaches a browser or an export file, and cannot be pointed elsewhere by a client.
- Good: no migration and no backfill job; legacy rows are encrypted on the first sync after the
  key is mounted.
- Bad: discord-listener now depends on `TOKEN_ENCRYPTION_KEY_V1`. The deployment has to mount it
  (secret `allchat-secrets`, key `token-encryption-key-v1`), and during a key rotation it has
  to get the new key like the other consumers; it moves its rows within one sync.
- Bad: until the key is mounted, new webhooks are still stored in plaintext. This was chosen
  over failing the relay.
- Neutral: the config written by a save no longer round-trips the URL through the browser, so a
  save made while a webhook is being provisioned can drop it; the next sync provisions it again.

## Implementation

- `services/discord-listener/relay/repository.go`: encrypt on `StoreWebhookURL`, decrypt,
  validate and re-encrypt in `GetRelayConfigs`.
- `services/discord-listener/relay/manager.go`: follow a changed webhook URL.
- `services/discord-listener/relay/poster.go`: redacted errors.
- `services/discord-listener/cmd/main.go`: optional cipher from `TOKEN_ENCRYPTION_KEY_V<n>`.
- `services/overlay-manager/models/chat_source.go` (`RelayWebhookURLKey`, `MarshalJSON`) and
  `handlers/sources.go` (`keepRelayWebhook`).
- `services/auth-service/handlers/data_export.go`: `config - 'relay_webhook_url'`.
- caesar-deployment: mount `TOKEN_ENCRYPTION_KEY_V1` on `discord-listener-deployment.yaml`.

## Related Decisions

- [ADR-0048](./0048-delegated-overlay-moderators.md): the Discord source guard that validates
  `relay_channel_id` ownership.
- [ADR-0089](./0089-retention-sweeper-cronjob.md): which credentials are deleted when; a relay
  webhook goes with its source row.
