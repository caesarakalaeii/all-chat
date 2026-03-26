---
phase: quick
plan: 260326-poh
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/src/app/layout.tsx
  - frontend/src/app/opengraph-image.tsx
autonomous: true
requirements: [og-meta-tags]
must_haves:
  truths:
    - "Sharing allch.at on Discord/Twitter/Slack shows a rich embed with title, description, and branded OG image"
    - "OG image visually communicates multi-platform chat aggregation (Twitch, YouTube, Kick, TikTok) with emote support"
    - "Twitter Card renders as summary_large_image with the OG image"
  artifacts:
    - path: "frontend/src/app/layout.tsx"
      provides: "Comprehensive metadata with OG + Twitter Card tags"
      contains: "metadataBase"
    - path: "frontend/src/app/opengraph-image.tsx"
      provides: "Dynamic OG image generated via Next.js ImageResponse"
      min_lines: 30
  key_links:
    - from: "frontend/src/app/layout.tsx"
      to: "frontend/src/app/opengraph-image.tsx"
      via: "Next.js automatic OG image discovery"
      pattern: "metadataBase.*allch\\.at"
---

<objective>
Add comprehensive Open Graph and Twitter Card metadata to allch.at so that link embeds on Discord, Twitter, Slack, and other platforms serve as compelling advertisements for the project.

Purpose: Currently sharing allch.at links produces bare/minimal embeds. Rich OG tags with a branded image will make every shared link a visual advertisement showcasing multi-platform chat aggregation.
Output: Updated root layout metadata + dynamically generated OG image via Next.js ImageResponse API.
</objective>

<execution_context>
@/home/moersener/.claude/get-shit-done/workflows/execute-plan.md
@/home/moersener/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@frontend/src/app/layout.tsx
@frontend/src/app/overlay/layout.tsx

<interfaces>
From frontend/src/app/layout.tsx:
```typescript
import type { Metadata } from 'next'
// Current metadata is minimal — only title, description, keywords
export const metadata: Metadata = {
  title: 'All-Chat - Multi-Platform Chat Aggregation',
  description: 'Aggregate chat from Twitch, YouTube, and more in one overlay for OBS',
  keywords: ['twitch', 'youtube', 'chat', 'overlay', 'streaming', 'obs'],
}
```

Fonts available in frontend/public/fonts/:
- NoitaBlackletter-Regular.otf
- NoitaBlackletter-Regular.ttf

Google Fonts loaded in layout:
- Barlow (400, 500, 600, 700, 800)
- DM Mono (400, 500)
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create dynamic OG image with Next.js ImageResponse</name>
  <files>frontend/src/app/opengraph-image.tsx</files>
  <action>
Create `frontend/src/app/opengraph-image.tsx` using Next.js built-in OG image generation (next/og ImageResponse). This file is auto-discovered by Next.js App Router — no manual linking needed.

Image specs:
- Size: 1200x630 (standard OG)
- Export `runtime = 'edge'` for fast generation
- Export `alt`, `size`, `contentType` metadata

Visual design (use inline JSX styles, no Tailwind — ImageResponse uses Satori):
- Dark background (#0f0f13 or similar dark theme matching allch.at)
- Large bold title: "All-Chat" at the top
- Subtitle/tagline: "Multi-Platform Chat Aggregation for Streamers"
- Platform badges/labels in a row: Twitch (purple #9146FF), YouTube (red #FF0000), Kick (green #53FC18), TikTok (pink #FE2C55) — each as a colored pill/badge with the platform name
- Below platforms, a smaller row for emote providers: "7TV + BTTV + FFZ Emotes"
- Bottom tagline: "One overlay. Every chat. All platforms." or similar punchy line
- Keep it clean, modern, and readable at small embed sizes

Use fetch to load Barlow font from Google Fonts for Satori rendering (ImageResponse requires explicit font loading — cannot use next/font). Load Barlow Bold (700) weight.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/frontend && npx tsc --noEmit src/app/opengraph-image.tsx 2>&1 | head -20</automated>
  </verify>
  <done>OG image route exists at /opengraph-image, TypeScript compiles without errors, image renders platform badges and project branding</done>
</task>

<task type="auto">
  <name>Task 2: Update root layout metadata with comprehensive OG and Twitter Card tags</name>
  <files>frontend/src/app/layout.tsx</files>
  <action>
Update the `metadata` export in `frontend/src/app/layout.tsx` to include comprehensive meta tags. Keep all existing layout JSX unchanged — only modify the metadata object.

Update metadata to:

```typescript
export const metadata: Metadata = {
  metadataBase: new URL('https://allch.at'),
  title: {
    default: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    template: '%s | All-Chat',
  },
  description: 'Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single real-time overlay for OBS. Supports 7TV, BTTV, and FFZ emotes. Free and open source.',
  keywords: [
    'twitch chat', 'youtube chat', 'kick chat', 'tiktok chat',
    'chat overlay', 'obs overlay', 'streaming', 'multistream',
    'chat aggregator', '7tv', 'bttv', 'ffz', 'emotes',
    'streamer tools', 'live streaming', 'multi-platform chat',
  ],
  openGraph: {
    type: 'website',
    locale: 'en_US',
    url: 'https://allch.at',
    siteName: 'All-Chat',
    title: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    description: 'Aggregate chat from Twitch, YouTube, Kick, and TikTok into one real-time overlay. 7TV, BTTV, and FFZ emote support included.',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    description: 'Aggregate chat from Twitch, YouTube, Kick, and TikTok into one real-time overlay. 7TV, BTTV, and FFZ emote support included.',
  },
  robots: {
    index: true,
    follow: true,
  },
}
```

Notes:
- The `openGraph.images` and `twitter.images` fields are NOT needed — Next.js auto-discovers `/opengraph-image` from Task 1
- `metadataBase` is critical — it makes all relative URLs absolute (required for OG images to work)
- The `title.template` enables child pages to set just their own title (e.g., "Dashboard" becomes "Dashboard | All-Chat")
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat/frontend && npx tsc --noEmit src/app/layout.tsx 2>&1 | head -20</automated>
  </verify>
  <done>Root layout has metadataBase set to https://allch.at, openGraph tags with type/locale/url/siteName, twitter card set to summary_large_image, keywords include all four platforms and emote providers, description is compelling and complete</done>
</task>

</tasks>

<verification>
1. `cd frontend && npx tsc --noEmit` — full type check passes
2. `cd frontend && npm run build` — build succeeds (validates OG image route compiles with edge runtime)
3. After deploying, validate with https://www.opengraph.xyz/ or Discord link preview
</verification>

<success_criteria>
- Root layout metadata includes metadataBase, openGraph, twitter card tags
- OG image route exists and renders a branded image with all 4 platform names and emote provider mentions
- TypeScript compiles without errors
- `npm run build` succeeds
</success_criteria>

<output>
After completion, create `.planning/quick/260326-poh-update-open-graph-and-meta-embed-tags-fo/260326-poh-SUMMARY.md`
</output>
