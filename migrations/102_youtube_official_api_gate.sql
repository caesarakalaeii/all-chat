-- All-Chat Migration 102: YouTube official-API mode feature gate
-- Migration: 102
--
-- Registers the 'youtube_official_api' feature gate (ADR-0008, ADR-0065): a per-source
-- opt-in (overlay_chat_sources.config->>'official_api') that routes a channel the
-- streamer owns to the Data API listener (services/youtube-listener), which discovers
-- unlisted, members-only and "Stream now" broadcasts with the owner's token.
--
-- Seeded is_premium=TRUE: every claimed channel spends YouTube Data API quota (discovery
-- plus 5 units per streamList call), a shared, capped resource the innertube listener
-- does not touch. The listener re-reads this gate on every sync round, so flipping the
-- row to FALSE via the feature-gate admin endpoint opens the mode to everyone without a
-- redeploy, and flipping it back drops non-premium channels to innertube.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart (the
-- runner does not track applied migrations), so this script must be safe to
-- re-execute, hence ON CONFLICT DO NOTHING (the row's is_premium is owned by the
-- admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES (
    'youtube_official_api',
    TRUE,
    'Official YouTube API mode for an owned channel: unlisted, members-only and Stream now broadcasts via the Data API listener (ADR-0065)'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
