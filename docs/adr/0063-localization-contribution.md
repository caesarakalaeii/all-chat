# ADR-0063: Beta-tester localization contribution with admin review and a repo-based catalog pipeline

**Date**: 2026-09-15
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

All-Chat's UI strings live in the typed ADR-0055 catalog (dotted keys checked
at compile time). Community members repeatedly offer to translate the
interface, and the beta-tester cohort (ADR-0020) is the natural first group.
Three constraints shape the tool:

- **Contributors are not developers**: no git, PRs, TypeScript or key
  syntax; the tool reads like a form.
- **The catalog is the source of truth**: a tool storing its own key list
  drifts within a week, and accepted translations must end up in the repo
  because the catalog's compile-time key check and no-provider runtime both
  assume code, not rows.
- **Nothing auto-commits**: a service pushing to the repo needs a credential
  and a blast radius no contributor feature justifies; every accepted
  translation travels through a reviewed PR.


## Decision

**A contributor tool at `/translate`, gated by the
`localization_contribution` feature gate (seeded early-access, migration
097), with a review-and-export pipeline owned by share-service.**

Four pieces:

1. **Guided translator view** (`/translate`): pick a language, pick an area of
   the app (a namespace, with per-area progress), translate string by string
   from a source table rendered from the local `enMessages` catalog, with
   placeholder hints and previous submission status visible. The key list is
   derived from the catalog in the frontend, never sent by the backend — a key
   the repo no longer has cannot appear in the tool.

2. **Open, request-based locales.** Any contributor proposes a locale
   (`localization_locales`, status `requested`), invisible to other
   contributors until an admin approves it. No fixed launch set: effort
   follows demand.

3. **Submit, then admin review.** One row per `(locale, key)` in
   `localization_translations`; resubmitting overwrites and resets to
   `pending` (no forking, no voting). Admins approve or request changes via
   `/admin/localization`. Placeholder parity against the English source is
   checked server-side and live in the client before submit, because the
   ADR-0055 runtime lookup never throws and would render a missing
   placeholder as a visible literal to viewers.

4. **Export, script, PR.** `GET /api/v1/admin/localization/export/:code`
   returns approved rows; `scripts/generate-locale-catalog.mjs` turns them
   into `messages/<locale>/<namespace>.ts` files in the English catalog's
   shape. The script does not wire the locale into `SUPPORTED_LOCALES` or the
   barrel (those edits are for the reviewer, named by the printed TODO), and
   keys absent from the export stay English, so a partial locale ships safely
   behind the fallback.

Ownership is share-service (it owns the beta-tester admin surface,
ADR-0020). Access control is the gate, not the route:
`RequireEarlyAccess('localization_contribution')` (beta testers and
ambassadors, ADR-0041), flippable to all authenticated users via the admin
feature-gates endpoint without a deploy; the frontend mirrors the check
purely as UX, the service remains the boundary. The `translate` namespace is
excluded from its own translation surface: a contributor would find the tool
changing under them mid-session.

## Rejected alternatives

- **Service opens the PR itself**: a service credential with repo write access
  for a contributor feature, and the review would rubber-stamp generated work.
  The script keeps the human in the loop where the repo changes.
- **Suggest + upvote**: translations are not opinions; there is one correct
  placeholder parity and few qualified reviewers. Voting rewards early
  submissions, not good ones.
- **Trust beta testers directly**: one bad translation ships to every viewer
  in that locale; review cost is one pass per string.
- **Store the key list in the DB**: it would drift from the catalog with every
  merged PR; the export/generator pair needs no reconciliation step only
  because it cannot need one.

Risks: approving a locale is a commitment (rejection deletes the request and
re-requesting is cheap); stale keys in approved rows surface at export time
(the generator reports unmatched keys); placeholder parity is enforced
against the client-supplied source, so the worst case of a lying client is a
bad DB row an admin must approve, never a broken catalog; migration 097
replays idempotently.

## Consequences

Beta testers gain `/translate` and an onboarding-tour entry; admins gain
`/admin/localization`. The first export produces catalog files plus a printed
TODO; `docs/frontend/I18N.md` "Adding another locale" carries the wiring
checklist. It ships behind a beta gate, not a premium gate: contribution is a
role behaviour, not a paid feature, so `/upgrade` does not list it. Out of
scope: request-time locale selection (ADR-0055), machine-translation drafts,
viewer chat content (never translated).

## References

- ADR-0055 (the catalog this feeds), ADR-0020/0041 (beta/ambassador roles),
  ADR-0008 (the `localization_contribution` gate)
- docs/frontend/I18N.md ("Adding another locale"), migration 097,
  scripts/generate-locale-catalog.mjs
