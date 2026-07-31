# ADR-0045: Auto-Provision a First Overlay + Default Source on First Sign-In (Activation Cliff)

**Date**: 2026-07-27
**Status**: Proposed
**Deciders**: caesarakalaeii

## Context and Problem Statement

The 2026-07-27 Umami analytics review (self-hosted, `analytics.allch.at`, ~50 days
of data) surfaced one dominant UX chokepoint: **most users who sign in never get
an overlay into their streaming software.** Measured as distinct sessions reaching
each activation stage (overlay OBS traffic excluded via the `overlay` tag):

| Stage | Sessions | % of sign-in |
|---|---|---|
| `signin_completed` | 587 | 100% |
| `overlay_created` | 223 | 38% |
| `source_added` | 280 | 48% |
| `obs_url_copied` (true activation) | 188 | **32%** |

**~68% of signed-in users never copy an OBS URL.** The guided onboarding shows the
same shape: of 152 sessions that start it, only ~60% reach the "copy OBS URL" step
and ~42% complete it — the final, most important step (the one that actually puts
the overlay on stream) leaks the hardest. Every step *before* the payoff sheds
users: create overlay → connect source → choose theme → copy OBS URL.

The root friction is that a brand-new user lands on an **empty dashboard** and must,
by hand: create an overlay, then start an OAuth reflow to add their own channel as
a source, then find and copy the OBS URL. But at the moment of first sign-in we
*already know who they are and which platform they just authed with* — the backend
holds the authenticated channel identity and credentials from the just-completed
OAuth. We are making the user re-assemble information the system already has.

## Decision Drivers

- **Shorten the path to the payoff.** The fewer steps between sign-in and a working
  OBS URL, the more users activate. The data shows each step is lossy.
- **Use what we already know.** First sign-in yields the authed channel + credentials;
  a default overlay that already streams *their own* chat is the obvious first artifact.
- **Don't produce dead-ends.** An auto-created overlay with **no** source is empty and
  worse than the current empty-state (it looks broken). Auto-create must include a
  working default source, or not happen at all.
- **Idempotency / no surprise duplicates.** Re-running the flow (repeat sign-ins,
  returning users, users who already have overlays) must never create extra overlays.
- **Preserve the onboarding guide.** The first-run guide (which already spotlights
  Theme/Sources/copy-OBS) should complement, not fight, auto-provisioning.
- **Measurable.** We must be able to tell whether this moves `obs_url_copied`.

## Considered Options

### 1. Backend auto-provisions a first overlay + default source on first sign-in (chosen, pending review)

On the **first** successful sign-in for a user with zero overlays, the backend
creates one overlay (named from the channel, e.g. "<display_name>'s Chat") and adds
a single source for the just-authed channel, using the channel id/credentials it
obtained during OAuth. The exchange response then directs the client straight to the
editor for that overlay, where the OBS URL and live preview are front-and-centre.

- **Pros**: shortest path to payoff (sign-in → editor with a working overlay + OBS URL,
  zero manual steps); the backend already holds the channel identity + creds, so no
  fragile client-side multi-call dance; atomic; naturally idempotent (guard on
  "user has zero overlays" / a `first_overlay_provisioned` marker).
- **Cons**: new backend behavior spanning auth-service (knows the OAuth result) and
  overlay-manager (owns overlay + source creation); must decide the seam (see below);
  must respect the existing `is_public_for_viewers = (first overlay)` default in
  `overlay-manager/handlers/overlay.go`.

### 2. Frontend auto-creates the overlay then adds the source (rejected)

After `signin_completed`, the client calls `POST /api/v1/overlays {name}` then
`POST /api/v1/overlays/{id}/sources {platform, channel_id}`.

- **Rejected because**: for real platform channels (twitch/youtube/kick) the frontend
  **never receives a ready-to-use `channel_id`** — the exchange response returns only
  the `User` object (`twitch_id`/`google_id`/`kick_id`), and sources for those platforms
  are normally added via the OAuth **add-source reflow** because the credentials come
  from OAuth. A client-built source POST would lack credentials and produce a
  non-working source. This is exactly the dead-end driver 3 forbids.

### 3. "Smart empty state" — one-click, pre-filled manual creation (fallback)

Keep manual creation but collapse it to a single primed button on the empty dashboard
("Create my <platform> chat overlay") that runs the create + add-source reflow for the
user in one action.

- **Pros**: smaller change; no silent auto-creation; keeps the user in control.
- **Cons**: still one deliberate click + an OAuth round-trip for the source; better than
  today but doesn't remove the reflow the way option 1 does. Reasonable interim step.

### 4. Do nothing structural; only improve onboarding copy (rejected as insufficient)

Make the copy-OBS step bigger/earlier in the guide.

- **Rejected as the primary fix**: worth doing regardless, but it does not address the
  structural cause (empty start state + manual re-assembly). The data shows the leak is
  broad (whole funnel), not just the guide.

## Decision Outcome

**Proposed**: **Option 1** — backend auto-provisions a first overlay + default source
(the just-authed channel) on first sign-in, and the client lands the user in the editor
with the OBS URL and live preview visible. **Option 3** is the acceptable fallback if the
backend seam proves too invasive for a first iteration.

This ADR is **Proposed, not Accepted** — it changes cross-service behavior (auth-service
↔ overlay-manager) and product behavior (auto-creating user content), so it needs the
owner's sign-off before implementation.

### Implementation sketch (to be refined at acceptance)

- **Seam**: preferred — auth-service, on first-time account creation, calls overlay-manager
  (service-to-service) to provision the default overlay + source using the channel identity
  and stored credentials from the OAuth it just completed. Alternative — overlay-manager
  exposes an idempotent `POST /api/v1/overlays/bootstrap` that the callback hits once.
- **Idempotency**: guard on `count(overlays for user) == 0` **and** a persistent
  `users.first_overlay_provisioned_at` marker so retries/races never duplicate.
- **Default-public interaction**: the auto-created overlay is the user's first, so the
  existing `is_public_for_viewers = (len(existing)==0)` rule makes it the default public
  overlay — intended, but call it out explicitly in the provisioning path.
- **Naming**: derive from `display_name`; never leak PII beyond what the user already sees.
- **Client**: when the exchange response carries the provisioned overlay id, redirect to
  `/overlays/{id}` (editor) instead of `/dashboard`; the onboarding guide adapts to
  "here's your overlay + OBS URL" rather than "create your first overlay".

### Measurement (already shipped)

The instrumentation added alongside this review makes the change measurable **before**
we build it, establishing the baseline and letting us confirm the intervention works:

- `editor_opened`, `overlay_settings_saved`, `source_configured` — editor engagement.
- `preview_rendered` — the aha-moment (first real message renders in the editor preview).
- `obs_url_copied` — the activation metric this ADR aims to lift.
- `source_add_failed` (now fired on the real add-source error redirects) — so a drop in
  activation can be told apart from an add-source failure spike.

Success criterion: a meaningful lift in `obs_url_copied` as a share of `signin_completed`,
and in `preview_rendered` per new user.

## Consequences

- **Positive**: removes the empty-start friction for the majority path (a streamer adding
  their own channel); turns first sign-in into an immediately useful, on-stream-ready
  overlay; the funnel becomes measurable end to end.
- **Negative / risks**: auto-creating user content must be reversible and unsurprising
  (the user can rename/delete); the auth→overlay-manager coupling must handle partial
  failure (overlay created, source failed) gracefully — provision atomically or clean up;
  multi-platform streamers still add their *other* platforms manually (acceptable — the
  default covers the just-authed one).
- **Follow-ups**: onboarding copy update (option 4) regardless; consider option 3's primed
  button for users who *skip* provisioning or arrive without a linkable channel.

## References

- Analytics review 2026-07-27 (Umami, `analytics.allch.at`; All-Chat website
  `c7a2e7ad-be45-4de3-954f-f15fd8e7dc97`). Funnel figures above.
- Instrumentation: `frontend/src/lib/analytics.ts`, `frontend/src/lib/analytics-auth.ts`,
  `frontend/src/hooks/useTrackOnce.ts`.
- Current creation paths: `services/overlay-manager/handlers/overlay.go` (`HandleCreateOverlay`),
  `services/overlay-manager/handlers/sources.go` (`HandleAddSource`),
  `frontend/src/app/auth/callback/page.tsx` (post-sign-in redirect),
  `frontend/src/app/dashboard/page.tsx` (empty-state + auto-start onboarding).
- Related: ADR-0042 (editor settings navigation), ADR-0008 (feature gates).
