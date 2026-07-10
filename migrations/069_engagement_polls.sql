-- 069_engagement_polls.sql
-- Description: Cross-platform polls (issue #523). All-Chat-native polls (source
-- 'allchat', voted via chat command / web page / extension across every platform)
-- and mirrored Twitch-native polls (source 'twitch_native', external_id = Twitch
-- poll id). One vote per viewer per poll (PK). source_message_id gives chat-origin
-- replay dedup so a redelivered engagement:commands entry is a no-op.

CREATE TABLE IF NOT EXISTS polls (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id   UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
    source       VARCHAR(16) NOT NULL DEFAULT 'allchat' CHECK (source IN ('allchat', 'twitch_native')),
    external_id  VARCHAR(64),                          -- Twitch poll id when mirrored
    question     TEXT NOT NULL,
    state        VARCHAR(16) NOT NULL DEFAULT 'ACTIVE' CHECK (state IN ('ACTIVE', 'CLOSED')),
    allow_change BOOLEAN NOT NULL DEFAULT TRUE,         -- may a viewer change their vote while ACTIVE
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    ends_at      TIMESTAMP,                             -- optional soft deadline (display + auto-close sweep)
    closed_at    TIMESTAMP
);

-- Mirror idempotency: at most one mirrored row per (overlay, Twitch poll id). One
-- Twitch poll fans out to EVERY overlay sourcing the channel (ADR-0028/0030), so the
-- uniqueness is per-overlay, NOT global on (source, external_id) — a global unique would
-- reject the second overlay's mirror row AND abort a migration RE-RUN once real
-- multi-overlay data exists (the runner replays every up-migration on each pod start).
-- Created in final per-overlay scope here so re-running this file is always a no-op
-- (P0-1); 071 only drops the retired global name on any dev DB that has it.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_poll_overlay_source_external
    ON polls(overlay_id, source, external_id) WHERE external_id IS NOT NULL;

-- At most one live All-Chat-native poll per overlay (Twitch-native excluded; a
-- channel may briefly have both while mirroring, resolved by handler policy).
CREATE UNIQUE INDEX IF NOT EXISTS uniq_active_poll_per_overlay
    ON polls(overlay_id) WHERE state = 'ACTIVE' AND source = 'allchat';

CREATE INDEX IF NOT EXISTS idx_polls_overlay_state ON polls(overlay_id, state);

CREATE TABLE IF NOT EXISTS poll_options (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    idx     INT NOT NULL,                               -- 1-based display / vote number
    label   TEXT NOT NULL,
    UNIQUE (poll_id, idx)
);

CREATE TABLE IF NOT EXISTS poll_votes (
    poll_id           UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    viewer_id         UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    option_id         UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    platform          VARCHAR(50),                      -- origin platform of the vote (audit)
    source_message_id UUID,                             -- chat-origin replay dedup
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (poll_id, viewer_id)                    -- one vote per viewer per poll
);

-- Chat replay dedup, scoped to the round: a single chat message fans out to every
-- overlay sourcing the channel (ADR-0028), so each overlay's poll legitimately records
-- the SAME source_message_id. Uniqueness is per (poll_id, source_message_id), NOT global
-- on source_message_id — a global unique would drop the 2nd+ overlay's vote AND abort a
-- migration re-run over real multi-overlay data. Final scope here so a re-run is a no-op
-- (P0-1); 072 only drops the retired global name on any dev DB that has it.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_poll_vote_msg_round
    ON poll_votes(poll_id, source_message_id) WHERE source_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_poll_votes_tally ON poll_votes(poll_id, option_id);
