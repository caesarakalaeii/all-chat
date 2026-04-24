---
status: partial
phase: 13-text-to-speech-tts-for-chat-messages
source: [13-03-PLAN.md]
started: 2026-04-24
updated: 2026-04-24
---

## Current Test

[awaiting human testing]

## Tests

### 1. ElevenLabs real-key happy path
expected: Save real ElevenLabs key -> Test-Key -> hear audible sample, quota displayed (format `8,432 / 10,000 characters this month (84%)`)
result: [pending]

### 2. Copy OBS URL -> paste in OBS -> send chat -> hear ElevenLabs voice
expected: OBS browser source renders overlay with audible TTS via ElevenLabs voice
result: [pending]

### 3. Regenerate OBS URL invalidates prior URL
expected: Old URL returns 401 in POST /tts; new URL works
result: [pending]

### 4. ElevenLabs failure triggers session-wide Web Speech fallback + one-time toast
expected: Invalid key at runtime -> toast "ElevenLabs unavailable — using browser voice." -> subsequent messages use Web Speech
result: [pending]

### 5. Remove key clears row + revokes access
expected: DELETE /tts-config -> POST /tts returns 404/401 -> UI shows "no key saved" state
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
