-- 070_engagement_predictions.sql
-- Description: Cross-platform predictions (issue #523). All-Chat-native predictions
-- (source 'allchat') wager viewer_points; winners split the losers' pool
-- proportionally. Mirrored Twitch-native predictions (source 'twitch_native') are
-- state/tally only -- they use Twitch Channel Points, NOT All-Chat points, so
-- engagement-service NEVER debits/credits viewer_points for twitch_native rows.
-- One wager per viewer per prediction (PK). State machine:
-- CREATED -> ACTIVE -> LOCKED -> (RESOLVED | CANCELED).

CREATE TABLE IF NOT EXISTS predictions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id         UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    source             VARCHAR(16) NOT NULL DEFAULT 'allchat' CHECK (source IN ('allchat', 'twitch_native')),
    external_id        VARCHAR(64),                     -- Twitch prediction id when mirrored
    title              TEXT NOT NULL,
    state              VARCHAR(16) NOT NULL DEFAULT 'CREATED'
                         CHECK (state IN ('CREATED', 'ACTIVE', 'LOCKED', 'RESOLVED', 'CANCELED')),
    winning_outcome_id UUID,                            -- FK added below (circular with prediction_outcomes)
    auto_lock_at       TIMESTAMP,                       -- optional; restart-safe sweep locks past this
    created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    locked_at          TIMESTAMP,
    resolved_at        TIMESTAMP
);

-- Mirror idempotency: at most one mirrored row per (overlay, Twitch prediction id).
-- Per-overlay, NOT global on (source, external_id) — one Twitch prediction fans out to
-- every overlay sourcing the channel (ADR-0028/0030), and a global unique would reject
-- the 2nd overlay's mirror row AND abort a migration re-run over real data. Final scope
-- here so a re-run is a no-op (P0-1); 071 only drops the retired global name on dev DBs.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_prediction_overlay_source_external
    ON predictions(overlay_id, source, external_id) WHERE external_id IS NOT NULL;

-- At most one live All-Chat-native prediction per overlay (ACTIVE or LOCKED).
CREATE UNIQUE INDEX IF NOT EXISTS uniq_active_pred_per_overlay
    ON predictions(overlay_id) WHERE state IN ('ACTIVE', 'LOCKED') AND source = 'allchat';

CREATE INDEX IF NOT EXISTS idx_predictions_overlay_state ON predictions(overlay_id, state);
CREATE INDEX IF NOT EXISTS idx_predictions_auto_lock ON predictions(auto_lock_at) WHERE state = 'ACTIVE' AND auto_lock_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS prediction_outcomes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prediction_id UUID NOT NULL REFERENCES predictions(id) ON DELETE CASCADE,
    idx           INT NOT NULL,                         -- 1-based display / wager number
    label         TEXT NOT NULL,
    color         VARCHAR(16),
    UNIQUE (prediction_id, idx)
);

-- Circular FK: predictions.winning_outcome_id -> prediction_outcomes.id.
-- Added post-hoc, guarded for idempotent re-runs (runner uses ON_ERROR_STOP=1).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_predictions_winning_outcome'
    ) THEN
        ALTER TABLE predictions
            ADD CONSTRAINT fk_predictions_winning_outcome
            FOREIGN KEY (winning_outcome_id) REFERENCES prediction_outcomes(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS prediction_entries (
    prediction_id     UUID NOT NULL REFERENCES predictions(id) ON DELETE CASCADE,
    viewer_id         UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    outcome_id        UUID NOT NULL REFERENCES prediction_outcomes(id) ON DELETE CASCADE,
    amount            BIGINT NOT NULL CHECK (amount > 0),
    platform          VARCHAR(50),                      -- origin platform of the wager (audit)
    source_message_id UUID,                             -- chat-origin replay dedup
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (prediction_id, viewer_id)              -- one wager per viewer per prediction
);

-- Chat replay dedup, scoped to the round (see 069's uniq_poll_vote_msg_round): one chat
-- message fans out to every overlay sourcing the channel (ADR-0028), so each overlay's
-- prediction legitimately records the SAME source_message_id. Per (prediction_id,
-- source_message_id), NOT global — a global unique would drop the 2nd+ overlay's wager
-- AND abort a re-run over real data. Final scope here (P0-1); 072 drops the retired name.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_pred_entry_msg_round
    ON prediction_entries(prediction_id, source_message_id) WHERE source_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pred_entries_pool ON prediction_entries(prediction_id, outcome_id);
