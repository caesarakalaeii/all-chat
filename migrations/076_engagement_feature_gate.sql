-- All-Chat Migration 076: Engagement feature gate
--
-- Registers the 'engagement' feature gate (ADR-0008) so STARTING All-Chat polls/predictions
-- (issue #523) can roll out premium-first. Seeded is_premium=TRUE: only premium users may
-- open a round, because opening one posts the round + participate link to chat
-- (announce_on_start) which consumes the streamer's send quota. Flip to is_premium=FALSE via
-- the feature-gate admin endpoint to graduate to all users — no redeploy.
--
-- engagement-service applies shared/middleware.RequirePremium("engagement") to CreatePoll /
-- CreatePrediction only. Viewer participation (vote/wager/balance/heartbeat) and points
-- earning are deliberately NOT gated — a viewer never needs premium to take part, and points
-- accrual sends no messages.
--
-- IDEMPOTENCY: the runner replays every migration on each pod restart and does not track
-- applied migrations, so this must be safe to re-run — hence ON CONFLICT DO NOTHING (the
-- row's is_premium is owned by the admin toggle thereafter and must not be reset by a re-run).

BEGIN;

INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('engagement', TRUE, 'Start All-Chat polls/predictions (posts to chat, uses send quota) — issue #523')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
