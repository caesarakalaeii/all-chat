/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import Link from 'next/link'
import { Code, External, Pre } from '@/components/docs/prose'
import type { BlogPostMeta } from './types'

export const meta: BlogPostMeta = {
  slug: 'twitch-chat-went-quiet-without-an-error',
  title: 'Three ways Twitch chat went quiet on the overlay, and none of them threw an error',
  description:
    'Retiring the Twitch IRC connection removed a fallback three pieces of code relied on: a daily Twitch chat dropout, watch streaks that never showed, and a revoked grant nobody could see.',
  date: '2026-10-15',
  author: 'Caesar',
}

const ADR_BASE = 'https://github.com/caesarakalaeii/all-chat/blob/main/docs/adr'
const ADR_0032_URL = `${ADR_BASE}/0032-source-liveness-heartbeat-and-overlay-self-heal.md`
const ADR_0046_URL = `${ADR_BASE}/0046-twitch-chat-notices-via-chat-notification.md`

const SCOPE_ERROR_COMMENT = `// A scope error is non-fatal: return nil so the manager marks chat active and
// doesn't retry-spam; that channel just won't get EventSub chat (it stays on IRC
// unless/until the owner grants the missing scope).`

export default function Body() {
  return (
    <>
      <p>
        Between July and September, Twitch chat disappeared from All-Chat overlays three separate
        times, for three separate reasons, and not one of them produced an error. An overlay left
        open around the clock lost its Twitch chat once a day. Watch-streak messages and
        announcements were never shown at all. And a streamer whose Twitch authorization had been
        revoked lost all of their Twitch chat, with every re-add quietly reusing the dead grant. All
        three are fixed. They turned out to have one cause in common: when I retired the old Twitch
        IRC connection, I also removed a safety net that three pieces of code were standing on
        without saying so.
      </p>

      <section>
        <h2>The safety net</h2>
        <p>
          All-Chat used to read Twitch chat the way most chat tools do: one bot account joins your
          channel over IRC, Twitch’s old chat protocol, and reads along. That bot account can only
          sit in about 100 channels. On 2026-06-01 it was in 74, and climbing.
        </p>
        <p>
          So on 2026-06-01 I added a second path. EventSub is Twitch’s newer event API: instead of a
          bot sitting in your chat, Twitch sends each message to All-Chat, authorized by a grant you
          give once when you add your Twitch source. It has no channel cap. For a while both paths
          ran side by side, and any channel EventSub could not serve stayed on IRC.
        </p>
        <p>
          Then IRC was wound down in two phases. From 2026-06-23 the overlay showed a notice every
          five minutes asking streamers to re-add their Twitch source. On 2026-06-30 the IRC
          listener was switched to join no channels at all. At that moment 168 sources already had
          the EventSub grant and kept their chat, and 104 did not and lost Twitch chat until they
          re-added it. That loss was the announced one. From that point, EventSub was the only way
          Twitch chat reached an overlay.
        </p>
        <p>
          That is the plan working as designed. The problem is what else had quietly been relying on
          IRC.
        </p>
      </section>

      <section>
        <h2>Number one: the overlay that forgot Twitch every 24 hours</h2>
        <p>
          On 2026-07-13, thirteen days after the switch, I found it on my own stream: the overlay in
          my OBS scene had stopped showing Twitch chat, and at least one other streamer’s overlay
          did the same in the same hour. YouTube chat on the same overlay was fine. Refreshing the
          Browser Source brought Twitch back. A day later, it was gone again.
        </p>
        <p>
          All-Chat has a cleanup job that switches off chat sources nobody is watching, so that
          listeners stop polling, and YouTube stops charging quota, for overlays that were abandoned
          weeks ago. It decides “nobody is watching” by a timestamp: a source whose last-updated
          time is more than 24 hours old gets marked inactive. That only works if something keeps
          touching the timestamp for sources that <em>are</em> in use. The YouTube and Kick
          listeners did that on every poll. The IRC listener did it for Twitch.
        </p>
        <p>
          The EventSub listener never did. It only ever read the source table. As long as IRC was
          joining channels, that did not matter. Once IRC joined nothing, nothing touched a Twitch
          source’s timestamp, and 24 hours after an overlay was opened, the cleanup job did exactly
          what it was built to do. The message processor, which only routes messages to active
          sources, then dropped every Twitch message for that overlay.
        </p>
        <p>
          The part that made it silent was the overlay itself. It has a watchdog that reconnects
          when the connection goes quiet. But the connection was not quiet: the server answers a
          heartbeat every 20 seconds, and YouTube messages kept arriving. From the overlay’s point
          of view, everything was fine. It just had less to say. A manual refresh fixed it because
          reconnecting re-activates the overlay’s sources, which is also why the cycle restarted the
          next day.
        </p>
        <p>
          The fix, merged the same evening, moved the heartbeat to the one place that actually knows
          an overlay is in use: the server holding its connection. Every two minutes it now
          refreshes the timestamp of every active source on every connected overlay, for every
          platform, whatever the listener does. The EventSub listener also heartbeats its own
          sources now, as a second line. And the overlay treats a source coming back from “down” to
          “connected” as a reason to reconnect and replay what it missed, so a gap of this kind
          heals without anyone pressing refresh.
        </p>
        <p>
          The cleanup job was not changed. It was right. The contract it depended on was simply not
          being kept.
        </p>
      </section>

      <section>
        <h2>Number two: the messages that were never subscribed to</h2>
        <p>
          I noticed the second one myself too: watch streaks that showed up in Twitch’s own chat
          never reached my overlay. It was fixed on 2026-08-01, three weeks after the first, and
          this one had never worked.
        </p>
        <p>
          Twitch delivers chat over EventSub on two separate subscriptions.{' '}
          <Code>channel.chat.message</Code> carries ordinary chat.{' '}
          <Code>channel.chat.notification</Code> carries things that happen in chat: subs, raids,
          and a few notices that contain the viewer’s own message. A watch streak is one of those.
          When a returning viewer hits a milestone, the message they type is delivered inside the
          notice, and Twitch does not send it a second time as ordinary chat. The same goes for{' '}
          <Code>/announce</Code>.
        </p>
        <p>
          All-Chat subscribed to the first and never to the second. So the viewer typed, Twitch
          celebrated their streak in its own chat, and the overlay showed nothing. No error, because
          there was nothing to fail: no subscription, so no event, so no unhandled event in a log.
          On EventSub, that had been the case from the day the path shipped, 2026-06-01, to the day
          of the fix, 2026-08-01: two months.
        </p>
        <p>
          Before that, IRC would not have saved it either. The IRC parser mapped watch streaks and
          announcements to <Code>unknown</Code> and rejected them. That loss was at least noisy, in
          a log nobody read for it. Neither path had a way to show a watch streak.
        </p>
        <p>
          The fix subscribes to the notification feed alongside the chat subscription, skips the
          five notice types a dedicated subscription already delivers (so subs and raids do not
          appear twice), maps the rest, and turns anything Twitch adds in future into a generic
          notice plus a log line rather than nothing. Watch streaks got their own toggle in the
          overlay settings.
        </p>
        <p>
          That toggle was the first wrong turn, and I merged it. The message processor drops a
          disabled event type outright, which is correct for a follow or a raid. For a watch streak,
          the event <em>is</em> the viewer’s chat message. Any streamer who found the streak alerts
          noisy and switched them off would have deleted those viewers’ first message of every
          stream, recreating the exact bug the change had fixed, this time from the settings page. A
          review of my own diff caught it an hour and a half after merge. A disabled watch streak
          now drops only the milestone decoration and shows the message as ordinary chat. The
          toggle’s description says so.
        </p>
        <p>
          The same review found a second one. The message processor has two checks that must agree
          on which messages carry a Twitch message ID: one for duplicate detection, one for
          deletions that arrive before the message they delete. I had widened the first for notices
          and not the second, which sat forty lines above it. When a moderator’s deletion raced
          ahead of a watch streak, the deleted message would have stayed on every overlay until the
          buffered deletion expired. Both checks now call one function, so they cannot disagree
          again.
        </p>
        <p>
          The review also showed that a sentence in my own decision record was wrong, and that one
          pointed at what came next. A chat subscription only got created when an overlay started
          needing it. If Twitch later revoked one, nothing tried again until demand cycled or the
          pod restarted. For ordinary chat that is loud, since everything stops. For the
          notification feed it would have been silent all over again. Since 2026-08-01 there is a
          repair pass every five minutes that re-asserts the full set of subscriptions for every
          channel that should have chat.
        </p>
        <p>That repair pass matters in the third story, though not in the way I meant it to.</p>
      </section>

      <section>
        <h2>Number three: the grant that was dead, and the comment that was true once</h2>
        <p>
          On 2026-09-25 at 00:44, Twitch revoked every EventSub subscription for one broadcaster in
          one go: for a streamer with the default grant, that is about eleven revocations at once,
          one per event type. That pattern is what you see when a streamer’s authorization for the
          app has been withdrawn. The streamer saw follows and raids still arriving, YouTube fine,
          and no Twitch chat. They did the reasonable thing and re-added their Twitch source.
          Nothing changed. Then they asked on Discord.
        </p>
        <p>
          I was dumbfounded. Of all the platforms All-Chat reads, Twitch had been the stable one. I
          started digging on 2026-09-26, and the fix was merged the morning after.
        </p>
        <p>Two bugs stacked here.</p>
        <p>
          The first was in how a re-add works. All-Chat records which permissions you granted, and
          if you already granted the chat permissions, it skips the Twitch consent screen. That
          shortcut was added on 2026-06-01 with the EventSub path. It trusted the stored record.
          After the revocation, the record still listed the chat permissions, so every re-add reused
          the dead grant without ever showing the consent screen that would have fixed it.
        </p>
        <p>
          The second was in the listener. When Twitch refused the chat subscription with a 403, this
          is what the code did, as of the commit that removed it:
        </p>
        <Pre>{SCOPE_ERROR_COMMENT}</Pre>
        <p>
          That comment was written on 2026-06-01, and for 29 days it was true. A channel EventSub
          could not serve did stay on IRC. On 2026-06-30 IRC was switched to join nothing, and the
          comment went on describing a fallback that no longer existed for another three months.
          Returning nil marked the channel as having active chat, wrote the claim that tells the
          rest of the system “EventSub owns this channel”, and logged nothing worth noticing. The
          channel’s Twitch chat was gone until someone looked.
        </p>
        <p>
          The fix had wrong turns of its own. My first version checked the stored grant against
          Twitch’s validation endpoint before trying a refresh. That endpoint answers 401 for an
          expired token as well as a revoked one, so the first version would have treated every
          merely expired token as revoked, wiped the record and sent the streamer through a consent
          screen they did not need. A self-review caught it before merge: refresh first, then
          validate. A refresh never widens permissions, so the answer is the same and the false
          alarm is gone.
        </p>
        <p>
          A second review, still before merge, found the bigger one, and it was the incident’s own
          shape. In this incident, the 403s did not arrive when chat was first set up. They arrived
          on the five-minute repair pass from August, on a channel that had been working fine. And
          the repair pass dropped every error. So the channel stayed marked active forever, the
          ownership claim was re-asserted every pass against the revocation that had just released
          it, and the new heartbeat from July kept the source looking alive. Two fixes from earlier
          in this post were, between them, keeping a dead channel looking healthy. I find that fair,
          if not flattering.
        </p>
        <p>
          What happens now: a refused chat subscription is an error. The channel is marked inactive,
          the claim is released, and the overlay shows the source as <strong>Auth Required</strong>{' '}
          in red, which is the existing indicator for exactly this case. Re-adding the source now
          checks the grant against Twitch instead of trusting the record, so it shows the consent
          screen when it has to. And a revocation from Twitch now increments a metric with an alert
          on it, so the next mass revocation reaches me before it reaches a streamer.
        </p>
      </section>

      <section>
        <h2>The cost</h2>
        <p>Each fix bought something and costs something.</p>
        <ul>
          <li>
            The heartbeat means an overlay that stays connected keeps its sources alive
            indefinitely. That is the point, but it also means an overlay left open in a forgotten
            OBS scene keeps a YouTube source polling. Abandoned overlays are still reaped, 24 hours
            after their last connection closes.
          </li>
          <li>
            The notification subscription is one more subscription per channel to keep in step, and
            the list of notice types to skip is a closed set: if All-Chat ever drops one of the
            dedicated subscriptions, that notice type would quietly go missing again. The repair
            pass is a periodic job that talks to Twitch. A channel whose grant can never be
            satisfied is retried once per pass, indefinitely.
          </li>
          <li>
            Watch streaks cannot be tested end to end. Twitch only emits one when a real viewer’s
            watch history crosses a milestone, and there is no way to trigger one on demand. The
            test that exists drives a verbatim Twitch payload through the listener, the message
            stream and the message processor. That is as far upstream as a test can start.
          </li>
        </ul>
      </section>

      <section>
        <h2>Where it stands</h2>
        <p>
          All three are fixed in production: the 24-hour dropout since 2026-07-13, watch streaks and
          announcements since 2026-08-01, the dead-grant case since 2026-09-27.
        </p>
        <p>What is still open:</p>
        <ul>
          <li>
            The overlay’s watchdog still cannot tell “this source is quiet” from “this source is
            dead”. It relies on listeners reporting a status. A content-aware watchdog was
            deliberately left out; the heartbeat removes the cause instead.
          </li>
          <li>
            If your Twitch authorization is revoked, All-Chat can now tell you, but it cannot fix it
            for you. The source shows Auth Required until you re-add it and approve the consent
            screen.
          </li>
          <li>
            The old IRC parser still drops watch streaks. It serves no channels, so this changes
            nothing today, and it is scheduled for removal.
          </li>
        </ul>
        <p>
          The lesson I took is narrower than “test more”. Each of these was correct code relying on
          something it never named. The cleanup job relied on heartbeats. The listener relied on
          IRC. The comment relied on a world that ended on the day the IRC listener stopped joining
          channels. When you remove a fallback, the things that leaned on it do not fail. They just
          stop being right.
        </p>
        <p>
          If you run Twitch alongside other platforms in one overlay, the{' '}
          <Link href="/multistream-chat">multistream chat guide</Link> and the{' '}
          <Link href="/obs-chat-overlay">OBS chat overlay guide</Link> cover the setup. The decision
          records behind the first two fixes are <External href={ADR_0032_URL}>ADR-0032</External>{' '}
          and <External href={ADR_0046_URL}>ADR-0046</External>.
        </p>
        <p>
          <Link href="/">allch.at</Link>
        </p>
      </section>
    </>
  )
}
