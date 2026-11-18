# ADR-0091: Discord relay webhook URLs are encrypted at rest and server-owned

**Date**: 2026-11-03
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

A Discord source with relay enabled posts the overlay's chat from other platforms into a
Discord channel. discord-listener provisions an "AllChat Relay" webhook in the chosen
`relay_channel_id` with the shared bot and stored its URL in
`overlay_chat_sources.config.relay_webhook_url`. That URL embeds the webhook token, so it
is a credential, but it sat in plaintext in the config JSONB: overlay-manager returned
the whole config (owner and admin source lists, and the GDPR data export, though that
export's rule is that credentials never leave the database), `POST`/`PATCH` sources
stored the client's config verbatim so any client could point the relay at any URL, the
editor's save kept the old channel's webhook after a move (the relay manager never
noticed a changed URL until a restart), and poster errors logged the full URL.

Constraints: encrypt under the mechanism and key every other credential uses; plaintext
rows keep relaying through the rollout and are then encrypted (SQL cannot encrypt, so Go);
the relay must not break when the deployment has not mounted the key yet; nothing outside
discord-listener needs the URL.

## Considered options

- Encrypt in place under the same key, server-owned (chosen): the stored value becomes
  `MultiKeyEncryptor` ciphertext; overlay-manager never returns or accepts it.
- A separate column/table with an `encryption_version` like the OAuth tables: a migration,
  a second read path and a backfill for a value only one service reads, still hidden from
  the API. Rejected.
- Mask the URL in responses instead of removing it: the editor has no use for it, and the
  editor's save would write the mask back over the real URL. Rejected.

## Decision

discord-listener writes `relay_webhook_url` as `MultiKeyEncryptor` ciphertext (base64 has
no `:`, so a value containing `://` is unambiguously a legacy plaintext URL). On every
relay sync (30 s and each `chat_source_changes` NOTIFY) it decrypts the stored values:
plaintext still relays and is rewritten encrypted, as is a value under an older key (the
key-rotator does not cover this JSONB key; rotation happens here). The rewrite is
`jsonb_set(...) WHERE id = $1 AND config->>'relay_webhook_url' = <value read>`, so it never
overwrites a webhook provisioned meanwhile. Undecryptable values, or values that are not
`https://discord.com/api/webhooks/` (or `discordapp.com`) URLs, are skipped and logged by
source id: one bad row stops nothing, and a value injected while config was client-
writable is never POSTed to. The key is optional like in other listeners; without it the
service warns at startup, relays plaintext rows and stores new URLs plaintext until
`TOKEN_ENCRYPTION_KEY_V1` is mounted. Poster errors name only the webhook id.

API side: `models.ChatSource` marshals its config without `relay_webhook_url` (no source
list or create response carries it); `POST`/`PATCH` drop a client-supplied URL (PATCH
carries the stored value over only while `relay_channel_id` is unchanged; a channel
change or relay-off leaves it unset, so discord-listener provisions in the new channel,
reusing an existing webhook); the relay manager restarts a relay whose URL changed; the
data export drops the key.

## Consequences

- The URL is encrypted at rest under the standard credential key, never reaching a
  browser or export file, and cannot be redirected by a client; no migration or backfill,
  legacy rows are encrypted on the first sync after the key is mounted.
- discord-listener now depends on `TOKEN_ENCRYPTION_KEY_V1` (mount the
  `allchat-secrets`/`token-encryption-key-v1` key; during a rotation it moves its rows in
  one sync). Until then new webhooks are stored plaintext, chosen over failing the relay.
- A save made while a webhook is being provisioned can drop it; the next sync provisions
  again.

## Implementation

- `services/discord-listener/relay/repository.go` (encrypt, decrypt, validate, re-encrypt),
  `relay/manager.go` (follow URL changes), `relay/poster.go` (redacted errors),
  `cmd/main.go` (optional cipher from `TOKEN_ENCRYPTION_KEY_V<n>`); overlay-manager
  `models/chat_source.go` (`RelayWebhookURLKey`, `MarshalJSON`) and
  `handlers/sources.go` (`keepRelayWebhook`); auth-service `handlers/data_export.go`;
  caesar-deployment mounts the key on `discord-listener-deployment.yaml`.
