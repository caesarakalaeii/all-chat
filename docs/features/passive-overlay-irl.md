# Passive Overlay Mode (24/7 OBS / IRL Streams)

## Overview

A **passive overlay** renders chat exactly like a normal overlay but does **not**
ask the backend to start capturing chat. It exists for streamers who keep an OBS
instance connected around the clock — most commonly IRL streamers running a 24/7
OBS server for disconnect protection.

Add `?passive=true` to your overlay browser-source URL:

```
https://allch.at/overlay/<overlay-id>?passive=true
```

You then start chat capture **on demand** from your chat monitor when you actually
go live (see [Going live](#going-live) below).

## Why it exists

For YouTube, All-Chat discovers your live stream by polling. To avoid hammering
YouTube for a channel that is offline, discovery **gives up after 1 hour** of not
finding a live stream and parks the channel until it is re-triggered.

A normal overlay asserts "demand" for the whole time it is connected. If your OBS
overlay is connected 24/7, that demand is always on — so when you are offline,
discovery runs, hits the 1-hour limit, and parks. By the time you go live, the
overlay is parked and chat does not appear until you re-trigger discovery.

A **passive** overlay never asserts that demand, so:

- A 24/7 OBS instance can stay connected indefinitely without ever starting the
  discovery clock while you are offline.
- Nothing gets parked, so there is nothing to "un-stick" later.
- You decide exactly when capture starts, right when you go live.

> Passive mode only changes whether the overlay **drives capture**. It still
> receives and renders every chat message once capture is running. Twitch, Kick,
> and TikTok are unaffected by the YouTube discovery timeout, but passive mode is
> safe to use for any overlay.

## Going live

1. Keep your 24/7 OBS browser source on the **passive** URL
   (`…?passive=true`). Leave it running.
2. When you go live, open your overlay's **chat monitor** (the `View` / monitor
   page for the overlay).
3. If chat is not already flowing, use **Rediscover** on the monitor to trigger
   discovery. Capture starts within about a minute.
4. Keep the chat monitor open while you stream — that is what keeps capture
   running for the session. When you close it, capture winds down a few minutes
   later (which is the behaviour you want when the stream ends).

> A plain browser-source refresh does **not** re-trigger a parked YouTube
> channel — use the monitor's **Rediscover** button.

## Reading the platform indicator

The small per-platform status dots on the overlay/monitor now distinguish the
parked state:

| Indicator | Meaning |
|-----------|---------|
| Green | Connected — chat is flowing |
| Yellow (`reconnecting`) | Searching for the stream / retrying |
| **Indigo (`paused`)** | Discovery gave up after 1h and is parked — use the chat monitor's **Rediscover** to retry |
| Red (`error`) | A real error (e.g. auth required) |
| Dim (`offline`) | Not live |

The **indigo "paused"** state is not an error: nothing is broken, it just means
capture is idle and waiting for you to trigger it.

## Quick reference

| You want… | Use |
|-----------|-----|
| A 24/7 OBS overlay that never parks while offline | `…/overlay/<id>?passive=true` |
| A normal overlay that captures whenever connected | `…/overlay/<id>` (no flag) |
| To start capture for a passive overlay | Chat monitor → **Rediscover** |

## How it works (technical)

`?passive=true` makes the overlay's WebSocket connect **without asserting overlay
demand** — the API gateway attaches it via the no-demand path (the same mechanism
used for anonymous participate tabs), skipping source auto-activation while still
delivering chat frames. With no demand asserted, `source-manager` never tells the
YouTube listener to poll, so the 1-hour discovery give-up is never reached.

Triggering **Rediscover** from the monitor publishes a control message that forces
a fresh discovery for the channel; the monitor's own (non-passive) connection then
sustains demand for the live session.
