import Link from 'next/link'
import LegalLayout from '@/components/legal/LegalLayout'

export const metadata = {
  title: 'Privacy Policy | All-Chat',
  description: 'Learn how All-Chat collects, processes, and protects your information.',
}

const listClasses = 'list-disc pl-6 space-y-1 text-text-sub'

export default function PrivacyPolicyPage() {
  return (
    <LegalLayout title="Privacy Policy" lastUpdated="April 12, 2026">
      <div className="space-y-4">
        <div className="rounded-xl border border-twitch/20 bg-twitch/5 p-5 text-text-sub">
          <strong className="text-text">TL;DR:</strong> All-Chat only collects the information we
          need to authenticate with your streaming platforms and render chat in your overlays. Tokens
          are encrypted, chat messages are processed in-memory, and we never sell your data.
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

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">1. Information We Collect</h2>

        <div>
          <h3 className="text-lg font-semibold text-text">1.1 Authentication Information</h3>
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
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">1.2 Chat Data</h3>
          <p>For active overlays we temporarily process:</p>
          <ul className={listClasses}>
            <li>Messages flowing through connected channels</li>
            <li>Message metadata (timestamps, emotes, badges, highlights)</li>
            <li>Per-message author details (display name, color, avatar)</li>
          </ul>
          <p className="text-sm text-text-dim">
            Chat is streamed through memory and never written to disk once an overlay session ends.
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">1.3 Overlay Configuration</h3>
          <p>To render and sync overlays we store:</p>
          <ul className={listClasses}>
            <li>Overlay names, IDs, and custom CSS or theme settings</li>
            <li>Connected chat sources (platform, channel ID, channel name)</li>
            <li>Filter settings (blocked words, user-level rules)</li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">1.4 Usage Data</h3>
          <p>For observability and abuse prevention we log:</p>
          <ul className={listClasses}>
            <li>IP address, browser user agent, and basic request info</li>
            <li>API usage metrics, cache hits, and connection status</li>
            <li>Error traces and diagnostic logs (retained up to 90 days)</li>
          </ul>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">2. How We Use Your Information</h2>
        <p>Everything we store directly supports the core overlay experience:</p>
        <ul className={listClasses}>
          <li>Authenticate against the platforms you authorize</li>
          <li>Fetch live chat messages and normalize them into a single feed</li>
          <li>Render overlays, perform emote lookups, and respect your filters</li>
          <li>Monitor service reliability and debug incidents</li>
          <li>Contact you regarding service-impacting updates</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">3. Data Storage &amp; Security</h2>
        <h3 className="text-lg font-semibold text-text">3.1 Storage Locations</h3>
        <ul className={listClasses}>
          <li>PostgreSQL for account data, overlays, and OAuth tokens</li>
          <li>Redis for ephemeral sessions, message fan-out, and rate limiting</li>
        </ul>
        <h3 className="text-lg font-semibold text-text">3.2 Safeguards</h3>
        <ul className={listClasses}>
          <li>OAuth tokens encrypted with AES-GCM before touching the database</li>
          <li>HTTPS for all external traffic via TLS termination at the ingress layer</li>
          <li>Kubernetes network policies restricting internal service-to-service communication</li>
          <li>Role-scoped infrastructure access and audit logging</li>
          <li>Regular dependency upgrades and security patching</li>
        </ul>
        <p className="text-sm text-text-dim">
          No storage system is perfectly secure, but we follow industry best practices to keep your
          tokens and overlays safe.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">4. Data Sharing &amp; Third Parties</h2>
        <p>We only talk to the services that power your overlays:</p>
        <ul className={listClasses}>
          <li>Twitch IRC and Helix APIs</li>
          <li>YouTube internal APIs and OAuth APIs</li>
          <li>TikTok Live via unofficial reverse-engineered WebSocket library (beta)</li>
          <li>Kick Pusher WebSocket APIs</li>
          <li>Discord Gateway APIs</li>
          <li>7TV, BTTV, FFZ for emote metadata</li>
        </ul>
        <p>
          Every integration remains subject to the platform&apos;s own policies and scopes you
          approve.
        </p>
        <p className="font-semibold text-text">
          YouTube Integration: YouTube chat is fetched using YouTube&apos;s internal APIs (InnerTube).
          Your use of All-Chat&apos;s YouTube integration is also governed by the{' '}
          <a
            href="http://www.google.com/policies/privacy"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Google Privacy Policy
          </a>
          .
        </p>
        <p className="text-sm text-text-dim">
          We never sell or rent your data, but we may disclose information when required by law or
          to respond to legitimate security incidents.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">5. Data Retention</h2>
        <ul className={listClasses}>
          <li>
            <strong className="text-text">Account &amp; overlay data:</strong> kept until you delete
            your account
          </li>
          <li>
            <strong className="text-text">OAuth tokens:</strong> deleted when you disconnect a
            platform or when they expire
          </li>
          <li>
            <strong className="text-text">Chat messages:</strong> never written to disk; cleared
            after sessions end
          </li>
          <li>
            <strong className="text-text">Usage logs:</strong> retained for up to 90 days
          </li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">6. Your Rights</h2>
        <p>You can exercise the following at any time:</p>
        <ul className={listClasses}>
          <li>Access a copy of the data we store about you</li>
          <li>Correct inaccurate information</li>
          <li>Disconnect platforms or delete your account from the Settings page</li>
          <li>Export overlay configuration JSON</li>
          <li>
            Contact us at{' '}
            <a
              href="mailto:all.chat.support@gmail.com"
              className="text-twitch underline decoration-twitch/30 underline-offset-4"
            >
              all.chat.support@gmail.com
            </a>
          </li>
        </ul>
        <p className="text-sm text-text-dim">
          For platform-specific details on revoking access, see Section 10 below.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">7. Cookies &amp; Tracking</h2>
        <p>We keep tracking minimal:</p>
        <ul className={listClasses}>
          <li>Session cookie for JWT token storage</li>
          <li>WebSocket connections for real-time chat delivery</li>
          <li>No ad tech, cross-site tracking, or analytics pixels</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">8. Children&apos;s Privacy</h2>
        <p>
          All-Chat is not intended for children under 13. If we discover data belonging to a minor
          we will delete it immediately.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">9. International Transfers</h2>
        <p>
          Our infrastructure may process information in the United States or other regions. By using
          All-Chat you consent to this processing.
        </p>
      </section>

      <section className="space-y-6">
        <h2 className="text-2xl font-semibold text-text">10. Platform-Specific Privacy Notes</h2>

        <div>
          <h3 className="text-lg font-semibold text-text">TikTok Integration</h3>
          <p>When you add a TikTok source to your overlay:</p>
          <ul className={listClasses}>
            <li>
              We connect to TikTok Live streams using an unofficial, reverse-engineered WebSocket
              library (beta)
            </li>
            <li>No OAuth authentication or TikTok account connection is required</li>
            <li>We receive real-time chat messages during live streams</li>
            <li>
              We do not access TikTok videos, followers, or other personal content beyond live chat
            </li>
            <li>
              You can stop data collection by removing the TikTok source from your overlay in
              All-Chat settings
            </li>
            <li>
              This integration may break without notice if TikTok changes its internal APIs
            </li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">Twitch Integration</h3>
          <p>When you connect your Twitch account:</p>
          <ul className={listClasses}>
            <li>We connect to Twitch IRC to receive chat messages</li>
            <li>We use the Twitch Helix API for user authentication and EventSub webhooks</li>
            <li>
              We do not access your channel analytics, subscriber information, or payment data
            </li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">YouTube Integration</h3>
          <p>When you connect your YouTube account:</p>
          <ul className={listClasses}>
            <li>We use YouTube&apos;s internal APIs (InnerTube) to fetch live chat messages</li>
            <li>We use YouTube OAuth for account authentication</li>
            <li>We do not access your video content, channel analytics, or subscriber data</li>
            <li>
              Your use of All-Chat&apos;s YouTube integration is also governed by the{' '}
              <a
                href="http://www.google.com/policies/privacy"
                target="_blank"
                rel="noopener noreferrer"
                className="text-twitch underline decoration-twitch/30 underline-offset-4"
              >
                Google Privacy Policy
              </a>
            </li>
          </ul>
          <p>
            <strong className="text-text">Revoking YouTube Access:</strong> You can revoke
            All-Chat&apos;s access to your YouTube data at any time by visiting the{' '}
            <a
              href="https://myaccount.google.com/connections?filters=3,4&hl=en"
              target="_blank"
              rel="noopener noreferrer"
              className="text-twitch underline decoration-twitch/30 underline-offset-4"
            >
              Google security settings page
            </a>{' '}
            or by disconnecting YouTube from the All-Chat Settings page.
          </p>
          <p className="text-sm text-text-dim">
            Disconnecting YouTube deletes your stored OAuth tokens. Chat messages are processed in
            real-time and not stored permanently.
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">Kick Integration</h3>
          <p>When you add a Kick source to your overlay:</p>
          <ul className={listClasses}>
            <li>We connect to Kick chat channels via the Pusher WebSocket API</li>
            <li>We receive real-time chat messages from the specified channels</li>
            <li>
              We do not access your Kick account data, channel analytics, or subscriber information
            </li>
            <li>You can stop data collection by removing the Kick source from your overlay</li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">Discord Integration</h3>
          <p>When you add a Discord source to your overlay:</p>
          <ul className={listClasses}>
            <li>We connect to Discord channels via the Discord Gateway API using a bot</li>
            <li>We receive real-time messages from configured Discord channels</li>
            <li>
              We do not access your direct messages, server settings, or member lists beyond what is
              needed for chat relay
            </li>
            <li>
              You can stop data collection by removing the Discord source from your overlay or
              removing the bot from your server
            </li>
          </ul>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">11. Updates</h2>
        <p>
          We&apos;ll post updates to this page when the policy changes and include a new{' '}
          <em>Last Updated</em> date. Significant changes will be announced inside the dashboard.
        </p>
        <p>
          Need more details? Read the full document in{' '}
          <Link
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            our repository
          </Link>{' '}
          or reach out to{' '}
          <a
            href="mailto:all.chat.support@gmail.com"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            all.chat.support@gmail.com
          </a>
          .
        </p>
      </section>
    </LegalLayout>
  )
}
