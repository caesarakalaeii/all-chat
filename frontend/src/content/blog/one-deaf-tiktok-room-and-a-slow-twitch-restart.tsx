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
import { External, Pre } from '@/components/docs/prose'
import type { BlogPostMeta } from './types'

export const meta: BlogPostMeta = {
  slug: 'one-deaf-tiktok-room-and-a-slow-twitch-restart',
  title: 'One deaf TikTok room, one slow Twitch restart, and the budgets they used up',
  description:
    'In late October one TikTok room that never heard anything spent the hourly connect budget meant for every TikTok streamer, and the Twitch listener’s startup outgrew its health check. Neither one crashed.',
  date: '2026-10-31',
  author: 'Caesar',
}

const PR_BASE = 'https://github.com/caesarakalaeii/all-chat/pull'
const PR_1119_URL = `${PR_BASE}/1119`
const PR_1120_URL = `${PR_BASE}/1120`

const BACKOFF_RESET = `// Reset backoff completely
state.consecutiveOfflineChecks = 0;
state.consecutiveErrors = 0;
state.currentBackoffMs = 0;`

export default function Body() {
  return (
    <>
      <p>
        Two things went wrong in the last week of October, on two different platforms, and neither
        of them was a crash. On TikTok, one room whose connection never received anything kept
        reconnecting until it had used up the hourly connection budget, and every other TikTok
        streamer who went live after that was turned away. On Twitch, a restart that had always
        worked stopped working, because the work the listener did before answering its health check
        had grown past the time the health check allows. Both are fixed. Both came down to the same
        thing: a budget I had set myself, spent by one piece of work on behalf of everyone else.
      </p>

      <section>
        <h2>Number one: the TikTok room that heard nothing</h2>
        <p>
          The{' '}
          <Link href="/blog/tiktok-live-chat-without-a-sign-server">
            post about TikTok’s sign server
          </Link>{' '}
          ended with All-Chat signing its own TikTok connections. Part of that setup is a limit I
          put on myself. New TikTok connections go through a session my signer holds, and TikTok
          notices a session that connects too often. So each TikTok listener pod allows itself 12
          new connections an hour, and refuses itself above that. A refusal is not an outage: the
          room is parked and tried again later. There is an alert for when the budget runs out,
          because a refused room is a streamer whose TikTok chat is not on their overlay.
        </p>
        <p>
          That alert fired on 2026-10-23, again on 2026-10-26, and twice on 2026-10-27. The obvious
          reading was that All-Chat had outgrown the budget: more TikTok streamers live at once than
          12 connections an hour can serve. Before raising the limit, I counted connections per room
          in the listener logs. They did not spread out the way growth would. Over about 30 hours,
          one room accounted for 58 connections and 56 silent failures. Overnight on one pod, 43 of
          57 connections were that room.
        </p>
        <p>
          The room was deaf. The connection was accepted, and then nothing that arrived on it could
          be decoded. The heartbeat monitor from the earlier post did its job: after about two
          minutes of silence it closed the connection, and the room reconnected roughly thirty
          seconds later. Between 18:19 and 18:42 UTC on 2026-10-27 it did that ten times on one pod,
          which spent the budget for the hour. After that, seven other streamers went live on TikTok
          and the pod refused every one of them.
        </p>
        <p>
          There was code meant to stop exactly this. A room that keeps failing silently has a
          streak, and the streak was supposed to slow its reconnects down through the same backoff
          that handles connection errors. It never did, and the reason is a function that is still
          correct today. When a connection succeeds, the backoff resets:
        </p>
        <Pre>{BACKOFF_RESET}</Pre>
        <p>
          For a room that drops now and then, that is right: once it connects, it is healthy again.
          A deaf room connects successfully every single time. So every reconnect wiped the error
          count the streak was trying to build up. In the logs the streak climbed to 42 while the
          backoff stayed at four seconds, one error, the whole time.
        </p>
        <p>
          The fix, merged on the evening of 2026-10-27, stops treating a silent failure as an error
          that a later success can cancel. Once a room has failed silently twice in a row it is
          parked: for 10 minutes, then 20, then at most 30. A room that stays deaf now costs about
          two connections an hour instead of 24. The park uses the same mechanism as a budget
          refusal, so the parts of the listener that already respected one now respect the other.
        </p>
        <p>
          The alert was not changed. It was right: rooms really were being refused. What was wrong
          was what one room was allowed to cost.
        </p>
        <p>
          I still do not know why that particular room’s connection receives nothing. The fix is
          about what a deaf room is allowed to cost, not about curing it.
        </p>
      </section>

      <section>
        <h2>Number two: the restart that ran out of time</h2>
        <p>
          On 2026-10-28 I rolled out a security hardening change that restarted every service in the
          cluster. Everything came back except one of the two Twitch listener replicas. It started,
          logged that it had a Twitch app token, and then said nothing until Kubernetes killed it.
          Then it started again, and the same thing happened.
        </p>
        <p>
          No error, no stack trace. The pod was killed from outside because its liveness check never
          answered. Kubernetes gives it 30 seconds before the first check and then three failed
          checks ten seconds apart, so a pod that is not answering after about 57 seconds is
          considered dead.
        </p>
        <p>
          The listener only starts answering that check once its HTTP server is up, and the HTTP
          server only started after the channel manager had finished its first sync. That first sync
          had to find out which Twitch channels need chat. Right after a restart the listener does
          not know yet which channels have an overlay open, so it plays safe and resolves every
          active Twitch login with Twitch. One request per login, one after the other. On 2026-10-28
          that was 352 logins, about 60 seconds of work. The pod was killed at 57, and every restart
          started from an empty cache, so it never got further.
        </p>
        <p>
          Nothing in the listener had changed. The number of Twitch channels on All-Chat had grown,
          and at some point in the past weeks startup quietly crossed the line. A restart of all
          services at once was simply the first time anyone asked it to start from cold.
        </p>
        <p>
          It did not cost anyone their chat that day. The other replica came up a little faster,
          took the leadership, and handled Twitch alone, so this was one replica down rather than
          Twitch chat down. That is what the second replica is for, and it would have been a bad way
          to find out it was needed.
        </p>
        <p>
          The fix, merged the same afternoon, moves the first sync into the background. The listener
          starts, answers its health checks, and syncs while it does. Stopping the listener now
          cancels a sync that is still running, instead of waiting for it, which could otherwise
          have outlasted the time Kubernetes allows for a clean shutdown. Moving the sync also
          opened a small ordering race when a replica becomes leader, so the leader now marks itself
          as leader before it resets what it has subscribed to, not after.
        </p>
        <p>
          The test for this points the listener at a database that accepts the connection and then
          never answers, and checks that it still starts and still stops. It failed before the
          change and passes after it. The real proof is the cluster: both replicas restarted again
          earlier today, 2026-10-31, and came up without a single crash.
        </p>
      </section>

      <section>
        <h2>The cost</h2>
        <ul>
          <li>
            A deaf TikTok room now sits parked for up to 30 minutes between attempts. If the room
            starts hearing chat again in the meantime, it can take that long to come back. I chose
            that over a room that starves every other TikTok streamer on its pod.
          </li>
          <li>
            The 12-an-hour budget itself is unchanged. If All-Chat ever does have more TikTok
            streamers going live in an hour than it allows, rooms will be refused again, and this
            time the alert will mean growth.
          </li>
          <li>
            The Twitch listener now answers its health check before it knows which channels it
            serves. Right after a restart a replica reports itself alive while its first sync is
            still running, so a passing health check says less than it used to.
          </li>
        </ul>
      </section>

      <section>
        <h2>Where it stands</h2>
        <p>
          The TikTok fix has been in production since the night of 2026-10-27. In the five days
          before it, the listeners refused themselves 139 times. In the three and a half days since,
          twice, and the budget alert has not fired once. The Twitch fix has been in production
          since 2026-10-28.
        </p>
        <p>What is still open:</p>
        <ul>
          <li>
            When a Twitch listener becomes leader after a restart, it re-creates the chat
            subscriptions for every channel one broadcaster at a time, and it holds a lock while it
            talks to Twitch. After today’s restart that was 59 broadcasters at about seven seconds
            each: the last one waited roughly six and a half minutes for its chat to come back. On a
            busier evening the list is longer and so is the wait. Fixing it means resolving channels
            in batches and taking the Twitch calls out from under the lock. That has not been done
            yet.
          </li>
          <li>
            Why one TikTok room stays deaf is not understood. Parking it limits the damage; it does
            not explain it.
          </li>
        </ul>
        <p>
          The lesson I took: a budget is shared unless something makes it otherwise. One deaf room
          could spend the connections every TikTok streamer on that pod relied on, and one slow step
          could spend the whole startup window for the rest of the listener. In both cases the code
          that spent it was doing something reasonable on its own. Nothing was watching what it cost
          everybody else.
        </p>
        <p>
          If you run TikTok next to other platforms, the{' '}
          <Link href="/tiktok-live-chat-overlay">TikTok LIVE chat overlay guide</Link> explains what
          to expect from it. The changes behind this post are{' '}
          <External href={PR_1119_URL}>#1119</External> for TikTok and{' '}
          <External href={PR_1120_URL}>#1120</External> for Twitch, and the last time Twitch chat
          went quiet is in{' '}
          <Link href="/blog/twitch-chat-went-quiet-without-an-error">
            three ways Twitch chat went quiet
          </Link>
          .
        </p>
        <p>
          <Link href="/">allch.at</Link>
        </p>
      </section>
    </>
  )
}
