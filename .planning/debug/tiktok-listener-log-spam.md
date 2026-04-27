---
status: verifying
trigger: "TikTok listener service spams logs with event notifications it shouldn't even receive"
created: 2026-03-26T00:00:00Z
updated: 2026-03-26T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED ROOT CAUSE - MigrationSubscriber receives ALL platform migration events (twitch, youtube, kick, tiktok) from Redis Pub/Sub. Each event is logged twice at info level before platform check: once in subscriber.ts ("Received migration event") and once in index.ts ("Processing migration event"). Non-TikTok events are logged then discarded.
test: Code trace: subscriber.ts handleMessage logs every event at info; index.ts handleMigrationEvent logs at info before if (event.platform !== 'tiktok') check
expecting: Fix by: 1) downgrade subscriber.ts log to debug, 2) move index.ts info log to after platform check
next_action: Implement fix in both files

## Symptoms

expected: TikTok listener should only log events relevant to chat messages (or at minimum, not spam the logs with irrelevant event notifications)
actual: TikTok listener floods logs with event notifications it shouldn't be receiving/logging
errors: Log spam from unwanted TikTok events
reproduction: Check tiktok-listener pod logs in the allchat namespace
started: Unknown — needs investigation

## Eliminated

- hypothesis: TikTok listener subscribes to all WebSocket events (MEMBER, ROOM_USER, etc.) and logs them
  evidence: Code in index.ts only subscribes to CHAT, GIFT, LIKE, FOLLOW, SOCIAL. Library has no built-in logger. Unhandled events are silently dropped by Node.js EventEmitter.
  timestamp: 2026-03-26

- hypothesis: like events cause spam because they fire too frequently
  evidence: handleLike uses logger.debug only; like aggregation publisher also uses logger.debug. Not visible at info level.
  timestamp: 2026-03-26

## Evidence

- timestamp: 2026-03-26
  checked: services/tiktok-listener/src/index.ts event subscriptions (lines 940-956)
  found: Service only subscribes to WebcastEvent.CHAT, GIFT, LIKE, FOLLOW, SOCIAL. No MEMBER, ROOM_USER, or other high-frequency events subscribed.
  implication: The spam is NOT from high-frequency TikTok stream events being logged — those aren't subscribed to.

- timestamp: 2026-03-26
  checked: tiktok-live-connector library client.js handleError method
  found: handleError only emits 'error' event if there's a listener registered. Temporary connections in status-checker.ts have no error listener so those errors are silently dropped.
  implication: Status check fallback errors do NOT cause log spam from the temp connection objects.

- timestamp: 2026-03-26
  checked: handleMigrationEvent (index.ts lines 648-750)
  found: logger.info('Processing migration event') fires BEFORE the platform !== 'tiktok' check. Every migration event from any platform (Twitch, Kick, YouTube) causes an info log in tiktok-listener.
  implication: If source-manager sends frequent migration events for non-TikTok platforms, tiktok-listener would spam "Processing migration event" logs.

- timestamp: 2026-03-26
  checked: status-checker.ts checkLiveStatus error handling (lines 122-139)
  found: When TikTok API blocks requests, logger.error('Failed to check live status') fires for every check attempt. Status checker creates temp TikTokLiveConnection for each check.
  implication: If TikTok is rate-limiting/blocking, error spam could be generated every 30 seconds per monitored channel.

- timestamp: 2026-03-26
  checked: tiktok-live-connector library events.js and client.js
  found: WebcastEventMap has 30+ event types. Library emits ControlEvent.DECODED_DATA for every message. processDecodedData handles mapping. Service only registers handlers for 5 events.
  implication: Unhandled events (MEMBER, ROOM_USER, HOURLY_RANK etc.) are silently dropped - no logging.

## Resolution

root_cause: The MigrationSubscriber in coordination/subscriber.ts subscribes to the Redis Pub/Sub channel `migration:events`, which receives migration events for ALL platforms (twitch, youtube, kick, tiktok) from the source-manager. Each received event was logged at info level in TWO places: (1) subscriber.ts `handleMessage` logged "Received migration event" at info, and (2) index.ts `handleMigrationEvent` logged "Processing migration event" at info BEFORE the `if (event.platform !== 'tiktok') return` guard. Every Twitch/YouTube/Kick migration generated 2 spurious info logs in the tiktok-listener before being discarded.

fix: 1) Changed subscriber.ts `handleMessage` log from logger.info to logger.debug with explanatory comment about the shared multi-platform channel. 2) In index.ts `handleMigrationEvent`, moved the platform check to the TOP of the function (before any info logging), so non-TikTok events are silently dropped at debug level. Renamed the info log to "Processing TikTok migration event" for clarity.

verification: TypeScript compilation passes (npx tsc --noEmit). Logic verified: TikTok events still get full info logging. Non-TikTok events are now only logged at debug level (invisible at default info log level).

files_changed:
  - services/tiktok-listener/src/coordination/subscriber.ts
  - services/tiktok-listener/src/index.ts

root_cause:
fix:
verification:
files_changed: []
