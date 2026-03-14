import Link from 'next/link'
import LegalLayout from '@/components/legal/LegalLayout'

export const metadata = {
  title: 'Privacy Policy | All-Chat',
  description: 'Learn how All-Chat collects, processes, and protects your information.',
}

const listClasses = 'list-disc pl-6 space-y-1 text-slate-700'

export default function PrivacyPolicyPage() {
  return (
    <LegalLayout title="Privacy Policy" lastUpdated="November 15, 2025">
      <div className="space-y-4">
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-amber-900">
          <strong>TL;DR:</strong> All-Chat only collects the information we need to authenticate
          with your streaming platforms and render chat in your overlays. Tokens are encrypted, chat
          messages are processed in-memory, and we never sell your data.
        </div>
        <div className="rounded-2xl border border-sky-200 bg-sky-50 p-5 text-sky-900">
          <strong>Open Source Transparency:</strong> All-Chat is licensed under AGPL-3.0. Review the
          entire codebase—including how we store and process your data—on{' '}
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold underline decoration-sky-300 underline-offset-4"
          >
            GitHub
          </a>
          .
        </div>
      </div>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">1. Information We Collect</h2>

        <div>
          <h3 className="text-lg font-semibold text-slate-800">1.1 Authentication Information</h3>
          <p>
            When you connect Twitch, YouTube, TikTok, or Kick we store the minimum data required to
            create overlays and reconnect later:
          </p>
          <ul className={listClasses}>
            <li>Platform identifiers (user ID, username, display name)</li>
            <li>Profile images provided by the platform</li>
            <li>Encrypted OAuth access and refresh tokens</li>
            <li>Token scopes and expiration dates</li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-slate-800">1.2 Chat Data</h3>
          <p>For active overlays we temporarily process:</p>
          <ul className={listClasses}>
            <li>Messages flowing through connected channels</li>
            <li>Message metadata (timestamps, emotes, badges, highlights)</li>
            <li>Per-message author details (display name, color, avatar)</li>
          </ul>
          <p className="text-sm text-slate-500">
            Chat is streamed through memory and never written to disk once an overlay session ends.
          </p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-slate-800">1.3 Overlay Configuration</h3>
          <p>To render and sync overlays we store:</p>
          <ul className={listClasses}>
            <li>Overlay names, IDs, and custom CSS or theme settings</li>
            <li>Connected chat sources (platform, channel ID, channel name)</li>
            <li>Filter settings (blocked words, user-level rules)</li>
          </ul>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-slate-800">1.4 Usage Data</h3>
          <p>For observability and abuse prevention we log:</p>
          <ul className={listClasses}>
            <li>IP address, browser user agent, and basic request info</li>
            <li>API usage metrics, cache hits, and connection status</li>
            <li>Error traces and diagnostic logs (retained up to 90 days)</li>
          </ul>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">2. How We Use Your Information</h2>
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
        <h2 className="text-2xl font-semibold text-slate-900">3. Data Storage & Security</h2>
        <h3 className="text-lg font-semibold text-slate-800">3.1 Storage Locations</h3>
        <ul className={listClasses}>
          <li>PostgreSQL for account data, overlays, and OAuth tokens</li>
          <li>Redis for ephemeral sessions, message fan-out, and rate limiting</li>
        </ul>
        <h3 className="text-lg font-semibold text-slate-800">3.2 Safeguards</h3>
        <ul className={listClasses}>
          <li>OAuth tokens encrypted with AES-GCM before touching the database</li>
          <li>Strict HTTPS everywhere and signed internal service tokens</li>
          <li>Role-scoped infrastructure access and audit logging</li>
          <li>Regular dependency upgrades and security patching</li>
        </ul>
        <p className="text-sm text-slate-500">
          No storage system is perfectly secure, but we follow industry best practices to keep your
          tokens and overlays safe.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">4. Data Sharing & Third Parties</h2>
        <p>We only talk to the services that power your overlays:</p>
        <ul className={listClasses}>
          <li>Twitch IRC and Helix APIs</li>
          <li>YouTube Live Chat and OAuth APIs</li>
          <li>TikTok Live APIs (beta) and Kick APIs</li>
          <li>7TV, BTTV, FFZ for emote metadata</li>
        </ul>
        <p>
          Every integration remains subject to the platform&apos;s own policies and scopes you
          approve.
        </p>
        <p className="font-semibold text-slate-800">
          YouTube Integration: Your use of All-Chat&apos;s YouTube integration is also governed by
          the{' '}
          <a
            href="http://www.google.com/policies/privacy"
            target="_blank"
            rel="noopener noreferrer"
            className="underline"
          >
            Google Privacy Policy
          </a>
          .
        </p>
        <p className="text-sm text-slate-500">
          We never sell or rent your data, but we may disclose information when required by law or
          to respond to legitimate security incidents.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">5. Data Retention</h2>
        <ul className={listClasses}>
          <li>
            <strong>Account & overlay data:</strong> kept until you delete your account
          </li>
          <li>
            <strong>OAuth tokens:</strong> deleted when you disconnect a platform or when they
            expire
          </li>
          <li>
            <strong>Chat messages:</strong> never written to disk; cleared after sessions end
          </li>
          <li>
            <strong>Usage logs:</strong> retained for up to 90 days
          </li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">6. Your Rights</h2>
        <p>You can exercise the following at any time:</p>
        <ul className={listClasses}>
          <li>Access a copy of the data we store about you</li>
          <li>Correct inaccurate information</li>
          <li>Disconnect platforms or delete your account from the Settings page</li>
          <li>Export overlay configuration JSON</li>
          <li>
            Contact us at{' '}
            <a href="mailto:allchat@caes.ar" className="underline">
              allchat@caes.ar
            </a>
          </li>
        </ul>
        <p className="font-semibold text-slate-800">
          For YouTube Data: You can revoke All-Chat&apos;s access to your YouTube data via the{' '}
          <a
            href="https://myaccount.google.com/connections?filters=3,4&hl=en"
            target="_blank"
            rel="noopener noreferrer"
            className="underline"
          >
            Google security settings page
          </a>
          . Disconnecting YouTube from Settings will delete your OAuth tokens. Chat messages are
          processed in real-time and not stored permanently.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">7. Cookies & Tracking</h2>
        <p>We keep tracking minimal:</p>
        <ul className={listClasses}>
          <li>Session cookie for JWT token storage</li>
          <li>WebSocket connections for real-time chat delivery</li>
          <li>No ad tech, cross-site tracking, or analytics pixels</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">8. Children&apos;s Privacy</h2>
        <p>
          All-Chat is not intended for children under 13. If we discover data belonging to a minor
          we will delete it immediately.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">9. International Transfers</h2>
        <p>
          Our infrastructure may process information in the United States or other regions. By using
          All-Chat you consent to this processing.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-slate-900">10. Updates</h2>
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
            className="underline"
          >
            our repository
          </Link>{' '}
          or reach out to{' '}
          <a href="mailto:allchat@caes.ar" className="underline">
            allchat@caes.ar
          </a>
          .
        </p>
      </section>
    </LegalLayout>
  )
}
