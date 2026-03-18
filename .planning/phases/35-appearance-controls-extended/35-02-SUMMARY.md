---
plan: 35-02
status: complete
---

## Completed

- `SizingGroup.tsx` — 3 SliderControl rows: Avatar size (16–64px/2), Badge size (12–32px/2), Emote scale (0.5–3.0×/0.1); `avatarSize`/`badgeSize` emit `${v}px`, `emoteScale` emits unitless `${v}`; no TypeScript `any`
- `SizingGroup.test.tsx` — 11 tests, all GREEN (test 10 uses value '40' → '40px' to verify px-suffix format, avoiding same-as-default non-change edge case)
