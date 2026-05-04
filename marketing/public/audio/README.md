# Audio assets

## bed.mp3

Background music bed for both `HeroShowcase` and `HeroShowcaseSocial`.

- **Source:** YouTube `jVTsD4UPT-k` (per the channel's owner, copyright-free)
- **Trim:** first 2:30 cut off (track intro/build); kept ~95 seconds of usable bed
- **Processing:** `afade` 0.6s fade-in + `loudnorm I=-22 TP=-2 LRA=8` for safe headroom
- **Format:** 48 kHz stereo MP3 q3

To re-fetch / regenerate (requires `yt-dlp` + `ffmpeg`):

```bash
yt-dlp -x --audio-format mp3 --audio-quality 0 \
  "https://www.youtube.com/watch?v=jVTsD4UPT-k" \
  -o /tmp/bed-raw.%(ext)s

ffmpeg -y -ss 150 -i /tmp/bed-raw.mp3 \
  -af "afade=t=in:st=0:d=0.6,loudnorm=I=-22:TP=-2:LRA=8" \
  -codec:a libmp3lame -q:a 3 \
  public/audio/bed.mp3
```

If swapping for a different track later: 95 s is more than enough for both
compositions (40 s + 20 s). The compositions use a Remotion `<Audio>` with a
`volume` function that holds at 0.6 then fades to 0 over the last 30 frames
(1 s) — so a longer/shorter bed both work as long as duration ≥ composition.
