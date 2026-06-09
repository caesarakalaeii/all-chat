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
import LegalLayout from '@/components/legal/LegalLayout'

export const metadata = {
  title: 'Privacy Policy | All-Chat',
  description: 'Learn how All-Chat collects, processes, and protects your information.',
  alternates: { canonical: '/legal/privacy' },
}

const listClasses = 'list-disc pl-6 space-y-1 text-text-sub'

export default function PrivacyPolicyPage() {
  return (
    <LegalLayout title="Privacy Policy (Datenschutzerkl&auml;rung)" lastUpdated="June 8, 2026">
      <div className="space-y-4">
        <div className="rounded-xl border border-twitch/20 bg-twitch/5 p-5 text-text-sub">
          <strong className="text-text">TL;DR:</strong> All-Chat only collects the information we
          need to authenticate with your streaming platforms and render chat in your overlays. Tokens
          are encrypted, chat messages are automatically deleted after one hour, and we never sell
          your data. We use cookieless, self-hosted analytics (no tracking cookies; see
          Section&nbsp;5.6).
        </div>
        <div className="rounded-xl border border-tiktok/20 bg-tiktok/5 p-5 text-text-sub">
          <strong className="text-text">Open Source Transparency:</strong> All-Chat is licensed
          under AGPL-3.0. Review the entire codebase&mdash;including how we store and process your
          data&mdash;on{' '}
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold text-tiktok underline decoration-tiktok/30 underline-offset-4"
          >
            GitHub
          </a>
          .
        </div>
      </div>

      {/* --- Verantwortlicher --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          1. Controller (Verantwortlicher, Art.&nbsp;4 Nr.&nbsp;7 DSGVO)
        </h2>
        <p>
          The person responsible for data processing on this website is listed in the{' '}
          <Link
            href="/legal/impressum"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Impressum
          </Link>
          . For privacy-related inquiries contact{' '}
          <a
            href="mailto:all.chat.support@gmail.com"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            all.chat.support@gmail.com
          </a>
          .
        </p>
      </section>

      {/* --- Information We Collect --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">2. Information We Collect</h2>

        <div>
          <h3 className="text-lg font-semibold text-text">2.1 Authentication Information</h3>
          <p>
            When you connect Twitch, YouTube, TikTok, Kick, or Discord we store the minimum data
            required to create overlays and reconnect later:
          </p>
          <ul className={listClasses}>
            <li>Platform identifiers (user ID, username, display name)</li>
            <li>Profile images provided by the platform</li>
            <li>Encrypted OAuth access and refresh tokens</li>
            <li>Token scopes and expiration dates</li>
          </ul>
          <p className="text-sm text-text-dim">
            Legal basis: Art.&nbsp;6(1)(b) DSGVO &ndash; performance of a contract (providing the
            service you signed up for).
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">2.2 Chat Data</h3>
          <p>For active overlays we temporarily process:</p>
          <ul className={listClasses}>
            <li>Messages flowing through connected channels</li>
            <li>Message metadata (timestamps, emotes, badges, highlights)</li>
            <li>Per-message author details (display name, color, avatar)</li>
          </ul>
          <p className="text-sm text-text-dim">
            Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and{' '}
            <strong className="text-text">automatically deleted after one hour</strong>. Messages
            displayed in the overlay are streamed through memory and are not persisted once the
            overlay session ends.
          </p>
          <p className="text-sm text-text-dim">
            Legal basis: Art.&nbsp;6(1)(b) DSGVO (service delivery) and Art.&nbsp;6(1)(f) DSGVO
            (legitimate interest in abuse prevention).
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">2.3 Overlay Configuration</h3>
          <p>To render and sync overlays we store:</p>
          <ul className={listClasses}>
            <li>Overlay names, IDs, and custom CSS or theme settings</li>
            <li>Connected chat sources (platform, channel ID, channel name)</li>
            <li>Filter settings (blocked words, user-level rules)</li>
          </ul>
          <p className="text-sm text-text-dim">Legal basis: Art.&nbsp;6(1)(b) DSGVO.</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">2.4 Cross-Platform Viewer Identity</h3>
          <p>
            If you choose to link multiple platform accounts as a viewer (e.g.&nbsp;Twitch +
            YouTube), we create a unified viewer profile that associates your platform identities.
            This is{' '}
            <strong className="text-text">
              opt-in only and requires your explicit action
            </strong>
            .
          </p>
          <p className="text-sm text-text-dim">
            Legal basis: Art.&nbsp;6(1)(a) DSGVO &ndash; your consent. You can unlink platforms at
            any time from the viewer settings.
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">2.5 Usage &amp; Log Data</h3>
          <p>For observability and abuse prevention we log:</p>
          <ul className={listClasses}>
            <li>
              Anonymised IP address (last octet zeroed), browser user agent, and basic request info
            </li>
            <li>API usage metrics, cache hits, and connection status</li>
            <li>Error traces and diagnostic logs (retained up to 90 days)</li>
          </ul>
          <p className="text-sm text-text-dim">
            Legal basis: Art.&nbsp;6(1)(f) DSGVO &ndash; legitimate interest in maintaining service
            security and availability.
          </p>
        </div>
      </section>

      {/* --- How We Use --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">3. How We Use Your Information</h2>
        <p>Everything we store directly supports the core overlay experience:</p>
        <ul className={listClasses}>
          <li>Authenticate against the platforms you authorize</li>
          <li>Fetch live chat messages and normalize them into a single feed</li>
          <li>Render overlays, perform emote lookups, and respect your filters</li>
          <li>Monitor service reliability and debug incidents</li>
          <li>Detect and prevent abuse (rate-limiting, bans)</li>
        </ul>
      </section>

      {/* --- Storage & Security --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">4. Data Storage &amp; Security</h2>
        <h3 className="text-lg font-semibold text-text">4.1 Storage Locations</h3>
        <ul className={listClasses}>
          <li>PostgreSQL for account data, overlays, and encrypted OAuth tokens</li>
          <li>Redis for ephemeral sessions, message fan-out, and rate limiting</li>
        </ul>
        <h3 className="text-lg font-semibold text-text">4.2 Safeguards</h3>
        <ul className={listClasses}>
          <li>OAuth tokens encrypted with AES-GCM before touching the database</li>
          <li>HTTPS at the ingress layer for all external traffic; internal services isolated via Kubernetes network policies</li>
          <li>Role-scoped infrastructure access and audit logging</li>
          <li>Regular dependency upgrades and security patching</li>
          <li>Security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy)</li>
        </ul>
        <p className="text-sm text-text-dim">
          No storage system is perfectly secure, but we follow industry best practices to keep your
          tokens and overlays safe.
        </p>
      </section>

      {/* --- Third Parties --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          5. Data Sharing &amp; Third Parties
        </h2>

        <h3 className="text-lg font-semibold text-text">5.1 Streaming Platform APIs</h3>
        <p>We connect to the following services to deliver the core product:</p>
        <ul className={listClasses}>
          <li>Twitch IRC and Helix APIs</li>
          <li>YouTube Live Chat and OAuth APIs</li>
          <li>TikTok Live APIs and Kick APIs</li>
          <li>Discord Gateway API</li>
          <li>7TV, BTTV, FFZ for emote metadata</li>
        </ul>
        <p>
          Every integration remains subject to the platform&apos;s own policies and scopes you
          approve.
        </p>

        <h3 className="text-lg font-semibold text-text">5.2 Fonts (Self-Hosted)</h3>
        <p>
          Typography assets originally distributed by Google Fonts are{' '}
          <strong className="text-text">self-hosted on our infrastructure</strong>. The fonts used
          by the All-Chat interface are bundled at build time via Next.js, and fonts selectable for
          overlay customization are served through a server-side proxy at{' '}
          <code className="rounded bg-surface-2 px-1 py-0.5 text-xs">/api/fonts/*</code>. Your
          browser only connects to the All-Chat origin; <strong className="text-text">no IP
          address, user agent, or request metadata is transmitted to Google</strong> when fonts
          are loaded. This aligns with the Landgericht M&uuml;nchen I ruling on Google Fonts (20
          January 2022, Az. 3 O 17493/20).
        </p>
        <p className="text-sm text-text-dim">
          Legal basis: Art.&nbsp;6(1)(f) DSGVO &ndash; legitimate interest in delivering the
          visual appearance of the overlay. The font files themselves are licensed under the SIL
          Open Font License 1.1 or Apache 2.0.
        </p>

        <h3 className="text-lg font-semibold text-text">5.3 Third-Party Frontend Resources</h3>
        <p>
          Overlay and dashboard pages may load the following external resources. Each request
          transmits your <strong className="text-text">IP address</strong> and browser user agent
          to the respective provider:
        </p>
        <ul className={listClasses}>
          <li>
            <strong className="text-text">UI Avatars</strong> (ui-avatars.com) &ndash; generates
            fallback avatar images from display names when a platform avatar is unavailable
          </li>
          <li>
            <strong className="text-text">GitHub API</strong> (api.github.com) &ndash; fetches
            community themes from our public repository in the theme marketplace
          </li>
        </ul>
        <p className="text-sm text-text-dim">
          Legal basis: Art.&nbsp;6(1)(f) DSGVO &ndash; legitimate interest in providing a
          functional and visually complete overlay experience. You can avoid loading these by not
          opening the theme marketplace and by providing a custom avatar URL.
        </p>

        <h3 className="text-lg font-semibold text-text">5.4 YouTube-Specific Notice</h3>
        <p className="font-semibold text-text">
          Your use of All-Chat&apos;s YouTube integration is also governed by the{' '}
          <a
            href="https://policies.google.com/privacy"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Google Privacy Policy
          </a>
          . This applies only to the YouTube API integration (data flows described in Section 5.1);
          it does <strong className="text-text">not</strong> apply to fonts, which are self-hosted
          as described in Section 5.2.
        </p>

        <h3 className="text-lg font-semibold text-text">5.5 No Data Sales</h3>
        <p className="text-sm text-text-dim">
          We never sell or rent your data. We may disclose information when required by law or to
          respond to legitimate security incidents.
        </p>

        <h3 className="text-lg font-semibold text-text">
          5.6 Usage Analytics (Self-Hosted, Cookieless)
        </h3>
        <p>
          To understand how the site is used and where to improve it, we run{' '}
          <a
            href="https://umami.is"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Umami
          </a>
          , an open-source analytics tool that we{' '}
          <strong className="text-text">host ourselves</strong>. It is{' '}
          <strong className="text-text">cookieless</strong>: it sets no cookies, creates no
          persistent identifier, and performs no cross-site or cross-device tracking. The data is
          processed on our own infrastructure and is{' '}
          <strong className="text-text">not shared with any third party</strong> (unlike Google
          Analytics or similar services).
        </p>
        <p>For each page view it records aggregate, non-identifying information:</p>
        <ul className={listClasses}>
          <li>The page you visited and the referring URL</li>
          <li>Browser, operating system, device type, and screen size</li>
          <li>Approximate country, derived from your IP address at request time</li>
        </ul>
        <p className="text-sm text-text-dim">
          Your <strong className="text-text">IP address is not stored</strong>: it is used only
          momentarily to derive the country and to generate a daily, salted hash for counting unique
          visits, after which it is discarded. We do{' '}
          <strong className="text-text">not</strong> track public overlay views (the pages OBS loads
          as a browser source). You can block the analytics script with any browser content blocker
          without affecting the site.
        </p>
        <p className="text-sm text-text-dim">
          Because the tracker stores no information on, and reads none from, your device, it does not
          require consent under &sect;&nbsp;25 TTDSG; the processing of the resulting data rests on
          Art.&nbsp;6(1)(f) DSGVO &ndash; our legitimate interest in measuring and improving the
          service.
        </p>
      </section>

      {/* --- Data Retention --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">6. Data Retention</h2>
        <ul className={listClasses}>
          <li>
            <strong className="text-text">Account &amp; overlay data:</strong> kept until you delete
            your account
          </li>
          <li>
            <strong className="text-text">OAuth tokens:</strong> deleted when you disconnect a
            platform, when they expire (cleaned up after 7 days), or when you delete your account
          </li>
          <li>
            <strong className="text-text">Chat messages sent through All-Chat:</strong> automatically
            deleted after 1 hour
          </li>
          <li>
            <strong className="text-text">Chat messages displayed in overlays:</strong> streamed
            through memory only; not persisted
          </li>
          <li>
            <strong className="text-text">Usage logs:</strong> retained for up to 90 days
          </li>
          <li>
            <strong className="text-text">YouTube quota audit logs:</strong> retained for 30 days
          </li>
        </ul>
      </section>

      {/* --- Your Rights --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          7. Your Rights (Art.&nbsp;15&ndash;21 DSGVO)
        </h2>
        <p>You can exercise the following at any time:</p>
        <ul className={listClasses}>
          <li>
            <strong className="text-text">Access (Art.&nbsp;15):</strong> Request a copy of the data
            we store about you
          </li>
          <li>
            <strong className="text-text">Rectification (Art.&nbsp;16):</strong> Correct inaccurate
            information
          </li>
          <li>
            <strong className="text-text">Erasure (Art.&nbsp;17):</strong> Delete your account and
            all associated data from the Settings page or by contacting us
          </li>
          <li>
            <strong className="text-text">Restriction (Art.&nbsp;18):</strong> Request restriction
            of processing in certain circumstances
          </li>
          <li>
            <strong className="text-text">Data portability (Art.&nbsp;20):</strong> Export your data
            in a machine-readable format via the Settings page
          </li>
          <li>
            <strong className="text-text">Objection (Art.&nbsp;21):</strong> Object to processing
            based on legitimate interest
          </li>
          <li>
            <strong className="text-text">Withdraw consent:</strong> Disconnect platforms or unlink
            viewer identities at any time
          </li>
        </ul>
        <p>
          Contact us at{' '}
          <a
            href="mailto:all.chat.support@gmail.com"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            all.chat.support@gmail.com
          </a>{' '}
          or use the Settings page. You also have the right to lodge a complaint with your
          supervisory authority (Aufsichtsbeh&ouml;rde).
        </p>
        <p className="font-semibold text-text">
          For YouTube Data: You can revoke All-Chat&apos;s access to your YouTube data via the{' '}
          <a
            href="https://myaccount.google.com/connections?filters=3,4&hl=en"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Google security settings page
          </a>
          .
        </p>
      </section>

      {/* --- Cookies & Browser Storage --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          8. Cookies, Browser Storage &amp; Analytics
        </h2>
        <p>
          All-Chat uses <strong className="text-text">browser localStorage</strong> (not cookies)
          for essential functionality:
        </p>
        <ul className={listClasses}>
          <li>Authentication tokens (JWT) to keep you logged in</li>
          <li>User preferences and last-visited state</li>
        </ul>
        <p>
          We do not use advertising cookies or cross-site tracking. We do use{' '}
          <strong className="text-text">privacy-friendly, cookieless usage analytics</strong>{' '}
          (self-hosted Umami) &ndash; it sets nothing on your device and stores no personal
          identifier; see Section&nbsp;5.6 for the full description. Fonts are self-hosted
          (Section&nbsp;5.2) and do not set cookies. The third-party resources listed in
          Section&nbsp;5.3 (UI Avatars, GitHub API) may cause the respective providers to set their
          own cookies when loaded.
        </p>
      </section>

      {/* --- Children --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">9. Children&apos;s Privacy</h2>
        <p>
          All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data
          processing in Germany). If we discover data belonging to a minor we will delete it
          immediately.
        </p>
      </section>

      {/* --- International Transfers --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">10. International Data Transfers</h2>
        <p>
          When you use streaming platform integrations, data is transferred to servers operated by
          Twitch (Amazon, US), Google/YouTube (US), TikTok (various), and Kick (AU). These
          transfers are necessary to perform the service you requested (Art.&nbsp;49(1)(b) DSGVO).
          The GitHub API (theme marketplace) may also involve transfers to the US. Fonts are
          self-hosted on our infrastructure and therefore do not involve any third-country
          transfer when loaded.
        </p>
      </section>

      {/* --- Updates --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">11. Updates</h2>
        <p>
          We&apos;ll post updates to this page when the policy changes and include a new{' '}
          <em>Last Updated</em> date. Significant changes will be announced inside the dashboard.
        </p>
      </section>

      {/* --- Open Source --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">12. Transparency &amp; Open Source</h2>
        <ul className={listClasses}>
          <li>All source code is publicly auditable under AGPL-3.0</li>
          <li>
            No hidden tracking &mdash; our analytics are cookieless and documented (Section&nbsp;5.6),
            and the whole stack is verifiable on GitHub
          </li>
          <li>Privacy practices are open to community scrutiny</li>
        </ul>
        <p>
          Source:{' '}
          <Link
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            github.com/caesarakalaeii/all-chat
          </Link>
        </p>
      </section>

      {/* --- Platform-Specific --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">13. Platform-Specific Notes</h2>

        <h3 className="text-lg font-semibold text-text">Twitch</h3>
        <p>
          We connect to Twitch IRC to receive chat messages. We do not access channel analytics,
          subscriber information, or payment data.
        </p>

        <h3 className="text-lg font-semibold text-text">YouTube</h3>
        <p>
          We use the YouTube Live Chat API. We do not access video content, channel analytics, or
          subscriber data. API usage is subject to YouTube&apos;s quota limits. You can revoke
          access via{' '}
          <a
            href="https://myaccount.google.com/connections?filters=3,4&hl=en"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Google security settings
          </a>{' '}
          or the All-Chat Settings page. Disconnecting deletes your stored OAuth tokens.
        </p>

        <h3 className="text-lg font-semibold text-text">TikTok</h3>
        <p>
          We access live stream chat data only. We do not access your videos, followers, or other
          personal content. Revoke access through TikTok app settings or All-Chat settings.
        </p>

        <h3 className="text-lg font-semibold text-text">Kick</h3>
        <p>
          We connect via WebSocket to receive live chat. We do not access channel analytics or
          payment data.
        </p>

        <h3 className="text-lg font-semibold text-text">Discord</h3>
        <p>
          When you connect a Discord server, we store the guild ID and name. Chat relay uses
          webhook URLs you configure. We do not access server member lists or DMs.
        </p>
      </section>

      {/* --- Contact --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">14. Contact</h2>
        <p>
          Email:{' '}
          <a
            href="mailto:all.chat.support@gmail.com"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            all.chat.support@gmail.com
          </a>
        </p>
        <p className="text-sm text-text-dim">
          This contact is for users of the official hosted service at{' '}
          <strong className="text-text">allch.at</strong>. Self-hosted installations should contact
          their own administrator.
        </p>
      </section>
    </LegalLayout>
  )
}
