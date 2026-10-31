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
  slug: 'tiktok-live-chat-without-a-sign-server',
  title: 'TikTok LIVE chat without a third-party sign server, and what it cost',
  description:
    'How All-Chat stopped depending on a third-party sign server for TikTok LIVE chat: a free-tier ceiling, a TikTok rule change halfway through, and the outages on the way.',
  date: '2026-10-13',
  author: 'Caesar',
}

const ADR_0052_URL =
  'https://github.com/caesarakalaeii/all-chat/blob/main/docs/adr/0052-retiring-euler-stream-for-tiktok-signing.md'

export default function Body() {
  return (
    <>
      <p>
        Every TikTok LIVE connection All-Chat makes is now signed by a service I run myself. Until
        this autumn, each one went through Euler Stream, a third-party sign server. Since the night
        of 2026-09-17, production has had no Euler leg on any connection. Getting there took from
        mid-August to 2026-09-19, a rule change on TikTok’s side halfway through, a cutover done in
        under three hours because there was no longer anything to cut over from, and several
        outages, at least one of them shipped by me.
      </p>

      <section>
        <h2>The observation</h2>
        <p>
          TikTok has no public API for LIVE chat. Every tool that shows it reads chat the way
          TikTok’s own web player does, and that requires a signed connection: a request carrying
          values TikTok’s web code computes and checks. The library All-Chat uses for TikTok does
          not compute them. By default it asks Euler Stream to.
        </p>
        <p>
          On 2026-08-14 I was investigating something else entirely, a coin chest a streamer
          reported missing. Along the way I opened twelve TikTok connections at the same time. That
          was enough to exhaust the free tier of the sign service. The failure did not even arrive
          as a rate limit. It arrived as this:
        </p>
        <Pre>{"Cannot read properties of undefined (reading 'retry-after')"}</Pre>
        <p>
          The service had answered “too many requests”. The library tried to read how long to wait,
          found nothing to read, and threw. So the first symptom of a hard ceiling on how many
          TikTok streamers All-Chat could serve was a crash inside an error handler.
        </p>
        <p>
          Two more costs turned up the same day. Gift details for TikTok sit behind a paid plan on
          the sign service, which is why TikTok gifts were never enriched the way they are on other
          platforms. And if a connection ever used a logged-in TikTok session, the library would
          forward that session cookie to the sign server. None of that was anyone’s fault. It was
          simply the deal the free tier offered, and it was shaping the product in ways I had not
          chosen.
        </p>
      </section>

      <section>
        <h2>The dig</h2>
        <p>
          My first estimate was “replace the library”. Reading its bundled source made the job much
          smaller. The library keeps its network routes in a registry it explicitly invites you to
          swap out:
        </p>
        <blockquote className="border-l-2 border-white/40 pl-4 italic">
          <p>
            Global route registry. Call sites should read handlers from here rather than importing
            the route functions directly, so downstream consumers can swap implementations.
          </p>
        </blockquote>
        <p>
          So no fork. Better still, room lookup and the “is this account live” check already had
          direct routes to TikTok and only fell back to Euler when those failed. I turned that
          fallback off first. To prove it was really off, the test points the sign service’s address
          at TCP port 9 before it runs, a port where nothing listens. If anything still reached for
          Euler, it would hit a dead address. Room IDs and live checks for three public accounts
          came back fine.
        </p>
        <p>
          The signature was the real work, and reading the source made it look worse, not better.
          The sign service does not just sign a URL. It performs the first request to TikTok itself
          and hands back the decoded answer plus the cookies TikTok set. A replacement has to
          reproduce the whole exchange. The signing logic is nowhere in the library to copy. It
          lives on the sign server.
        </p>
        <p>
          There was also a second seam I had not expected. Gift details are fetched from TikTok
          directly, but that request is signed too, through a separate hook. The paid-plan error was
          never about the gift data. It was about signing my own request for it.
        </p>
        <p>
          On 2026-08-17 I shipped the switches and left them off. The plan had three modes, walked
          in order: keep Euler, then run my own signer alongside it and record both results for the
          same room at the same moment, then cut over with Euler as a safety net, then remove the
          net. The ADR states the trade-off in so many words: today a TikTok change is Euler’s
          problem and the fix comes free. Afterwards it is my problem, and TikTok chat is down until
          I fix it.
        </p>
      </section>

      <section>
        <h2>The calendar intervenes</h2>
        <p>
          On 2026-09-14 the ceiling hit production before any of this was switched on. None of my
          alerts noticed. A user did. The symptom was odd: TikTok chat did arrive, just minutes late
          and in batches.
        </p>
        <p>
          My first suspect was TikTok changing its data format again. It had done that before: in
          July, chat cards on the overlay showed a viewer’s name and no message, because TikTok had
          renamed the field that carries the message text and my library was still reading the old
          one. But a format change empties messages. It does not make them late.
        </p>
        <p>
          One listener pod had grabbed 43 of about 48 TikTok rooms at boot. The sign service’s free
          tier also limits how many live connections it carries at once, and that pod was well over
          it. Above the limit, a connection was accepted, the room’s backlog was replayed, and then
          nothing live followed. My heartbeat monitor noticed the silence and forced a reconnect
          roughly every two minutes per room. Each reconnect replayed the backlog. Each replay
          looked like a batch of late chat.
        </p>
        <p>
          The fix the next day did not raise the ceiling. It spread rooms evenly across pods, the
          same rebalancing the Go listeners already had, and capped each pod at 20 connections, with
          a visible warning instead of silent deafness above that.
        </p>
        <p>
          Later on 2026-09-15 my own signer went into production, still switched off. Around the
          same time, Euler’s sign endpoint started answering 404. Every TikTok connection All-Chat
          made went through it, so TikTok chat was down. The open-source community established the
          common cause within the day (
          <External href="https://github.com/zerodytrash/TikTok-Live-Connector/issues/329">
            TikTok-Live-Connector#329
          </External>
          ,{' '}
          <External href="https://github.com/isaackogan/TikTokLive/issues/376">
            TikTokLive#376
          </External>
          ): since 2026-09-09, TikTok had been judging the session, not the signature.
        </p>
        <p>
          The careful plan from August had a measurement step: run my signer alongside Euler and
          compare. At 20:40 that evening (all times in this post are CEST) I skipped it. The
          deployment commit gives the reason: “there is no baseline left to measure against.”
          Production switched to my signer with Euler kept as a fallback, in case it came back.
        </p>
        <p>
          It did not help for long. My signed request now came back with status 200 and zero bytes
          of content. The signature was accepted. It just no longer bought anything. So the signer
          changed shape that same evening, live at 22:25. Instead of computing signatures, it keeps
          a real browser tab open on the streamer’s live page, a viewer like any other, and hands
          over what that tab receives. I am not going to describe how that tab is set up. A post
          that teaches TikTok how to tell it apart from a person is a post that turns the overlay
          off.
        </p>
      </section>

      <section>
        <h2>The wrong turns</h2>
        <p>
          The next day every capture failed. All of them, across every room. With Euler gone, a
          failed capture meant no TikTok chat at all.
        </p>
        <p>
          That morning I raised the capture timeout three times, from 30 to 45 to 60 seconds, and
          changed the browser’s graphics setup twice, to make it look more like an ordinary desktop.
          In the afternoon an A/B test on the same room settled it. With my graphics changes, the
          page never made a single chat request in four minutes. Without them, it captured every
          time. After a browser update, the setting meant to look normal had become the thing that
          looked abnormal. I removed it.
        </p>
        <p>
          That night I built the second delivery path that ADR-0064 describes. When TikTok keeps
          refusing a premium streamer’s main connection, delivery switches to the frames that
          streamer’s viewer tab is already receiving. Building it meant rebuilding the code that
          decodes those frames, and that turned up a bug in the canary from the previous step. The
          canary compares the tab’s frames with the main connection’s frames and raises an alarm on
          four kinds of disagreement. It called an asynchronous decoder without <Code>await</Code>:
        </p>
        <Pre lang="javascript">
          {
            "// the fixed line; the bug was the same line without `await`\nconst decoded = await deserializeWebSocketMessage(Buffer.from(base64, 'base64'));"
          }
        </Pre>
        <p>
          Without <Code>await</Code>, <Code>decoded</Code> is a promise, not a result. It has no
          messages, so every frame looked like an empty keepalive, and every frame counted as
          healthy. Two of the four alarms could never fire. The commit message for the canary says
          “divergence counters firing”, which was true for the other two.
        </p>
        <p>
          Later on 2026-09-17 I shipped a performance change to the capture path. It taught the
          capture to give up early if one particular page element had not appeared, an element
          TikTok’s live page does not actually render. It also blocked one of TikTok’s own
          stylesheets as if it were third-party, and without its CSS the player never starts. From
          that deploy, a little after 16:00, 219 of 219 captures were rejected and every TikTok room
          on All-Chat went dark. The fix was committed at 22:09. That is about six hours of no
          TikTok chat, and the outage the ADR’s trade-off paragraph predicted, delivered by me
          rather than by TikTok.
        </p>
        <p>
          With the fix out, the signer carried 35 captures with 2 rejected. Euler, meanwhile, had
          answered every rescue attempt since 2026-09-15 with a failure in about five milliseconds.
          At 22:59 I removed it from the connect path. It had not been doing anything except fill
          error logs.
        </p>
        <p>
          Then TikTok had its turn. From about 11:00 on 2026-09-19, TikTok refused every capture
          from production: 45 requests, 0 successes. The same code, run from my lab, captured the
          same rooms in about five seconds. So it was not a bug, it was TikTok scoring where the
          requests came from. I had a change ready from the day before that moved the browser work
          off the per-room path, so production needs far fewer captures. It went live at 12:37. The
          details stay out of this post.
        </p>
        <p>
          The same deploy switched on the premium fallback. Two days earlier I had pinned it off
          “until a week of canary data”. It got two days, because the alternative during a refusal
          wall was no delivery at all.
        </p>
      </section>

      <section>
        <h2>The cost</h2>
        <p>The ceiling is gone, and so is the third party on every connection. In exchange:</p>
        <ul>
          <li>
            TikTok changes are my on-call problem now, with no one upstream to fix them for free.
            The outages above show the shape of that: some shipped by me, some by TikTok.
          </li>
          <li>
            The fallback path costs about 150 MB of memory per room while it is active, so it is
            reserved for premium streamers, and a fallback stint ends after six hours at most. That
            is a capacity decision, not a feature split.
          </li>
          <li>
            More moving parts. The listener used to talk to TikTok and to one sign server. It now
            talks to TikTok and to a browser fleet I run.
          </li>
          <li>
            The current mode does not enable gift details, so TikTok gifts are still not enriched
            the way they are on other platforms. One of the three original reasons for all this is
            still open.
          </li>
        </ul>
      </section>

      <section>
        <h2>Where it stands</h2>
        <p>
          TikTok LIVE chat works on All-Chat without a third-party sign server, on the free plan
          like before. Premium rooms get the second delivery path when TikTok refuses the main one.
          TikTok is still the platform most likely to see short interruptions. When TikTok changes
          its web player, every tool that reads TikTok LIVE chat has to adapt, and this one now
          adapts on my schedule rather than someone else’s. Twitch, YouTube, Kick and Discord in the
          same overlay keep running while it does.
        </p>
        <p>
          The setup is in the{' '}
          <Link href="/tiktok-live-chat-overlay">TikTok LIVE chat overlay guide</Link>, and the full
          decision history, updates included, is in{' '}
          <External href={ADR_0052_URL}>ADR-0052</External>.
        </p>
        <p>
          <Link href="/">allch.at</Link>
        </p>
      </section>
    </>
  )
}
