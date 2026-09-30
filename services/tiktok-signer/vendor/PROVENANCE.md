# Vendored files

Both files are vendored from <https://github.com/carcabot/tiktok-signature>
(MIT licensed, `carcabot` and contributors), fetched 2026-09-15 from the
`master` branch. They are the only reverse-engineered pieces of this service:
everything else talks to TikTok exactly the way a browser does.

## `xgnarly.mjs`

The X-Gnarly parameter encoder: a self-contained ChaCha20 variant over a
fixed-order field payload, custom base64 alphabet. Upstream calls it with
`{ ubcode: 4, sdkVersion: "1.0.0.368" }` (see `src/signing/session.ts`).

## `webmssdk_5.1.3.js`

TikTok's own obfuscated web SDK (the `/webmssdk/` script their pages load),
saved locally so the signer does not depend on TikTok's CDN serving it on
demand. It is injected into the headless browser before navigation and asked
to compute X-Bogus for our URLs, which is what makes signing survive TikTok's
signature churn: the algorithm stays TikTok's code, and X-Gnarly is the one
parameter we compute ourselves.

Update both together, from the same upstream commit, if signatures start
failing: upstream's release history tracks X-Bogus/SDK breakage.
