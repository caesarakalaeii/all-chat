# Credit Roll Themes

Custom CSS themes for the All-Chat end-of-stream credit roll: the leaderboards (subs, bits, raids, super chats, follows, gifts, memberships, coin chests, shares, likes), rankings, clip showcases and thank-you messages. Example `.css` files live next to this README (`classic-cinematic.css`, `neon-credits.css`, `minimal-clean.css`, `retro-gaming.css`, `elegant-script.css`, `gradient-wave.css`, `hollywood-blockbuster.css`) and are also browsable in the app at `frontend/src/app/overlays/[id]/credits/page.tsx` (editor at `/overlays/{id}/credits`).

## Selectors

The roll page renders Tailwind utility classes, a theme selects on them:

```css
.min-h-screen { }            /* page */
.container { }               /* the scrolling roll. It moves by an animated
                                transform, so a theme must not set `transform`
                                on it (an !important one stops the roll). */
h2 { }                       /* section headers */
.space-y-4 { }               /* leaderboard containers */
.space-y-4 > div { }         /* individual entries */

.bg-yellow-500\/20 { }       /* 1st place */
.bg-slate-400\/20 { }        /* 2nd place. Note it is slate, not gray */
.bg-orange-600\/20 { }       /* 3rd place */
.text-3xl.font-bold { }      /* rank numbers */
.text-xl.font-semibold { }   /* display names */
.text-sm.text-slate-400 { }  /* platform labels */
.text-2xl.font-bold { }      /* value/count display */
```

Examples:

```css
body { font-family: 'Your Font', sans-serif; }
.space-y-4 > div {
  background: rgba(your, colors, here, 0.2);
  border: 2px solid rgba(your, border, color, 0.5);
}
.bg-yellow-500\/20 { box-shadow: 0 0 30px rgba(255, 215, 0, 0.5); }
```

## Metadata

Include a header comment at the top of your CSS file: `Theme Name`, `Description`, `Tags`, `Author`, `Version`, `Updated`.

## Contributing

1. Create your theme CSS file with the metadata header.
2. Test it in the All-Chat credit roll preview.
3. Open a pull request adding it to `docs/credit-roll-themes/`.
4. Themes must be family-friendly and work when some leaderboards are empty.

## Notes for theme authors

- Respect the streamer's own settings: text content and text sizes are set in the credit roll editor, and a size set there is applied as an inline `font-size`. A theme that forces `font-size: ... !important` overrides it and makes those size sliders do nothing. The element classes (`.text-4xl.font-bold` and so on) stay on the roll for themes to select.
- Use web fonts via `@import`; provide fallbacks.
- Keep animations performant; background opacity should stay adjustable.
