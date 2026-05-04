# marketing/

Remotion-based marketing videos for All-Chat. The chat row, platform colors,
and design tokens are kept in sync with `frontend/` so the videos read as the
real product, not a mock.

Two compositions ship today:

| ID                    | Size        | Duration | Use                                |
| --------------------- | ----------- | -------- | ---------------------------------- |
| `HeroShowcase`        | 1920 × 1080 | 40s      | Landing-page hero, YouTube, demos  |
| `HeroShowcaseSocial`  | 1080 × 1920 | 20s      | TikTok / Reels / Shorts            |

## Quick start

```bash
cd marketing
npm install
npm run dev            # opens Remotion Studio
```

Studio runs at http://localhost:3000. The composition list shows both videos —
scrub the timeline, swap parameters, watch changes hot-reload.

## Rendering

```bash
npm run build          # HeroShowcase → out/hero.mp4
npm run build:social   # HeroShowcaseSocial → out/hero-social.mp4
npm run build:all      # both
npm run build:webm     # WebM (alpha-capable, OBS-friendly)

# Single still
npx remotion still HeroShowcase out/cover.png --frame=200
```

## Structure

```
marketing/
├── src/
│   ├── index.ts                          registerRoot
│   ├── Root.tsx                          composition registry, loads fonts + CSS
│   ├── index.css                         Tailwind v4 + design tokens
│   ├── theme/
│   │   └── fonts.ts                      Barlow + DM Mono via @remotion/google-fonts
│   ├── compositions/
│   │   ├── HeroShowcase.tsx              40s landscape, 6 scenes
│   │   └── HeroShowcaseSocial.tsx        20s portrait, 3 scenes
│   ├── scenes/
│   │   ├── LogoIntro.tsx                 brand intro
│   │   ├── DashboardPreview.tsx          mock /dashboard with overlay cards
│   │   ├── MultiPlatformChat.tsx         platform stat cards + chat stream (orientation-aware)
│   │   ├── OverlayEditor.tsx             split-view editor with animated hue slider
│   │   ├── CustomizationFlash.tsx        3 theme variants
│   │   └── Outro.tsx                     CTA
│   ├── primitives/
│   │   ├── ChatMessageRow.tsx            adapted from frontend renderMessage.tsx
│   │   ├── PlatformBadge.tsx
│   │   ├── platform-colors.ts
│   │   ├── types.ts                      narrowed ChatMessage types
│   │   └── FadeWrap.tsx                  scene crossfade helper
│   └── data/
│       ├── mock-messages.ts              hand-crafted ChatMessage[] with revealAt frames
│       └── overlays.ts                   mock overlay cards for DashboardPreview
└── public/
    └── emotes/                           bundled emotes (PNG, first-frame for animated)
        ├── kekw.png                      7TV (01KPY735H8SDHYDA7D2MCYD0DS)
        ├── lul.png                       Twitch global (id 425618)
        ├── pepelove.png                  7TV (01KPTPS7M1N5WTBAA8Q9PRAK8M)
        ├── gigachad.png                  7TV (01KPV7ZF8YAT3Y16EBF8R6VGMD, animated → first frame)
        ├── pog.png                       7TV (01KQ68089PVKPP1PFXM27EW8JT)
        └── caesar/                       channel emotes from ~/Documents/emotes
            ├── a7.png       → caesar51Pls (shy/pensive)
            ├── cheers.png   → caesarCHEERS
            └── a23, b11, b20, b21, c24, e2 — codes pending from channel owner
```

## Refreshing emotes

```bash
# Twitch global (LUL): static-cdn.jtvnw.net by emote id
curl -fSL "https://static-cdn.jtvnw.net/emoticons/v2/425618/default/dark/3.0" -o public/emotes/lul.png

# 7TV: search by name, then download by id
curl -sX POST "https://7tv.io/v3/gql" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ emotes(query: \"KEKW\", limit: 1) { items { id name } } }"}' \
  | jq -r '.data.emotes.items[0].id'
curl -fSL "https://cdn.7tv.app/emote/<id>/4x.png" -o public/emotes/<name>.png

# Animated 7TV emotes (GIGACHAD) only ship as gif/webp; convert first frame:
curl -fSL "https://cdn.7tv.app/emote/<id>/4x.webp" -o /tmp/x.webp
magick "/tmp/x.webp[0]" public/emotes/<name>.png
```

## Authoring more scenes

The pattern: `<AbsoluteFill>` root → use `useCurrentFrame()` and `useVideoConfig()`
for timing → reuse `ChatMessageRow`, `PlatformBadge`, and `PLATFORM_HEX` from
`src/primitives/`. Sequences in the composition handle layout and crossfades.

For orientation-aware layouts: read `width`/`height` from `useVideoConfig()` and
branch on `height > width`. See `MultiPlatformChat.tsx` for the pattern.

## Adapting from the frontend

`ChatMessageRow.tsx` mirrors `frontend/src/lib/renderMessage.tsx`. Two changes:

1. `next/image` → `remotion`'s `<Img>` for deterministic preloading.
2. Wrapped in a row layout (badge stripe / username / message body).

The `ChatMessage` type is a narrowed copy from `frontend/src/lib/types/message.ts`,
plus a `revealAt: number` for frame-based timing.

Design tokens in `src/index.css` mirror `frontend/src/app/globals.css` — when
the frontend evolves its palette or typography, mirror the change here.

## TODO / next iterations

- **Audio**. Drop a music bed in `public/audio/bed.mp3` and add `<Audio src={staticFile('audio/bed.mp3')} />` to each composition root. Royalty-free options: Uppbeat, YouTube Audio Library, Pixabay Music. Per-scene SFX (whoosh, pop) can come from Freesound. License: keep it AGPL-compatible or note clearly in `LICENSES.md`.
- **Real emotes**. The bundled SVGs in `public/emotes/` are stand-ins. Swap for licensed/owned artwork or pre-cache real CDN assets to `public/emotes/`.
- **Captions / on-screen text** for accessibility and silent autoplay (most social feeds mute by default).
- **More scene variants**: OBS browser-source setup, mobile dashboard, share-link flow.
- **Square 1:1 composition** (1080×1080) for Instagram feed.
- **Shared primitives package**. Currently the chat row is *copied* from `frontend/`. If marketing surface grows, extract to `shared/ui-primitives/` so bugfixes propagate.
