# All-Chat Stream Deck plugin

Drive an All-Chat account from Elgato Stream Deck hardware keys: send preset chat
messages, and start, close, lock, resolve or cancel polls and predictions without
alt-tabbing out of a stream.

Built with the official [Stream Deck SDK for Node](https://github.com/elgatosf/streamdeck).
It is a self-contained subproject: its own `package.json` and lockfile, no
dependency on the Go services and no entry in the root `package.json`.

---

## The premium boundary, up front

**The plugin is free. Two of its buttons need All-Chat premium, and that is
deliberate.**

The server gates *starting* a poll or a prediction behind the premium
`engagement` feature. Everything that finishes or unwinds a round is free:

| Action                    | Route                                       | Premium? |
| ------------------------- | ------------------------------------------- | -------- |
| Send chat message         | `POST /api/v1/auth/chat/send`               | free     |
| Poll → **start**          | `POST …/engagement/overlays/:id/polls`      | **premium** |
| Poll → close              | `POST …/polls/:pollId/close`                | free     |
| Prediction → **start**    | `POST …/engagement/overlays/:id/predictions`| **premium** |
| Prediction → lock         | `POST …/predictions/:pid/lock`              | free     |
| Prediction → resolve      | `POST …/predictions/:pid/resolve`           | free     |
| Prediction → cancel       | `POST …/predictions/:pid/cancel`            | free     |

So on a free account, the two **start** keys return **HTTP 403** — and that is
the server working correctly, not the plugin failing. The plugin treats that 403
as its own distinct state: the key shows Stream Deck's alert and the plugin log
carries a specific message explaining that starting polls and predictions is a
premium feature, that the close/lock/resolve/cancel keys still work, and that you
can upgrade at <https://allch.at/upgrade>. It is never reported as a generic
"request failed".

The asymmetry is intentional in the product: if you started a round while
premium, or from the web app, you can always end it — a free account is never
left with a poll it cannot close.

The plugin does not attempt to work around the gate, and contains no server-side
code; the gate lives in `services/engagement-service`.

---

## Install

Requires Node 20.5.1 or newer and the Stream Deck app 6.5+.

```bash
cd streamdeck-plugin
npm ci
npm run build
```

`npm run build` compiles `src/` into
`com.allchat.streamdeck.sdPlugin/bin/plugin.js`. That `.sdPlugin` folder *is* the
plugin; to install it, copy or symlink it into the Stream Deck plugins directory
and restart the Stream Deck app:

- **macOS** — `~/Library/Application Support/com.elgato.StreamDeck/Plugins/`
- **Windows** — `%APPDATA%\Elgato\StreamDeck\Plugins\`

```bash
# macOS, developing against a live Stream Deck
ln -s "$PWD/com.allchat.streamdeck.sdPlugin" \
  ~/Library/Application\ Support/com.elgato.StreamDeck/Plugins/
```

With the [Elgato CLI](https://www.npmjs.com/package/@elgato/cli) installed you can
instead use `streamdeck link com.allchat.streamdeck.sdPlugin`, then
`streamdeck restart com.allchat.streamdeck`.

### Scripts

| Script          | What it does                                          |
| --------------- | ----------------------------------------------------- |
| `npm ci`        | Installs from the lockfile.                           |
| `npm run build` | Type-checks and compiles to `…sdPlugin/bin/`.         |
| `npm run lint`  | ESLint over `src/`, plus a `tsc --noEmit` type check.  |
| `npm run watch` | Recompiles on change, for plugin development.          |

> The build toolchain (`typescript`, `eslint`) is listed under `dependencies`
> rather than `devDependencies` on purpose: this package is private and never
> published, and CI runs with `NODE_ENV=production`, where `npm ci` drops dev
> dependencies and the build would fail with `tsc: not found`.

### Packaging

`.github/workflows/plugins.yml` lints, builds, validates the manifest and packs
this plugin on every PR that touches it, and uploads the resulting
`com.allchat.streamdeck.streamDeckPlugin` as a run artifact. That artifact is
how a reviewer installs a branch on real hardware without a local toolchain:
download it from the run page, unzip, double-click.

The same two commands work locally, with the Elgato CLI pinned to the version CI
uses:

```bash
npx --yes @elgato/cli@1.9.0 validate com.allchat.streamdeck.sdPlugin
npx --yes @elgato/cli@1.9.0 pack com.allchat.streamdeck.sdPlugin --output dist
```

> **Gotcha:** the Elgato CLI **rewrites `manifest.json` in place**, reformatting
> it and stripping the trailing newline. That is invisible in CI (the checkout is
> thrown away) but shows up as an unrelated diff locally, so
> `git checkout com.allchat.streamdeck.sdPlugin/manifest.json` after packing.

The plugin is not published to the Elgato Marketplace yet; the artifact is the
only distribution today.

---

## Where to mint a token

The plugin authenticates with an All-Chat **personal access token**, not a
browser login — a desktop app has no browser session to borrow, which is exactly
why these tokens exist (see ADR-0051).

1. Sign in at <https://allch.at>.
2. Go to **Settings → API tokens**
   (<https://allch.at/settings/api-tokens>).
3. Create a token. Give it the `chat:write` scope for the chat action and
   `engagement:write` for the poll and prediction actions.
4. **Copy it immediately** — All-Chat stores only a SHA-256 hash, so the
   plaintext is shown exactly once and cannot be recovered. If you lose it,
   revoke it and mint another.
5. Paste the whole string, including the `allchat_pat_` prefix, into the
   **Access token** field of each key's property inspector.

Every request then carries `Authorization: Bearer allchat_pat_…`.

### Token handling

- The token is stored by Stream Deck alongside the key's other settings.
- **The plugin never logs it.** It goes into one HTTP header; it is not written
  to the plugin log, not put in a URL, and not echoed into any error message.
- If a diagnostic ever needs to tell two configured keys apart, it prints
  `allchat_pat_…` plus the last four characters, never the secret.
- A token cannot delete your account, export your data, or reach any admin
  surface — those refuse personal access tokens outright.

### Self-hosting

The **Server** field defaults to `https://allch.at`, the production host. Leave
it empty unless you run your own All-Chat; then set it to your own base URL, with
no trailing slash.

---

## Actions

### Send Chat Message

Sends a fixed message to your chat on key-down. Configure the message text and a
target platform — `twitch`, `youtube`, `kick`, `tiktok`, or `all` to fan out to
every connected platform. Free on every account.

### Poll Control

One action, two modes:

- **Start a poll** *(premium)* — needs a question and 2–5 options, one per line.
  An optional duration auto-closes the poll; leave it at 0 to close it manually.
- **Close the live poll** *(free)* — leave the poll id blank and the key closes
  whichever poll is currently active on the overlay, so a single "close" key
  needs no upkeep.

Both modes need the **Overlay ID** — the UUID of the overlay, from your All-Chat
dashboard.

### Prediction Control

One action, four modes:

- **Start** *(premium)* — a title and 2–10 outcomes, one per line, with an
  optional auto-lock delay.
- **Lock** *(free)* — stops accepting wagers.
- **Resolve** *(free)* — pays out. Needs the **winning outcome's id**, not its
  label; copy it from the prediction in your dashboard.
- **Cancel** *(free)* — voids the prediction and refunds every stake.

Lock, resolve and cancel act on whichever prediction is live on the overlay
unless you pin a specific prediction id.

A premium streamer typically binds four keys, one per mode. A free account binds
three; the start key will keep explaining, informatively, what it would unlock.

---

## What the key tells you

A key press shows Stream Deck's tick on success and its alert on any failure. A
key cannot show two different glyphs, so the *meaning* lives in the plugin log,
and each case is specific:

| Situation                             | What you get                                                       |
| ------------------------------------- | ------------------------------------------------------------------ |
| Success                               | Tick.                                                              |
| **403** on a start key                | Alert, title hint `Premium`, and a log line naming the `engagement` feature, noting the free keys still work, and pointing at <https://allch.at>. |
| **401** — token rejected              | Alert, title hint `Token?`, and a log line telling you to mint a fresh token and re-paste it. Never the token itself. |
| No token / not an `allchat_pat_` / missing setting | Alert, title hint `Setup`, and a log line naming the field to fill in. |
| 403 elsewhere                         | Alert plus "you do not own that overlay, or the token lacks the scope". |
| 404 / 409 / 429                       | Alert plus the specific cause (missing overlay, a poll already running, rate limited). |
| Host unreachable                      | Alert plus the URL that failed.                                    |

Plugin logs live next to the plugin:

- **macOS** — `~/Library/Application Support/com.elgato.StreamDeck/Plugins/com.allchat.streamdeck.sdPlugin/logs/`
- **Windows** — `%APPDATA%\Elgato\StreamDeck\Plugins\com.allchat.streamdeck.sdPlugin\logs\`

---

## Layout

```
streamdeck-plugin/
├── src/
│   ├── plugin.ts                     entry point; registers the three actions
│   ├── actions/
│   │   ├── base.ts                   shared key plumbing; the 401 vs 403 branch
│   │   ├── send-message.ts           chat send
│   │   ├── poll-control.ts           start (premium) / close (free)
│   │   └── prediction-control.ts     start (premium) / lock / resolve / cancel
│   └── allchat/
│       ├── api.ts                    the routes, and which are premium-gated
│       ├── client.ts                 HTTP, bearer auth, status classification
│       ├── errors.ts                 the error taxonomy and its messages
│       └── settings.ts               per-key settings and their normalisation
└── com.allchat.streamdeck.sdPlugin/  the installable plugin
    ├── manifest.json
    ├── ui/                           property inspectors
    ├── imgs/                         key and category art
    └── bin/                          build output (git-ignored)
```

`src/allchat/api.ts` is the single place recording which routes the server gates;
`src/allchat/errors.ts` is the single place deciding what the user is told. If
the premium boundary ever moves, those two files are what change.

---

## Licence

AGPL-3.0-or-later, as with the rest of All-Chat.
