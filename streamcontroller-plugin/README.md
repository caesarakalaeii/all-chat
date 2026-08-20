# All-Chat StreamController plugin (Linux)

Drive an All-Chat account from Stream Deck hardware on Linux: send a preset chat
message to every connected platform at once, and start, close, lock, resolve or
cancel polls and predictions without alt-tabbing out of a stream.

This is a plugin for **[StreamController]**, the open-source Linux Stream Deck
application. StreamController has its own **Python** plugin API and does *not*
load Elgato plugins, which is why this exists separately from
[`../streamdeck-plugin/`](../streamdeck-plugin) — that one is TypeScript against
Elgato's Node SDK, for Windows and macOS. The two are deliberate mirrors of each
other: same actions, same endpoints, same failure states. ADR-0049 records the
reasoning, and every action module here carries a `KEEP IN SYNC` pointer at its
counterpart.

Self-contained: standard library only, no entry in the root `package.json`, no
Go module, and nothing under `services/` is touched.

[StreamController]: https://github.com/StreamController/StreamController

---

## The premium boundary, up front

**The plugin is free. Two of its buttons need All-Chat premium, and that is
deliberate product behaviour, not a bug.**

The server gates *starting* a poll or a prediction behind the premium
`engagement` feature. Everything that finishes or unwinds a round is free:

| Action                 | Route                                          | Premium?    |
| ---------------------- | ---------------------------------------------- | ----------- |
| Send chat message      | `POST /api/v1/auth/chat/send`                  | free        |
| Poll → **start**       | `POST …/engagement/overlays/:id/polls`         | **premium** |
| Poll → close           | `POST …/polls/:pollId/close`                   | free        |
| Prediction → **start** | `POST …/engagement/overlays/:id/predictions`   | **premium** |
| Prediction → lock      | `POST …/predictions/:pid/lock`                 | free        |
| Prediction → resolve   | `POST …/predictions/:pid/resolve`              | free        |
| Prediction → cancel    | `POST …/predictions/:pid/cancel`               | free        |

So on a free account the two **start** keys return **HTTP 403** — and that is
the server working correctly, not the plugin failing. The plugin treats that 403
as its own state: the key shows **⭐ Premium**, and the log carries a specific
message saying that starting polls and predictions is a premium feature, that
the close/lock/resolve/cancel keys still work, and that you can upgrade at
<https://allch.at/upgrade>. It is never reported as a generic "request failed",
and it is logged at INFO rather than ERROR, because nothing went wrong.

The asymmetry is intentional: if you started a round while premium, or from the
web dashboard, you can always end it. A free account is never left holding a
poll it cannot close, or a prediction whose stakes it cannot refund.

The plugin does not attempt to work around the gate and contains no server-side
code; the gate lives in `services/engagement-service`.

### Three different things return 403 — the plugin tells them apart

Sending a user to the billing page when their real problem is a mis-scoped token
would be worse than a generic error, so the plugin branches on the server's own
`error` field:

| Server says                  | Key shows | What actually fixes it                      |
| ---------------------------- | --------- | ------------------------------------------- |
| `Premium feature required`   | ⭐ Premium | Upgrade the account.                        |
| `insufficient token scope`   | ⚠ Error   | Mint a **new token with the right scope**. Upgrading would *not* help. |
| anything else (e.g. an overlay you do not own) | ⚠ Error | Check the overlay ID.       |

**HTTP 401** is separate again: the token is expired, revoked or mistyped — mint
a fresh one and re-paste it. It never points at the upgrade page.

---

## Install

Requires StreamController 1.5.0-beta.8 or newer, and Python 3.11+ (whatever
StreamController itself runs on — the plugin adds no interpreter requirement of
its own).

The plugin folder must be named after the plugin ID, `com_allchat_streamcontroller`.

**Flatpak install of StreamController** (the usual case):

```bash
git clone https://github.com/caesarakalaeii/all-chat.git
mkdir -p ~/.var/app/com.core447.StreamController/data/plugins
cp -r all-chat/streamcontroller-plugin \
  ~/.var/app/com.core447.StreamController/data/plugins/com_allchat_streamcontroller
```

**Native / source install:**

```bash
mkdir -p ~/.local/share/StreamController/plugins
cp -r all-chat/streamcontroller-plugin \
  ~/.local/share/StreamController/plugins/com_allchat_streamcontroller
```

Then restart StreamController. The three actions appear in the action picker
under **All-Chat**.

Developing against a checkout? Symlink instead of copying, so edits land without
re-copying — StreamController re-imports plugins on restart:

```bash
ln -s "$PWD/streamcontroller-plugin" \
  ~/.var/app/com.core447.StreamController/data/plugins/com_allchat_streamcontroller
```

### Dependencies

There are none. [`requirements.txt`](requirements.txt) is intentionally empty of
packages and explains why: a StreamController plugin is imported into the host
application's own Python process, so every dependency is a chance to break an app
we do not control. The plugin uses `urllib.request`, `json` and `urllib.parse`
from the standard library.

You do **not** need to `pip install` anything.

---

## Where to mint a token

The plugin authenticates with an All-Chat **personal access token** (PAT) — a
long-lived, scoped, individually revocable credential, introduced in ADR-0051.
It is not your session cookie and not a JWT.

1. Sign in at <https://allch.at>.
2. Go to **Settings → API tokens** (<https://allch.at/settings/api-tokens>).
3. Create a token, give it a name that identifies the machine (`stream-pc-deck`),
   and grant the scopes you need:
   - **`chat:write`** — for the *Send chat message* action.
   - **`engagement:write`** — for the *Poll control* and *Prediction control*
     actions.
4. Copy the token. It starts with **`allchat_pat_`** and **is shown once** — only
   a SHA-256 of it is stored server-side, so it cannot be shown again.
5. Paste it into the **API token** field of each All-Chat key.

Lost it, or pasted it on stream by accident? Revoke it on the same page and mint
a replacement; nothing else about your account is affected.

### Token handling in this plugin

- The token is sent only as `Authorization: Bearer allchat_pat_…`. It is never
  placed in a URL and never written to a log line — there is a test that asserts
  this for every failure path.
- It is stored by StreamController in that key's settings, per key, so one deck
  can drive two accounts.
- The plugin checks the `allchat_pat_` prefix locally, purely to catch a pasted
  session JWT before a pointless round-trip. Only the server can say whether a
  token is actually valid.
- Grant the narrowest scopes that work. A chat-send key does not need
  `engagement:write`.

---

## The actions

All three share two settings: **API token**, and an optional **Base URL** for
self-hosters (blank means `https://allch.at`, which is the production host,
confirmed as `FRONTEND_URL` in `deployments/k8s/base/configmap.yaml`).

### Send chat message

| Setting    | Meaning                                                         |
| ---------- | --------------------------------------------------------------- |
| `message`  | The text to send. Max 500 characters.                           |
| `platform` | `all` (default), `twitch`, `youtube`, `kick` or `tiktok`.       |

`all` fans the message out to every connected platform in one press — the reason
the action exists. Free; needs `chat:write`.

### Poll control

| Setting            | Meaning                                                    |
| ------------------ | ---------------------------------------------------------- |
| `mode`             | `start` (**premium**) or `close` (free).                   |
| `overlay_id`       | The overlay the poll belongs to. Required.                 |
| `question`         | `start` only.                                              |
| `options`          | `start` only. 2–5, one per line (or comma-separated).      |
| `duration_seconds` | `start` only. Blank/0 = run until closed by hand.          |
| `poll_id`          | `close` only, optional — see below.                        |

Leave `poll_id` blank on a close key. The plugin remembers the ID of a poll it
started, and otherwise asks the overlay for its active poll, so the usual setup
is a start key and a close key that both carry only the overlay ID.

### Prediction control

| Setting              | Meaning                                                          |
| -------------------- | ---------------------------------------------------------------- |
| `mode`               | `start` (**premium**), or `lock` / `resolve` / `cancel` (free).  |
| `overlay_id`         | Required.                                                        |
| `title`              | `start` only.                                                    |
| `outcomes`           | `start` only. 2–10, one per line.                                |
| `auto_lock_seconds`  | `start` only. Blank/0 = lock by hand.                            |
| `prediction_id`      | Optional on the free modes — same fallback as polls.             |
| `winning_outcome_id` | **Required by `resolve`.**                                       |

For `resolve`, the usual layout is one key per possible outcome, each with that
outcome's ID, so declaring a winner is a single press.

---

## Development

The plugin imports StreamController's `PluginBase`, `ActionBase` and
`ActionHolder` at runtime. Those live inside the host application and are not on
PyPI, so [`actions/host.py`](actions/host.py) imports them defensively and falls
back to small stand-ins when they are absent. That is what lets everything below
run on a machine with no Stream Deck attached.

```bash
cd streamcontroller-plugin

# Syntax gate — the same check CI runs.
python3 -m compileall -q .

# Unit tests: the premium/scope/401 split, every route as a literal string,
# input validation, and that no failure path can log a token.
python3 -m unittest discover -s tests -t .
```

`nix develop` provides the `python3` these use.

The tests stub `urllib.request.urlopen`, so they never touch the network and need
no All-Chat account. What they mainly protect is the premium boundary: there is a
test asserting that **exactly one** error kind maps to the premium key state, so
a future refactor cannot quietly turn the feature's only advertisement into a
generic error, nor start telling mis-scoped users to buy something that will not
help them.

### Packaging

`.github/workflows/plugins.yml` runs the two commands above on every PR that
touches this directory, validates `manifest.json` / `about.json` /
`attribution.json` (including that the two versions and ids agree, and that the
licence is still `AGPL-3.0-or-later`), and uploads an installable zip as a run
artifact. Unzip it into the plugins directory named under
[Install](#install) to test a branch.

The zip is a staged **allowlist**, not the whole directory: `main.py`, the three
JSON files, `requirements.txt`, `README.md`, `actions/` and `allchat/`. `tests/`
is deliberately left out — it is mock plumbing that a user's StreamController
would import-scan for nothing.

Not submitted to the StreamController plugin store yet; see
[Before submitting to StreamController's plugin store](#before-submitting-to-streamcontrollers-plugin-store).

### Layout

```
streamcontroller-plugin/
├── main.py                  # PluginBase subclass; registers three ActionHolders
├── manifest.json            # plugin ID, version, host compatibility
├── about.json               # author, licence, links
├── attribution.json         # third-party credits + the GPL/AGPL release gate
├── requirements.txt         # intentionally empty; explains why
├── allchat/                 # host-independent client (no StreamController imports)
│   ├── settings.py          #   defaults, validation, redaction
│   ├── errors.py            #   error taxonomy, incl. the three-way 403 split
│   ├── client.py            #   the one urllib request function
│   └── api.py               #   endpoints, with the premium flag beside each path
├── actions/
│   ├── host.py              #   real host classes, or offline stand-ins
│   ├── base.py              #   error kind -> key state
│   ├── send_message.py
│   ├── poll_control.py
│   └── prediction_control.py
└── tests/
```

`allchat/` deliberately imports nothing from StreamController, so the interesting
logic is testable in isolation.

### Before submitting to StreamController's plugin store

ADR-0049 records the licence question as a **release gate**, not a follow-up:
this plugin is loaded into StreamController's GPL-3.0 process, while All-Chat is
AGPL-3.0-or-later. No conflict is expected, but it has not been formally signed
off — see `attribution.json`. Settle it before the first submission.

---

## Troubleshooting

| Key shows        | Meaning                                                                    |
| ---------------- | -------------------------------------------------------------------------- |
| **⭐ Premium**    | Expected on a free account when starting a poll/prediction. Everything else still works. Upgrade at <https://allch.at/upgrade>. |
| **⚠ Error**, log says *token scope* | The token lacks `chat:write` or `engagement:write`. Mint a new one; upgrading will not help. |
| **⚠ Error**, log says *HTTP 401*    | Token expired, revoked or mistyped. Mint a fresh one and re-paste it. |
| **⚠ Error**, log says *HTTP 404*    | Wrong overlay ID, or the round already ended.             |
| **⚠ Error**, log says *HTTP 409*    | There is already an active poll/prediction on that overlay. |
| **⚠ Error**, log says *HTTP 429*    | Rate limited — a chat send fans out to every platform, and per-platform limits bind fastest. Press more slowly. |
| **⚠ Error**, *could not reach*      | Network, DNS or TLS. Check the Base URL if you self-host.  |

The plugin's messages appear in StreamController's log. None of them ever
contains your token.
