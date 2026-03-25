import Link from 'next/link'
import LegalLayout from '@/components/legal/LegalLayout'

export const metadata = {
  title: 'Terms of Service | All-Chat',
  description: 'Understand the rules and responsibilities for using All-Chat.',
}

const listClasses = 'list-disc pl-6 space-y-1 text-text-sub'

export default function TermsOfServicePage() {
  return (
    <LegalLayout title="Terms of Service" lastUpdated="November 15, 2025">
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">1. Acceptance of Terms</h2>
        <p>
          By accessing or using All-Chat you agree to these Terms of Service. If you disagree with
          any part, you should discontinue use immediately.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">2. Description of Service</h2>
        <p>
          All-Chat aggregates real-time chat from Twitch, YouTube, Kick, and TikTok into a single
          overlay so you can display cross-platform conversation on your stream. You can customize
          overlays, connect sources, and broadcast them via OBS or browser sources.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">3. Accounts &amp; Authentication</h2>
        <p>You are responsible for all activity that happens under your account. You agree to:</p>
        <ul className={listClasses}>
          <li>Provide accurate registration details</li>
          <li>Maintain the security of your credentials and OAuth grants</li>
          <li>
            Notify us at{' '}
            <a
              href="mailto:allchat@caes.ar"
              className="text-twitch underline decoration-twitch/30 underline-offset-4"
            >
              allchat@caes.ar
            </a>{' '}
            if you suspect compromise
          </li>
          <li>
            Comply with the terms of Twitch, YouTube, TikTok, Kick, and any other connected platform
          </li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">4. Acceptable Use</h2>
        <p>You agree not to misuse the Service, including but not limited to:</p>
        <ul className={listClasses}>
          <li>Breaking local, national, or international laws</li>
          <li>Infringing intellectual property or privacy rights of others</li>
          <li>Uploading malware, spam, or malicious code</li>
          <li>Attempting to bypass authentication, rate limits, or security controls</li>
          <li>Harassing or abusing other users</li>
          <li>Using All-Chat in a way that violates partner platform policies</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">5. Third-Party Integrations</h2>
        <p>
          All-Chat relies on third-party APIs. Their availability, scopes, and rate limits may
          change.
        </p>
        <ul className={listClasses}>
          <li>You must comply with each platform&apos;s terms of service</li>
          <li>We are not accountable for outages or policy shifts by those platforms</li>
          <li>Platform-specific quotas can impact overlay functionality</li>
        </ul>
        <p className="font-semibold text-text">
          YouTube Integration: By using All-Chat to connect to YouTube, you agree to be bound by the{' '}
          <a
            href="https://www.youtube.com/t/terms"
            target="_blank"
            rel="noopener noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            YouTube Terms of Service
          </a>
          .
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">6. Privacy</h2>
        <p>
          Your use of All-Chat is also governed by our{' '}
          <Link
            href="/legal/privacy"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            Privacy Policy
          </Link>
          , which explains what we collect and how it is used.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">7. Open Source License</h2>
        <p>
          All-Chat is released under the{' '}
          <a
            href="https://www.gnu.org/licenses/agpl-3.0.html"
            target="_blank"
            rel="noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            GNU Affero General Public License v3.0 (AGPL-3.0)
          </a>
          . That means you may use, study, modify, and distribute the software as long as your
          derivative works also inherit the AGPL-3.0 terms. If you run a modified version of
          All-Chat as a hosted service, you must provide the source to your users.
        </p>
        <p>
          The canonical repository is available on{' '}
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noreferrer"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            GitHub
          </a>
          .
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">8. Availability &amp; Support</h2>
        <p>We aim for high uptime but do not guarantee:</p>
        <ul className={listClasses}>
          <li>Uninterrupted access or zero bugs</li>
          <li>Compatibility with every browser or streamer setup</li>
          <li>Immediate fixes or feature requests</li>
        </ul>
        <p>
          Support for the hosted service at <strong className="text-text">allch.at</strong> is
          best-effort. Community/self-hosted deployments must rely on their own maintainers or the
          open source community for assistance.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">9. Limitation of Liability</h2>
        <p>
          To the fullest extent permitted by law, All-Chat is not liable for indirect, incidental,
          special, or consequential damages arising from your use of the Service.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">10. Indemnification</h2>
        <p>
          You agree to indemnify All-Chat against claims, damages, losses, and expenses arising from
          your use of the Service, violation of these Terms, or infringement of another party&apos;s
          rights.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">11. Changes to Terms</h2>
        <p>
          We may update these Terms over time. Material changes will be announced in the dashboard,
          and the new version will be posted here with an updated effective date.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">12. Termination</h2>
        <p>
          We reserve the right to suspend or terminate your access for any breach of these Terms or
          abusive behavior. You may stop using the Service at any time by deleting your account in
          the Settings page.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">13. Governing Law</h2>
        <p>
          These Terms are governed by applicable laws without regard to conflict-of-law provisions.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">14. Contact</h2>
        <p>
          Questions? Reach us at{' '}
          <a
            href="mailto:allchat@caes.ar"
            className="text-twitch underline decoration-twitch/30 underline-offset-4"
          >
            allchat@caes.ar
          </a>
          . Hosted community forks should contact their own administrators.
        </p>
      </section>
    </LegalLayout>
  )
}
