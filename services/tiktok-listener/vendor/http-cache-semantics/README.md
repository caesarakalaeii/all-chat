# Vendored: http-cache-semantics

A patched copy of [`http-cache-semantics@4.2.0`](https://github.com/kornelski/http-cache-semantics)
(BSD-2-Clause; `index.js` and `LICENSE` are upstream verbatim except for the
patch described below). It is pinned into `tiktok-listener`'s dependency tree
because **every published version of the package is vulnerable** and there is
nothing to upgrade to.

## Why it is vendored

Advisory [GHSA-ch52-4w7c-c8xp](https://github.com/advisories/GHSA-ch52-4w7c-c8xp)
(high, CVSS 7.5, published 2026-09-18) covers `http-cache-semantics <= 4.2.0`
with `first_patched_version: null`. The chain that pulls it in:

    tiktok-live-connector -> got -> cacheable-request -> http-cache-semantics

cannot be walked out by version bumps: every `cacheable-request` release
(0.x through 13.x) depends on `http-cache-semantics`, every `got` release
(8.x through 15.x) depends on `cacheable-request`, and `tiktok-live-connector`
(current and all releases back to 2.1.1-beta1) depends on `got`. There is no
patched fork on npm worth trusting, and downgrading `tiktok-live-connector` to
2.1.0 (the only tree `npm audit fix --force` offers) breaks the listener.

The security gate (`security-scan.yml`, `npm audit --audit-level=high
--omit=dev`) fails on the advisory with no upgrade to take, so the fix lives
here until upstream publishes one.

## The bug and the patch

`maxAge()` deliberately zeroes the freshness of a shared-cache response that
carries `Set-Cookie` unless the response opts in with `public` or `immutable` —
a cookie'd response must not be reused without revalidation. But
`evaluateRequest()`'s max-stale shortcut served any stale entry to a client that
sent `Cache-Control: max-stale`, resurrecting that deliberately-zeroed entry:
one user's cached `Set-Cookie` answered another user's request.

The patch is two call sites plus one extracted predicate, all marked
`PATCH (GHSA-ch52-4w7c-c8xp)` in `index.js`:

- `_freshnessZeroedForSecurity()` names the condition `maxAge()` already used to
  zero freshness, and `maxAge()` now calls it, so the two cannot drift.
- The max-stale shortcut requires `!this._freshnessZeroedForSecurity()`, so a
  client directive can no longer override the zeroing. The entry is revalidated
  instead.

Deliberately untouched: `stale-while-revalidate` and `stale-if-error`. Those
are the *response's own* opt-in directives for stale reuse, not a client
directive overriding the cache's policy, which is what the advisory is about.

`test/max-stale-security.test.ts` pins both directions: the three GHSA cases
fail against pristine 4.2.0, and three guard cases fail if the patch is applied
too broadly. They run with the service's `npm test` (vitest).

## How it is wired

- `services/tiktok-listener/package.json` declares the package as a direct
  dependency (`"http-cache-semantics": "file:./vendor/http-cache-semantics"`)
  and pins every transitive reference to it with
  `"overrides": { "http-cache-semantics": "$http-cache-semantics" }`. The `$`
  reference is load-bearing: a bare `file:` in `overrides` resolves relative to
  the overridden package's parent directory and npm installs a dangling
  symlink while reporting success.
- `services/tiktok-listener/Dockerfile` copies `vendor/` before each `npm ci`,
  which needs the directory present.

## When to delete this directory

As soon as upstream publishes a fixed release (anything where
GHSA-ch52-4w7c-c8xp is resolved):

1. delete `vendor/`, the direct dependency and the `overrides` entry from
   `package.json`, and the two `COPY .../vendor/` lines from the Dockerfile;
2. run `npm install` to regenerate the lockfile;
3. run `npm audit --omit=dev` (expect 0) and `npm test` (the vendored suite goes
   away with the directory).

The `4.2.1` in `package.json` is *this* patched build's number, not an upstream
release. If upstream publishes a real 4.2.1, it does not affect this pin.

## License

`index.js` and `LICENSE` are upstream's BSD-2-Clause. `package.json`, this
README and `test/` are All-Chat code under the repository's AGPL-3.0.