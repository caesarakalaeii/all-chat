---
name: announce-feature
description: >-
  Draft and publish an All-Chat feature announcement to Patreon, then cross-post
  a hook + link to X and Bluesky. Use whenever the user wants to announce, post
  about, or "make a Patreon post" for a shipped feature, or cross-post a release
  to social. Covers release checklist step 3 (Patreon post). Drives the browser
  with Playwright. Keywords: patreon post, announce feature, release announcement,
  cross-post, X/Twitter, Bluesky, shipped feature.
---

# Announce a shipped feature (Patreon + X + Bluesky)

Repeatable workflow for turning a shipped All-Chat feature into a Patreon update
plus short cross-posts on X and Bluesky. This is **release checklist step 3**
(see the root `CLAUDE.md` "Shipping a Feature" section).

This is a **human-in-the-loop, browser-driven** task, not a batch job. Always get
the user to approve the copy before publishing — these posts are public and hard
to unpublish.

## Accounts (browser is expected to already be logged in)

- **Patreon** (creator): `https://patreon.com/all_chat`
- **X**: `@TrueCaesarLP` — compose at `https://x.com/compose/post`
- **Bluesky**: `@caes.ar` — `https://bsky.app`

If a site isn't logged in, stop and ask the user to log in (they can run an
interactive login via the `! <command>` prompt, or just sign in in the browser).
Never enter or handle their credentials yourself.

House-voice reference (private, gitignored): `docs/marketing/OUTREACH_PLAN.md`.

---

## Step 1 — Identify the feature and verify it actually shipped

"Recently shipped" / "shipped today" means **check dates and verify against
`origin/main`** — do not trust a stale planning folder.

```bash
git fetch origin --quiet
gh pr list --state merged --limit 20 \
  --json number,title,mergedAt --jq '.[] | "\(.number)\t\(.mergedAt[0:10])\t\(.title)"'
git log origin/main --since="<recent>" --pretty=format:"%h %cd %s" --date=short
```

- Match the user's description to the right PR by **merge date**. (Lesson learned:
  a phase folder named for a feature can be months old; the thing that "shipped
  today" is a different, recent PR.)
- **Read the actual merged code**, not just the PR body or planning docs, so the
  copy is accurate: `git show origin/main:<path>`. Describe what the feature
  really does and where it lives (e.g. the broadcast overlay vs. the in-app
  monitor at `/overlay/[id]/view`).

## Step 2 — Free or premium?

Check whether the feature is behind a premium gate (ADR-0008 `feature_gates`,
`shared/middleware` RequirePremium/RequireEarlyAccess). Every recent Patreon post
states this explicitly:

- Ungated → say **"free for everyone, no premium needed"**.
- Premium-gated → frame it as a Premium perk, and remember premium features also
  need the onboarding tour entry (`OnboardingChecklist.tsx` + `/upgrade`) as a
  separate release step. Ungated features skip that.

## Step 3 — Draft the copy (match house voice)

**Global rule: no em-dashes.** They are an AI-writing tell. Use commas/periods.

**Patreon post** (full update). Match the established voice of recent posts:
- Warm opener ("Hey everyone,"), plain language, concrete before/after.
- Call the moderation/monitor view **"the in-app monitor"**; call the OBS output
  **"the overlay"**.
- State free vs premium (Step 2).
- Standard footer, then thanks. Template:
  > As always, All-Chat merges Twitch, YouTube, Kick, TikTok and Discord into one
  > overlay feed for OBS, with full 7TV, BTTV and FFZ emote support. Try it free
  > at https://allch.at
  >
  > Thanks for the support.
- Audience = **Free access** for free features (matches the other free-feature posts).

**X hook** (`@TrueCaesarLP`) — short hook **+ the Patreon post link**, not the full
post. Keep visible text **under ~230 chars**; on X every URL counts as **23 chars**
regardless of length.

**Bluesky hook** (`@caes.ar`) — same hook + Patreon link. Limit **300 chars**;
Bluesky counts the **full URL length**. The composer's live counter shows chars
remaining — confirm it before posting.

Present all three drafts to the user and get approval before touching the browser.

## Step 4 — Publish with Playwright (Patreon FIRST)

Publish Patreon first so its URL can go into the cross-posts. Use
`mcp__playwright__*` tools (load via ToolSearch: `browser_navigate`,
`browser_snapshot`, `browser_click`, `browser_type`, `browser_find`,
`browser_evaluate`). Prefer `browser_find` / targeted `browser_snapshot(target)`
over full-page snapshots — the X and Bluesky trees are huge.

**Patreon**
1. `browser_navigate` to `https://www.patreon.com/all_chat`; confirm creator UI
   (a "Create post" button = logged in).
2. Click **Create post** → pick **Post** from the menu (not Quip/Product/Live).
3. In the editor: leave **Audience = Free access** (or set Paid for premium).
4. Fill **Title** field, then the **post content** field. The body editor turns
   `\n\n` into separate paragraphs — type the whole body at once.
5. Click **Publish** (it enables once there's content; it may re-render with a new
   ref — re-snapshot if a click fails with "ref not found").
6. From the share dialog, grab the **clean post URL** (strip UTM params), e.g.
   `https://www.patreon.com/all_chat/posts/<slug>-<id>`.

**X**
1. `browser_navigate` to `https://x.com/compose/post`.
2. Type the hook + Patreon URL into the **Post text** box; the URL becomes a link
   chip. Confirm the Post button enables (char budget OK).
3. Click **Post**; verify the "Your post was sent." toast and capture the status URL.

**Bluesky**
1. `browser_navigate` to `https://bsky.app`; click **Compose new post**.
2. Type hook + Patreon URL; confirm the remaining-char counter is >= 0.
3. Click **Publish post**; verify via `browser_find` on the profile
   (`/profile/caes.ar`) that the post is live.

## Step 5 — Report

Give the user the three live URLs (Patreon, X, Bluesky) and note free-vs-premium.
Do **not** commit anything for this task — it is marketing ops, kept out of the
public repo (this skill itself is gitignored).
