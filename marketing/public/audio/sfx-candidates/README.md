# SFX candidates — audition and pick

7 short SFX synthesised with ffmpeg. Pick favourites, name them, and I'll wire
into the relevant scenes.

## Transition whooshes (for scene boundaries)

| File             | Character                         | Best fit                       |
| ---------------- | --------------------------------- | ------------------------------ |
| whoosh-soft.mp3  | low brown noise, ~600ms, gentle   | calm transitions               |
| whoosh-tight.mp3 | pink noise, ~350ms, percussive    | quick punchy cuts              |
| whoosh-airy.mp3  | bandpassed white, ~800ms, breezy  | airy "open up" moments         |

## Message-arrival pops (for each chat row reveal)

| File             | Character                  | Best fit                         |
| ---------------- | -------------------------- | -------------------------------- |
| pop-bright.mp3   | 1500 Hz sine, 120ms decay  | crisp high-energy chat           |
| pop-warm.mp3     | 600 Hz sine, 160ms decay   | mellow, less attention-stealing  |
| pop-click.mp3    | 2500 Hz sine, 60ms decay   | dry click, almost subliminal     |

## CTA hit (for Outro)

| File             | Character                  | Best fit                         |
| ---------------- | -------------------------- | -------------------------------- |
| hit-deep.mp3     | 85 Hz sine, 500ms decay    | deep impact under "allch.at" CTA |

## Audition

```bash
mpv public/audio/sfx-candidates/whoosh-soft.mp3
# or
ffplay -nodisp -autoexit public/audio/sfx-candidates/pop-warm.mp3
```

## How to wire one in

Once you pick favourites, tell me which file goes where (e.g. "whoosh-tight
for transitions, pop-warm for messages, hit-deep for outro") and I'll add
`<Audio src=...>` placements with appropriate timing inside the relevant
Sequences.

These are synthesised — fine for utility but not gold-mastered. If anything
sounds too "clean / cheap", the upgrade path is to swap any of the picked
files with a curated equivalent from Freesound.org / Pixabay SFX (CC0 ideal).
