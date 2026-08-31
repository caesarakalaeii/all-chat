# Driving All-Chat from a Stream Deck

**Audience**: streamers running the Elgato Stream Deck plugin or the Linux StreamController plugin
**You will need**: an All-Chat account, and about a minute once — you press one button and approve it in your browser

---

## What these plugins are for

A multistreamer already has both hands busy. The dashboard is one more window
competing for attention on a second monitor, and reaching for it mid-sentence to
close a poll is exactly the moment you did not want to be looking at a browser.
A physical button removes that step: one press sends a message to every connected
chat at once, or opens a poll, or locks a prediction just as the play resolves.

Two plugins exist because the hardware is driven by two different pieces of
software. Elgato's own app runs on Windows and macOS and loads the **Stream Deck
plugin**. It does not run on Linux at all, so Linux users run
**StreamController**, which has its own plugin format and loads the
**StreamController plugin**. They are separate programs, but they talk to the
same All-Chat endpoints and are set up the same way, so this guide covers both
and calls out the two places they differ.

Both plugins are free. So is installing them, so is linking one to your account,
and so is every action described below. The one thing that asks for a premium
account is *starting* a poll or a prediction, for reasons explained under
[Where premium comes in](#where-premium-comes-in).

---

## Step 1: Link the plugin to your account

A desktop plugin is not a browser. It has no cookie, no session, and nothing to
borrow one from, so it needs a credential of its own. **You do not have to type
or paste one.** Drag an All-Chat action onto a key, open its settings, and press
**Link with All-Chat**.

What happens next takes about ten seconds:

1. The plugin opens your browser at an approve screen on <https://allch.at>.
   You are normally already signed in, so it appears straight away.
2. The screen tells you what is being granted: **which overlay** this device may
   control, **what it may do** (sending chat, running polls and predictions), and
   what the plugin calls itself. That last one is labelled *self-reported*,
   because it comes from the plugin and nobody can verify it — if you did not
   just press Link, deny it.
3. Press **Approve**. The credential goes straight from All-Chat to the plugin.

Nothing appears on screen for you to copy, which is the point: a secret you never
see cannot be read aloud, screenshotted, or caught by the camera. (The reasoning
is recorded in [ADR-0049](../adr/0049-desktop-control-surfaces-via-paired-device-tokens.md).)

Two properties of the result are worth knowing about, because they differ from
the pasted-token route below:

- **It is locked to one overlay.** A device you paired against your main overlay
  cannot drive a different one. To move it, revoke it and link it again.
- **It expires if you stop using it.** Ninety days, pushed forward every time the
  plugin makes a request, so a deck in daily service never lapses while a deck on
  a PC you sold does.

You can see and revoke every paired device at
<https://allch.at/settings/devices>. Revoking takes effect on that device's very
next request.

### If a code appears instead

Linking needs the plugin and your browser on the **same computer**. When they are
not — a Stream Deck driving a second PC, a headless capture box, a machine with
no desktop session — the plugin cannot hand the credential to itself over a local
connection, so it shows you an eight-character code instead:

1. The plugin displays something like `KRPT-8W4M`.
2. On any device where you are signed in, open <https://allch.at/link>.
3. Type the code and approve exactly as above. Capital letters and the dash are
   optional; the alphabet deliberately has no `0`/`O` or `1`/`I`/`l` in it.
4. Go back to the plugin. It is already finishing on its own.

The code lasts ten minutes and tolerates five wrong attempts before it dies, so
if you fumble it a couple of times just start again from the plugin.

---

## Step 2 (alternative): Paste a personal access token

This is the route for a machine linking cannot reach at all: a headless capture
box, a PC you only reach over SSH, a script. It is fully supported and is not
going away — it simply asks more of you, because a pasted secret is a secret you
have handled.

1. Sign in at <https://allch.at>.
2. Open **Settings → API tokens** (<https://allch.at/settings/api-tokens>).
3. Click to create a token and give it a name that tells you which machine it
   is on later — `stream-pc-deck` beats `token1` the day you want to revoke one
   of three.
4. Grant it the scopes you actually need:
   - **`chat:write`** — the *Send chat message* action.
   - **`engagement:write`** — the poll and prediction actions.

   A token with neither scope authenticates fine and is refused at every action,
   which is a confusing way to spend an evening. Grant both if you intend to use
   both.
5. **Copy the token now.** It begins with `allchat_pat_` and it is shown exactly
   once.
6. Paste the whole string, prefix included, into the key's **Pasted token**
   field.

That "shown exactly once" is not an interface quirk to work around; it is the
design. The server stores only a SHA-256 hash of the token, never the token
itself, so "show it to me again" is impossible rather than merely unimplemented.
If you lose it, close the dialog too early, or read it aloud on stream by
accident, the fix is the same and it is cheap: revoke it on that page and mint a
new one. Nothing else about your account is affected, and your other tokens keep
working.

The prefix is part of the credential, not a label: the server routes on it to
decide whether a bearer is one of these tokens or an ordinary session token, so a
token pasted without it will not work.

Unlike a linked device, a pasted token is **not** tied to one overlay and has
**no** expiry unless you set one — which is exactly why it works on a box that
cannot link, and exactly why linking is the better choice when it is available.

### The Server field, either way

Both plugins have a **Server** field and both default to `https://allch.at`.
Leave it alone unless you self-host, in which case set it to your own base URL
with no trailing slash.

### A note on where the credential lives

The plugin keeps it with the key's other settings and sends it in one HTTP
header. It is never written to the plugin log, never put in a URL, and never
echoed back inside an error message. When a diagnostic has to tell two configured
keys apart it prints only the prefix — `allchat_dev_…` for a linked device,
`allchat_pat_…` for a pasted token — so a log line says which *kind* of
credential is configured without disclosing any of it.

Neither kind can do everything your login can. Neither can mint credentials,
delete your account, export your data, or reach any admin surface — those refuse
both outright. A leaked credential is a bad day, not a lost account.

---

## Step 3: The actions

**Send chat message.** Type the message into the key's settings; pressing the key
fans it out to every chat you have connected — Twitch, YouTube and Kick — in one
press. This is the action most people buy the hardware for. Requires
`chat:write`.

Those three are the platforms All-Chat can *post* to, which is a shorter list
than the platforms it *reads*. TikTok publishes no API for sending into a live
chat, and a Discord source is a one-way relay into your overlay, so messages
still arrive from both but neither can be a target. They are not offered in the
platform picker and are not included in "all".

**Poll control.** One action with a mode you choose per key. *Start* opens a poll
from the question and options configured on that key. *Close* ends the poll
that is currently running on the overlay. A common layout is one key to start
your usual poll and one key beside it to close it. Requires `engagement:write`.

**Prediction control.** The same shape, with four modes, because a prediction has
a longer life than a poll. *Start* opens it. *Lock* stops new wagers, which is
the one you want bound to a key you can hit without looking, since it is
normally pressed at a precise moment. *Resolve* settles it on the winning
outcome and pays out. *Cancel* calls it off and refunds. Requires
`engagement:write`.

Each key targets one overlay. A **linked** device already has one — you chose it
on the approve screen, and the server refuses a request for any other overlay, so
a compromised deck cannot reach the rest of your account. With a **pasted** token
you set the overlay per key instead, and nothing stops you pointing a key at a
different overlay you own.

---

## Where premium comes in

The plugins are free, linking is free, tokens are free, and closing a poll,
locking, resolving or cancelling a prediction are all free. **Starting** a poll or a
prediction is the part that requires a premium account, and when you press a
start key without one, the plugin tells you so and points you at the upgrade
page. It is a prompt at the point of use, not a failure you need to debug.

The split is deliberate rather than incidental. Opening a round posts an
announcement to your chats and draws on your send quota, so it costs something
to run; the actions that wind a round down do not. It would also be a poor trade
to let a subscription lapse and leave a prediction locked open with viewers'
points inside it, so the actions that finish what you started stay available
whether or not you are currently premium.

Worth being explicit about, because it surprises people: the credential is
**authentication, not authorization**. It proves who you are. It does not
upgrade you, and there is no scope that unlocks a premium feature. A correctly
scoped device or token belonging to a non-premium account is refused at a start
action in exactly the same way that account's browser session is refused. If a premium
feature works in the dashboard it will work from the key, and if it does not, it
will not.

---

## Troubleshooting

**`401` — the credential is the problem.** The server did not accept it at all.
Which fix applies depends on how the key was set up:

- **A linked device**: it was revoked, or it lapsed after ninety days unused.
  Press **Link with All-Chat** again. Check
  <https://allch.at/settings/devices> first if you want to see which.
- **A pasted token**: it was mistyped, truncated on paste, missing its
  `allchat_pat_` prefix, or revoked. Tokens are not recoverable, so do not hunt
  for the original — mint a fresh one at
  <https://allch.at/settings/api-tokens> and paste it in again. If one key works
  and another does not, compare their token fields; a partial paste is the usual
  cause.

**`403` — the token is fine, the request is not allowed.** You authenticated
successfully and were then refused, which narrows it to three things:

- **The feature needs premium.** By far the most common, and it means you
  pressed *start* on a poll or prediction. Follow the upgrade prompt.
- **The token is missing a scope.** An `engagement:write`-only token cannot send
  chat, and a `chat:write`-only token cannot touch polls or predictions. Scopes
  are fixed when the token is created, so mint a replacement with both.
- **A linked device is pointed at the wrong overlay.** A device is locked to the
  overlay you picked when you approved it, and a request for any other one is
  refused by design. Revoke it at <https://allch.at/settings/devices> and link
  again against the overlay you want. (This case cannot happen with a pasted
  token, which is not overlay-bound.)
- **The overlay is not yours.** Check the overlay selected on the key.

The distinction to keep hold of is that **401 means re-link or re-mint** and
**403 means the credential is working** — so re-doing it will not help, and the
answer is upgrading, re-scoping, or fixing the overlay.

**Link does nothing, or hangs.** The browser could not be opened, or the plugin
could not open a local port — a sandboxed host, a locked-down container, a
machine with no desktop. The plugin should fall back to the pairing code on its
own; if it does not, use <https://allch.at/link> with the code it shows, or paste
a personal access token instead.

**Nothing happens at all.** Check the **Server** field. A trailing slash, or a
self-host URL left over from testing, produces a request that never reaches
All-Chat.

**A start action reports a round is already running.** Polls and predictions are
one-at-a-time per overlay, including rounds started natively on Twitch. Close or
resolve the current one first.

---

## See also

- [ADR-0049: Stream Deck and desktop control surfaces via paired device tokens](../adr/0049-desktop-control-surfaces-via-paired-device-tokens.md) — why linking works the way it does, and why the loopback redirect gets its own validation rule
- [ADR-0051: Personal access tokens for non-browser clients](../adr/0051-personal-access-tokens-for-non-browser-clients.md) — why the pasted-token route exists, how tokens are stored, and why it stays supported
- [`streamdeck-plugin/README.md`](../../streamdeck-plugin/README.md) — installing and building the Elgato plugin
- [`streamcontroller-plugin/README.md`](../../streamcontroller-plugin/README.md) — installing the Linux StreamController plugin
