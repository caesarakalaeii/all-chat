# ADR-0073: The KLIPY App Key Is Delivered at Runtime from the Cluster Secret

**Date**: 2026-10-17
**Status**: Accepted; who receives the key is restricted by ADR-0074 (2026-10-17)
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

ADR-0071 baked the KLIPY app key into the frontend image at build time
(`NEXT_PUBLIC_KLIPY_API_KEY` from the `KLIPY_API_KEY` Docker build arg, filled
by CI from a repository variable). The key is instead kept as a Kubernetes
secret (`allchat-secrets`, key `klipy-api-key`), and a runtime secret cannot
reach a `NEXT_PUBLIC_` variable, which Next.js inlines at build time. How does
the browser get the key?

## Decision Drivers

- KLIPY's integration requirements
  (https://docs.klipy.com/integration-requirements) require API requests and
  media loads to originate from the user's browser. Routing them through
  partner-operated servers or proxies needs KLIPY's prior written approval.
- The key is public by nature: it is a path segment of every search URL the
  browser sends.
- Rotating the key should not need an image rebuild.

## Considered Options

1. **A server route returns the key; the browser keeps calling KLIPY
   (chosen).** `GET /klipy-config` reads `process.env.KLIPY_API_KEY` per
   request (`force-dynamic`, `Cache-Control: no-store`) and answers
   `{apiKey}` (empty when unset).
2. A server-side search proxy that keeps the key off the browser. Forbidden by
   KLIPY without written approval.
3. Pass the key as a prop from a server component. The editor page is a client
   component, so the prop would thread through several layers for one value.

## Decision Outcome

Option 1. `frontend/src/lib/klipy.ts` fetches `/klipy-config` once per page
load (a failed fetch is retried on the next search) and calls
`api.klipy.com` directly as before. The route lives outside `/api/` because
the ingress sends `/api/*` to the api-gateway. The deployment sets
`KLIPY_API_KEY` from `secretKeyRef allchat-secrets/klipy-api-key`; beta first,
production when the alerts overlays are released there. No CSP change:
`/klipy-config` is same-origin (`connect-src 'self'`).

The same change applies KLIPY's one REQUIRED attribution item
(https://docs.klipy.com/attribution): the search placeholder reads
"Search KLIPY" in every locale, untranslated.

### Consequences

- Key rotation is a secret change plus a pod restart, no rebuild.
- The build arg, the CI repository variable and `NEXT_PUBLIC_KLIPY_API_KEY`
  are gone.
- Anyone can read the key from `/klipy-config`, exactly as anyone could read
  it from the search URLs or the old bundle. KLIPY's partner panel, not
  secrecy, limits its use.

## Related Decisions

- ADR-0071: the alerts editor and browser-side KLIPY search; this record
  replaces only its key delivery.
