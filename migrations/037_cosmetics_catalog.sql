-- 037_cosmetics_catalog.sql
-- Phase 30: Add cosmetic_frames and cosmetic_flairs catalog tables + FK columns on viewer_cosmetics

CREATE TABLE IF NOT EXISTS cosmetic_frames (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    image_url  TEXT NOT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cosmetic_flairs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    image_url  TEXT NOT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ON DELETE SET NULL: admin deleting a catalog entry clears viewer selection gracefully
ALTER TABLE viewer_cosmetics
    ADD COLUMN IF NOT EXISTS avatar_frame_id UUID REFERENCES cosmetic_frames(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS avatar_flair_id UUID REFERENCES cosmetic_flairs(id) ON DELETE SET NULL;
