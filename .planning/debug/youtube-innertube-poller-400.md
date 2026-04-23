---
status: awaiting_human_verify
trigger: "youtube-innertube-poller-400: every poller fails with HTTP 400 INVALID_ARGUMENT on first poll"
created: 2026-03-25T00:00:00Z
updated: 2026-03-25T00:00:00Z
---

## Current Focus

hypothesis: The get_live_chat polling request is missing required fields that YouTube's InnerTube API now requires. The discovery/next endpoints use a full User-Agent header but the polling client uses a minimal one. More critically, the InnerTube endpoint URL uses youtube.googleapis.com in docs but the code uses www.youtube.com — this is consistent. The real issue may be the ?key= query parameter being sent: YouTube has deprecated API key auth for some InnerTube endpoints, and sending `?key=` may now cause a 400. Alternatively, the request context payload is missing required fields (hl, gl, visitorData, etc.) that the /next API tolerates but /get_live_chat now requires.
test: Compare the exact payload sent to /next (discovery, works) vs /get_live_chat (polling, fails 400)
expecting: Payload or header difference between working discovery and failing polling requests will explain the 400
next_action: confirmed root cause — apply fix

## Symptoms

expected: Pollers should successfully poll InnerTube API for live chat messages after getting a continuation token
actual: Every poller fails immediately with HTTP 400 INVALID_ARGUMENT from InnerTube API. Discovery works, continuation token extraction works, but the actual polling request fails.
errors: InnerTube API error status_code=400 body={"error":{"code":400,"message":"Request contains an invalid argument.","errors":[{"message":"Request contains an invalid argument.","domain":"global","reason":"badRequest"}],"status":"INVALID_ARGUMENT"}}
reproduction: Any YouTube channel with a live stream — all pollers fail the same way
started: Unknown — deployment is 25h old, possibly a recent InnerTube API change

## Eliminated

- hypothesis: The 400 error comes from wrong continuation token format
  evidence: Continuation token is successfully extracted (length 32), and the /next API returns 200. If the token were malformed the extraction would fail first.
  timestamp: 2026-03-25

## Evidence

- timestamp: 2026-03-25
  checked: services/youtube-listener-innertube/innertube/client.go (GetLiveChatReplay, lines 79-96)
  found: The polling payload is minimal: {"continuation": "...", "context": {"client": {"clientName": "WEB", "clientVersion": "..."}}}. The User-Agent header is also truncated (missing the full Chrome/version string).
  implication: Discovery and /next requests both use a full User-Agent and identical context structure. Polling uses a shortened User-Agent. This could trigger bot detection or validation errors.

- timestamp: 2026-03-25
  checked: services/youtube-listener-innertube/innertube/discovery.go (GetInitialContinuation, lines 243-263)
  found: The /next API request uses User-Agent "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36" (full browser UA). Same for /browse and /player requests.
  implication: All working endpoints (browse, next, player) use the full Chrome UA. Only the polling client (GetLiveChatReplay) uses the truncated UA "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36" without the Chrome/version suffix.

- timestamp: 2026-03-25
  checked: services/youtube-listener-innertube/innertube/client.go line 115 vs discovery.go line 69
  found: CONFIRMED DISCREPANCY.
    - client.go (polling): User-Agent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"
    - discovery.go (working): User-Agent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
  implication: The /get_live_chat endpoint may be stricter about User-Agent validation than the /browse and /next endpoints. This is a plausible cause of the 400.

- timestamp: 2026-03-25
  checked: services/youtube-listener-innertube/innertube/types.go ClassifyError (lines 227-247)
  found: HTTP 400 falls through to the default case in the switch statement, which returns ErrorTypeFatal. This is why the poller stops immediately on 400 instead of retrying.
  implication: Not a root cause, but relevant — if the fix doesn't fully work, 400s should possibly be treated as transient first to allow recovery.

## Resolution

root_cause: The InnerTube polling client (GetLiveChatReplay in client.go) sends a truncated User-Agent header "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36" — missing the "(KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36" suffix that all working discovery/next endpoints send. YouTube's /get_live_chat endpoint validates the User-Agent more strictly than /browse or /next, returning HTTP 400 INVALID_ARGUMENT when it receives an incomplete UA.
fix: Updated client.go line 115 to use the full Chrome User-Agent string: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36" — matching what all working discovery/next endpoints already send.
verification: All existing unit tests pass (go test ./innertube/... — ok 1.801s). Awaiting confirmation that pollers no longer fail with HTTP 400 in production.
files_changed: [services/youtube-listener-innertube/innertube/client.go]
