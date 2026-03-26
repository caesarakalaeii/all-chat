---
phase: quick
plan: 260326-poh
subsystem: frontend
tags: [og-meta, twitter-card, seo, opengraph, next-js]
dependency_graph:
  requires: []
  provides: [og-meta-tags]
  affects: [frontend/src/app/layout.tsx, frontend/src/app/opengraph-image.tsx]
tech_stack:
  added: [next/og ImageResponse, edge runtime OG image]
  patterns: [Next.js App Router OG image auto-discovery via metadataBase]
key_files:
  created:
    - frontend/src/app/opengraph-image.tsx
  modified:
    - frontend/src/app/layout.tsx
decisions:
  - metadataBase set to https://allch.at — makes all relative OG image URLs absolute; required for embeds to resolve correctly
  - openGraph.images and twitter.images omitted — Next.js auto-discovers /opengraph-image from the same directory
  - Barlow Bold 700 fetched from Google Fonts CDN in opengraph-image.tsx — ImageResponse/Satori cannot use next/font
  - title.template: '%s | All-Chat' pattern enables child pages to set contextual titles without repeating the brand name
metrics:
  duration: 4m
  completed: 2026-03-26
  tasks_completed: 2
  files_changed: 2
---

# Quick Task 260326-poh: Open Graph and Meta Embed Tags Summary

**One-liner:** Dynamic OG image via Next.js ImageResponse (edge runtime) with comprehensive Open Graph and Twitter Card metadata for rich Discord/Twitter/Slack embeds.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create dynamic OG image with Next.js ImageResponse | 5bcdae5 | frontend/src/app/opengraph-image.tsx |
| 2 | Update root layout metadata with OG and Twitter Card tags | f544bab | frontend/src/app/layout.tsx |

## What Was Built

### Task 1 — OG Image (`opengraph-image.tsx`)

Created `frontend/src/app/opengraph-image.tsx` using Next.js built-in OG image generation:

- **Size:** 1200x630 (standard OG)
- **Runtime:** `edge` for fast cold starts
- **Design:** Dark background (#0f0f13), bold "All-Chat" title (96px), subtitle, four colored platform badges (Twitch purple, YouTube red, Kick green, TikTok pink), emote provider row (7TV + BTTV + FFZ), bottom punchline "One overlay. Every chat. All platforms."
- **Font:** Barlow Bold 700 fetched from Google Fonts CDN (required by Satori — cannot use next/font)
- **Discovery:** Auto-discovered by Next.js App Router via `metadataBase` — no manual image URL needed in metadata

### Task 2 — Root Layout Metadata (`layout.tsx`)

Updated `metadata` export with:

- `metadataBase: new URL('https://allch.at')` — absolute URL resolution for OG images
- `title.template: '%s | All-Chat'` — enables contextual titles across child pages
- Full `openGraph` block: `type`, `locale`, `url`, `siteName`, `title`, `description`
- `twitter.card: 'summary_large_image'` — renders wide image preview on Twitter/X
- Expanded `keywords` covering all 4 platforms and emote providers
- `robots: { index: true, follow: true }`

## Verification

- `npx tsc --noEmit` passes with zero errors
- `npm run build` succeeds — `/opengraph-image` appears as a dynamic (edge) route in the build output

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- `frontend/src/app/opengraph-image.tsx` — FOUND
- `frontend/src/app/layout.tsx` — FOUND (modified)
- Commit 5bcdae5 — FOUND
- Commit f544bab — FOUND
