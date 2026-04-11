# Phase 11: Add username/keyword exclude list to overlay filter settings - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 11-add-username-keyword-exclude-list-to-overlay-filter-settings
**Areas discussed:** Filtering location, Matching behavior, Editor preview, Default presets

---

## Filtering Location

| Option | Description | Selected |
|--------|-------------|----------|
| Client-side only | Filter in overlay page JS before rendering. Simplest — no backend changes. Message-processor stays stateless. | ✓ |
| Server-side only | Filter in message-processor before Pub/Sub publish. Saves bandwidth but adds config-fetching complexity. | |
| Both | Client + server. Most work but belt-and-suspenders approach. | |

**User's choice:** Client-side only (Recommended)
**Notes:** Keeps message-processor stateless, no backend coupling needed.

---

## Matching Behavior — Keywords

| Option | Description | Selected |
|--------|-------------|----------|
| Substring, case-insensitive | "spam" blocks "spammy", "buy-spam-here" etc. | |
| Exact word match, case-insensitive | "spam" blocks "spam" but not "spammy" | |
| Regex support | Full regex patterns for power users | ✓ |

**User's choice:** Regex support
**Notes:** "Performance issues are not a concern if client side only, power user errors are a skill issue"

## Matching Behavior — Usernames

| Option | Description | Selected |
|--------|-------------|----------|
| Exact match, case-insensitive | "nightbot" matches "Nightbot" and "NIGHTBOT" | ✓ |
| Exact match, case-sensitive | "Nightbot" only matches "Nightbot" | |

**User's choice:** Exact match, case-insensitive (Recommended)
**Notes:** None

---

## Editor Preview

| Option | Description | Selected |
|--------|-------------|----------|
| Apply filters in preview | WYSIWYG — filtered messages don't appear. Test filters immediately. | ✓ |
| Show all, mark filtered | All messages visible but filtered ones visually marked (strikethrough/dimmed). | |
| No filtering in preview | Filters only on live overlay. Simplest but can't test. | |

**User's choice:** Apply filters in preview (Recommended)
**Notes:** None

---

## Default Presets

| Option | Description | Selected |
|--------|-------------|----------|
| Quick-add button with common bots | "Add common bots" populates Nightbot, StreamElements, Moobot, etc. | ✓ |
| Platform-specific presets | Separate preset lists per platform. More granular but complex. | |
| Blank list only | No presets, manual entry only. | |

**User's choice:** Quick-add button with common bots (Recommended)
**Notes:** None

---

## Claude's Discretion

- Exact UI layout and spacing of Filters section
- Tag/chip input component implementation
- Filter check ordering
- Invalid regex error handling
- Specific bot names in preset list

## Deferred Ideas

None — discussion stayed within phase scope
