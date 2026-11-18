# ADR-0045: Auto-Provision a First Overlay + Default Source on First Sign-In (Activation Cliff)

**Date**: 2026-07-27
**Status**: Proposed
**Deciders**: caesarakalaeii

## Context

The 2026-07-27 Umami analytics review (~50 days of data, `analytics.allch.at`, overlay traffic excluded) surfaced one dominant chokepoint: most users who sign in never get an overlay into their streaming software. Distinct sessions reaching each stage: `signin_completed` 587 (100%), `overlay_created` 223 (38%), `source_added` 280 (48%), `obs_url_copied` 188 (**32%**). Guided onboarding shows the same shape: of sessions that start it, ~60% reach the copy-OBS step and ~42% complete it. Every step before the payoff sheds users.

The root friction is that a new user lands on an empty dashboard and must, by hand: create an overlay, then run an OAuth reflow to add their own channel as a source, then find and copy the OBS URL. At first sign-in the backend already holds the authenticated channel identity and credentials — the user is made to re-assemble what the system has.

## Decision

**Proposed, not accepted**: backend auto-provisions a first overlay plus a default source (the just-authed channel) on first sign-in, and the client lands the user in the editor with the OBS URL and live preview visible. Because this changes cross-service behaviour (auth-service to overlay-manager) and auto-creates user content, it needs the owner's sign-off before implementation. Fallback if the seam proves too invasive: a "smart empty state" — one primed dashboard button ("Create my \<platform\> chat overlay") running the create + add-source reflow.

Rejected options: the frontend creating the overlay then the source itself (for real platform channels the exchange response returns only the `User` object, never a ready-to-use `channel_id` — a client-built source would lack credentials and be dead, exactly the failure mode to avoid); onboarding-copy improvements alone (worth doing regardless, but the leak is the whole funnel, not just the guide).

Implementation sketch (to refine at acceptance):

- **Seam**: preferred, auth-service on first-time account creation calls overlay-manager (service-to-service) to provision the overlay + source from the OAuth identity and stored credentials; alternative, an idempotent `POST /api/v1/overlays/bootstrap` hit once from the callback.
- **Idempotency**: guard on zero overlays for the user plus a persistent `users.first_overlay_provisioned_at` marker, so retries and races never duplicate.
- **Default-public interaction**: the auto-created overlay is the user's first, so the existing `is_public_for_viewers = (first overlay)` rule makes it the default public overlay — intended, but called out explicitly in the provisioning path.
- **Naming**: derived from `display_name`; no PII beyond what the user already sees.
- **Partial failure**: provision atomically or clean up (overlay created, source failed must not strand an empty overlay).
- **Client**: when the exchange response carries the provisioned overlay id, redirect to `/overlays/{id}` instead of `/dashboard`; the onboarding guide becomes "here's your overlay + OBS URL". Multi-platform streamers still add other platforms manually.

Measurement is already shipped, so a baseline exists before any build: `editor_opened`, `overlay_settings_saved`, `source_configured`, `preview_rendered`, `obs_url_copied`, and `source_add_failed` on real error redirects (`frontend/src/lib/analytics*.ts`, `useTrackOnce`). Success criterion: a meaningful lift in `obs_url_copied` as a share of `signin_completed` and in `preview_rendered` per new user.

## Consequences

- Removes the empty-start friction for the majority path (a streamer adding their own channel) and makes the funnel measurable end to end.
- Risks: auto-created content must be reversible and unsurprising; the auth→overlay-manager coupling must handle partial failure gracefully.

## References

- Analytics review 2026-07-27 (Umami site id `c7a2e7ad-be45-4de3-954f-f15fd8e7dc97`).
- Current creation paths: `services/overlay-manager/handlers/overlay.go`, `services/overlay-manager/handlers/sources.go`, `frontend/src/app/auth/callback/page.tsx`, `frontend/src/app/dashboard/page.tsx`.
