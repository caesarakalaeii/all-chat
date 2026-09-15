# ADR-0063: Beta-tester localization contribution with admin review and a repo-based catalog pipeline

**Date**: 2026-09-15
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

All-Chat's UI strings live in a typed catalog (ADR-0055): English only, one
file per namespace under `frontend/src/lib/i18n/messages/en/`, dotted keys
checked at compile time. Community members repeatedly offer to translate the
interface, and the beta-tester cohort (ADR-0020) is the natural first group:
already granted early access, already invested in the product's direction.

Three constraints shape any tool that accepts their help:

- **Contributors are not developers.** The tool must not expose git, pull
  requests, TypeScript or key syntax. It has to read like a form: source
  string on top, one input below.
- **The catalog is the source of truth, and it must stay so.** The English
  key set lives in the repo and changes with every PR. A translation tool
  that stores its own key list drifts within a week. Translations themselves
  must also end up in the repo, because the catalog's compile-time key check
  and the "no provider, no context" runtime both assume code, not rows.
- **Nothing auto-commits.** A service that pushes to the repo needs a
  credential and a blast radius no contributor feature justifies. An
  accepted translation still travels through a reviewed PR.

The open questions — which locales, what happens before a translation goes
live, and how accepted work reaches the repo — were answered by the operator
in the design interview (2026-09-15): locales are open and request-based;
submissions go through admin review; approved rows are exported and turned
into catalog files by a script, with the PR as the final gate.

## Decision

**A contributor tool at `/translate`, gated by the `localization_contribution`
feature gate (seeded early-access, migration 097), with a review-and-export
pipeline owned by share-service.**

Four pieces:

1. **Guided translator view** (`/translate`, `translate` catalog namespace).
   Pick a language, pick an area of the app (namespace, with per-area
   progress), then translate string by string: the English source rendered
   from the local `enMessages` catalog, one input per string, placeholder
   hints inline, previous submission status (pending / approved / changes
   requested with reviewer note) visible. The key list is derived from the
   catalog in the frontend, never sent by the backend — a key the repo no
   longer has cannot appear in the tool. Submissions batch per area on
   explicit submit; approved rows are read-only.

2. **Open, request-based locales.** Any contributor can propose a locale
   (`localization_locales`, status `requested`); requests are invisible to
   other contributors until an admin approves them (`approved`). No
   fixed launch set: the cohort proposes, the operator approves, effort
   follows demand.

3. **Submit → admin review.** One row per `(locale, key)` in
   `localization_translations`: resubmitting overwrites and resets to
   `pending`, so there is no forking or voting, just a current value with
   review state. Admins approve (or request changes with a note the
   contributor sees) via `/admin/localization`. The service validates key
   shape and placeholder parity (`{name}` must match the English source)
   server-side; the client checks the same parity live so a dropped
   placeholder is caught before submit, because the runtime lookup
   deliberately never throws (ADR-0055 overlay hardening) — a missing
   placeholder would otherwise render as a visible literal to viewers.

4. **Export → script → PR.** `GET /api/v1/admin/localization/export/:code`
   returns approved rows as JSON; `scripts/generate-locale-catalog.mjs`
   turns that into `messages/<locale>/<namespace>.ts` files in the same
   shape as the English catalog (nested, `as const`). The script does not
   wire the locale into `SUPPORTED_LOCALES` or the barrel — those are
   one-line edits a reviewer should see, and the printed TODO names them.
   Keys absent from the export stay English, so a partial locale ships
   safely behind the English fallback.

**Ownership: share-service.** It already owns the beta-tester admin surface
(ADR-0020) and the role/entitlement patterns; the tool is role-gated user
data with an admin half, which is its shape.

**Access control: the gate, not the route.** Contributor routes sit behind
`shared/middleware.RequireEarlyAccess('localization_contribution')`
(beta testers and ambassadors, ADR-0020/0041). Flipping `early_access` to
FALSE via the admin feature-gates endpoint opens contribution to all
authenticated users with no deploy. The frontend mirrors the role check in
`ProtectedRoute requireBetaTester` purely as UX — every fetch would 403
otherwise; the service remains the boundary.

**The `translate` namespace is excluded from its own translation surface.**
The tool's own strings are the strings a contributor reads while learning the
tool; translating them first would change the tool under the contributor's
hands mid-session.

## Rejected alternatives

- **Service opens the PR itself (GitHub API commits).** Rejected in the
  interview: a credential plus write access in a service for a contributor
  feature, and the PR review would rubber-stamp whatever the service
  generated. The script keeps the human in the loop at the exact point
  where the repo changes.
- **Suggest + upvote.** Community-driven, but translations are not
  opinions: there is one correct placeholder parity and a small set of
  qualified reviewers. Voting rewards early submissions, not good ones.
- **Trust beta testers directly.** A single bad translation ships to every
  viewer in that locale. Review cost is one pass per string.
- **Store the key list in the DB.** Would drift from the catalog with every
  merged PR that adds or renames a key; the export/generator pair would then
  need a reconciliation step that the current design does not have because
  it cannot need it.

## Risks and Operator Actions

- **Approving a locale is a commitment.** Once approved, a locale shows for
  all contributors; an admin approving `de` with no German-speaking
  contributors gets an empty locale. Rejection deletes the request —
  re-requesting is cheap.
- **Stale keys in approved rows.** A key renamed in the catalog stays
  translated in the DB until the export drops it. The generator reports
  unmatched keys, so the drift surfaces at export time, not in production.
- **Placeholder parity is enforced against the client-supplied source.** A
  hostile client can lie about `source_value`; the catalog itself remains
  the authority at generation time, so the worst case is a bad DB row an
  admin would have to approve, not a broken catalog.
- **Migration 097 replays idempotently** (CREATE IF NOT EXISTS, ON CONFLICT
  DO NOTHING), matching the runner's restart behavior.

## Consequences

- Beta testers gain `/translate` in the app nav and an onboarding-tour
  entry; admins gain `/admin/localization` (review queue, locale requests,
  export) in the admin sidebar.
- The first export for a locale produces catalog files plus a small
  checklist (barrel, `SUPPORTING_LOCALES`, index mapping) — the wiring that
  ADR-0055's "Adding locale #2" section already documents.
- The feature ships behind a beta gate, not a premium gate: contribution is
  a role behavior, not a paid feature, so `/upgrade` does not list it.
- Out of scope, explicitly: locale selection at request time (ADR-0055
  already requires its own decision), machine-translation drafts, viewer
  chat content (never translated).

## References

- ADR-0055 (the typed catalog this tool feeds), ADR-0020 (beta-tester role
  and early-access gates), ADR-0041 (ambassadors hold beta capability),
  ADR-0008 (feature gates seeded by migration 097)
- docs/frontend/I18N.md (the "Adding locale #2" checklist the generator's
  TODO mirrors)
- migration 097 (tables + gate), scripts/generate-locale-catalog.mjs
  (export → catalog files)
