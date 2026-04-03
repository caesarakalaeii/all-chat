# Phase 9: Alejo Pronouns Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 09-add-optional-support-for-alejo-pronouns
**Areas discussed:** Pronoun display style, Caching strategy, Per-overlay toggle UX, Platform scope

---

## Pronoun Display Style

### Q1: How should pronouns appear next to usernames?

| Option | Description | Selected |
|--------|-------------|----------|
| Parenthesized after name | e.g. "Username (she/her)" — simple, widely recognized convention | |
| Smaller muted text after name | Smaller, dimmer pronoun text — less visually prominent | |
| Badge-style pill | Small colored pill/tag like badges — e.g. [she/her] with background color | ✓ |

**User's choice:** Badge-style pill
**Notes:** None

### Q2: Where should the pronoun pill be positioned?

| Option | Description | Selected |
|--------|-------------|----------|
| After badges, before username | Groups it with identity badges | |
| After username, before colon | Reads as "Username, who uses she/her" | |
| You decide | Claude picks best position | |

**User's choice:** "Let users decide" (free text)
**Notes:** User wants position to be a per-overlay configurable setting, not a fixed position

### Q3: Should the pronoun pill color be configurable?

| Option | Description | Selected |
|--------|-------------|----------|
| Single default style | Subtle muted background matching overlay theme | |
| Configurable per overlay | Add a pronoun pill color picker to overlay settings | ✓ |
| You decide | Claude picks simplest approach | |

**User's choice:** Configurable per overlay
**Notes:** None

---

## Caching Strategy

### Q1: Cache TTL duration?

| Option | Description | Selected |
|--------|-------------|----------|
| 24 hours (match avatars) | Same TTL as avatar cache — consistent pattern | ✓ |
| 7 days | Longer TTL reduces API calls | |
| 1 hour | More responsive but heavier API load | |

**User's choice:** 24 hours (match avatars)
**Notes:** None

### Q2: API failure behavior?

| Option | Description | Selected |
|--------|-------------|----------|
| Silently skip pronouns | Message renders without pronoun — graceful degradation | ✓ |
| Use stale cache if available | Serve expired cached value when API is down | |
| You decide | Claude picks most resilient approach | |

**User's choice:** Silently skip pronouns
**Notes:** None

---

## Per-Overlay Toggle UX

### Q1: Default state for new overlays?

| Option | Description | Selected |
|--------|-------------|----------|
| Disabled by default | Opt-in — streamers enable when they want it | |
| Enabled by default | Opt-out — pronouns show immediately | ✓ |

**User's choice:** Enabled by default
**Notes:** None

### Q2: Settings UI location?

| Option | Description | Selected |
|--------|-------------|----------|
| In Visibility group | Next to show_badges, show_avatars in VisibilityGroup.tsx | ✓ |
| Own section | Dedicated "Pronouns" section — more prominent | |
| You decide | Claude picks best fit | |

**User's choice:** In Visibility group
**Notes:** None

---

## Platform Scope

### Q1: Non-Twitch message handling?

| Option | Description | Selected |
|--------|-------------|----------|
| Twitch only | Only look up pronouns for Twitch messages | |
| Try all platforms by username | Attempt Alejo lookup for all platform usernames | |
| You decide | Claude picks pragmatic approach | |

**User's choice:** Free text — "We have cross platform viewer info for our viewer endpoints, if a registered viewer sends a message and has a twitch account linked use that one"
**Notes:** User wants to leverage existing viewer identity system for cross-platform pronoun resolution

### Q2: How to resolve cross-platform identity?

| Option | Description | Selected |
|--------|-------------|----------|
| Piggyback on viewer enricher | Reuse viewer identity already resolved by viewer_badge_enricher | ✓ |
| Independent lookup | Pronoun enricher does its own viewer identity resolution | |
| You decide | Claude picks most efficient approach | |

**User's choice:** Piggyback on viewer enricher
**Notes:** Zero extra DB queries — reuse existing viewer identity data

---

## Claude's Discretion

- Enricher ordering in pipeline
- Redis cache key format details
- Alejo API HTTP client configuration
- Pronoun pill CSS styling specifics
- Default pronoun pill color
- How to pass resolved Twitch username between enrichers

## Deferred Ideas

None — discussion stayed within phase scope
