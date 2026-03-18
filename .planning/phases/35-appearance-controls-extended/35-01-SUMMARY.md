---
plan: 35-01
status: complete
---

## Completed

- `ToggleSwitch.tsx` — inline toggle primitive, role=switch, aria-checked, sliding thumb, ~30 lines, no external deps
- `VisibilityGroup.tsx` — 6 labeled toggle rows (Show avatars, badges, timestamps, platform badge, emotes, username); `isVisible` + `toDisplayValue` helpers; `visibilityDefaults` prop; showTimestamps emits 'block' on, others emit 'inline'; no TypeScript `any`
- `VisibilityGroup.test.tsx` — 8 tests, all GREEN
- `page.tsx` — `iframeVisibilityDefaults` state added; `handleIframeReady` queries 6 `--chat-show-*` CSS vars from iframe computed styles on ready; `visibilityDefaults={iframeVisibilityDefaults}` passed to AppearancePanel (wired in 35-03)
