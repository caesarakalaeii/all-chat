# Driving All-Chat from a Stream Deck

**Audience**: streamers running the Elgato Stream Deck plugin or the Linux StreamController plugin
**You will need**: an All-Chat account, and about five minutes once

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

Both plugins are free. So is installing them, and so is every action described
below. The one thing that asks for a premium account is *starting* a poll or a
prediction, for reasons explained under
[Where premium comes in](#where-premium-comes-in).

---

## Step 1: Mint a personal access token

A desktop plugin is not a browser. It has no cookie, no session, and nothing to
borrow one from, so it authenticates with a **personal access token** instead — a
long-lived, scoped, individually revocable credential you create once and paste
into the plugin. (The reasoning is recorded in
[ADR-0051](../adr/0051-personal-access-tokens-for-non-browser-clients.md).)

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

That last point is not an interface quirk to work around; it is the design. The
server stores only a SHA-256 hash of the token, never the token itself, so
"show it to me again" is impossible rather than merely unimplemented. If you
lose it, close the dialog too early, or read it aloud on stream by accident, the
fix is the same and it is cheap: revoke it on that page and mint a new one.
Nothing else about your account is affected, and your other tokens keep working.

---

## Step 2: Paste it into the plugin

Drag an All-Chat action onto a key, then open that key's settings.

- **Stream Deck** — the property inspector on the right. The field is
  **Access token**.
- **StreamController** — the action's settings panel. The field is
  **API token**.

Paste the whole string including the `allchat_pat_` prefix. The prefix is part
of the credential, not a label: the server routes on it to decide whether a
bearer is one of these tokens or an ordinary session token, so a token pasted
without it will not work.

Both plugins also have a **Server** field, and both default to
`https://allch.at`. Leave it alone unless you self-host, in which case set it to
your own base URL with no trailing slash.

You can paste the same token into every key. Many streamers prefer one token per
machine, so that revoking the token for a PC they no longer stream from does not
disturb the one under the desk.

### A note on where the token lives

The plugin keeps the token with the key's other settings and sends it in one
HTTP header. It is never written to the plugin log, never put in a URL, and
never echoed back inside an error message. When a diagnostic has to tell two
configured keys apart it prints `allchat_pat_…` plus the last four characters.

A personal access token also cannot do everything your login can. It cannot mint
more tokens, cannot delete your account, cannot export your data, and cannot
reach any admin surface — those refuse these tokens outright. A leaked token is
a bad day, not a lost account.

---

## Step 3: The actions

**Send chat message.** Type the message into the key's settings; pressing the key
fans it out to every chat you have connected — Twitch, YouTube, Kick, TikTok and
Discord — in one press. This is the action most people buy the hardware for.
Requires `chat:write`.

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

Each key targets one overlay, which you pick in the key's settings alongside the
token.

---

## Where premium comes in

The plugins are free, the tokens are free, and closing a poll, locking,
resolving or cancelling a prediction are all free. **Starting** a poll or a
prediction is the part that requires a premium account, and when you press a
start key without one, the plugin tells you so and points you at the upgrade
page. It is a prompt at the point of use, not a failure you need to debug.

The split is deliberate rather than incidental. Opening a round posts an
announcement to your chats and draws on your send quota, so it costs something
to run; the actions that wind a round down do not. It would also be a poor trade
to let a subscription lapse and leave a prediction locked open with viewers'
points inside it, so the actions that finish what you started stay available
whether or not you are currently premium.

Worth being explicit about, because it surprises people: the token is
**authentication, not authorization**. It proves who you are. It does not
upgrade you, and there is no scope that unlocks a premium feature. A correctly
scoped token belonging to a non-premium account is refused at a start action in
exactly the same way that account's browser session is refused. If a premium
feature works in the dashboard it will work from the key, and if it does not, it
will not.

---

## Troubleshooting

**`401` — the token is the problem.** The server did not accept the credential
at all: it was mistyped, truncated on paste, missing its `allchat_pat_` prefix,
or it has been revoked. Tokens are not recoverable, so do not hunt for the
original — mint a fresh one at
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
- **The overlay is not yours.** Check the overlay selected on the key.

The distinction to keep hold of is that **401 means re-mint the token** and
**403 means the token is working** — so re-minting it will not help, and the
answer is upgrading, re-scoping, or fixing the overlay.

**Nothing happens at all.** Check the **Server** field. A trailing slash, or a
self-host URL left over from testing, produces a request that never reaches
All-Chat.

**A start action reports a round is already running.** Polls and predictions are
one-at-a-time per overlay, including rounds started natively on Twitch. Close or
resolve the current one first.

---

## See also

- [ADR-0051: Personal access tokens for non-browser clients](../adr/0051-personal-access-tokens-for-non-browser-clients.md) — why these tokens exist and how they are stored
- [ADR-0049: Stream Deck and desktop control surfaces via paired device tokens](../adr/0049-desktop-control-surfaces-via-paired-device-tokens.md) — the intended one-click pairing flow these tokens precede
- [`streamdeck-plugin/README.md`](../../streamdeck-plugin/README.md) — installing and building the Elgato plugin
- [`streamcontroller-plugin/README.md`](../../streamcontroller-plugin/README.md) — installing the Linux StreamController plugin
