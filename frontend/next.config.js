/** @type {import('next').NextConfig} */
const withBundleAnalyzer = require('@next/bundle-analyzer')({
  enabled: process.env.ANALYZE === 'true',
})

const nextConfig = {
  // Use standalone output for Docker
  output: 'standalone',

  // Strict mode for better development
  reactStrictMode: true,

  // Generate unique build ID to prevent Server Action mismatches
  generateBuildId: async () => {
    // Use git commit hash if available, otherwise use timestamp
    return (
      process.env.NEXT_PUBLIC_GIT_COMMIT ||
      process.env.NEXT_PUBLIC_BUILD_DATE ||
      `build-${Date.now()}`
    )
  },

  // Image optimization
  images: {
    unoptimized: true,
    domains: [
      'static-cdn.jtvnw.net', // Twitch CDN
      'yt3.ggpht.com', // YouTube avatars
      'cdn.7tv.app', // 7TV emotes
      'cdn.betterttv.net', // BTTV emotes
      'cdn.frankerfacez.com', // FFZ emotes
      'files.kick.com', // Kick emotes
      'ui-avatars.com', // Generated avatar fallbacks
      'cdn.discordapp.com', // Discord guild icons
    ],
  },

  // Redirects for paths without a real page. /legal has no index route
  // (only /legal/privacy|terms|impressum), so /legal and /legal/ otherwise
  // dead-end on a 404. Send them to the privacy page.
  async redirects() {
    return [
      {
        source: '/legal',
        destination: '/legal/privacy',
        permanent: true,
      },
    ]
  },

  // Security headers (M10). Per-route CSP + standard hardening. Framing policy
  // is tiered (frame-ancestors is the authoritative control; browsers ignore
  // X-Frame-Options whenever frame-ancestors is present, CSP L2 §"Relation to
  // X-Frame-Options"):
  //   - app + editor SplitView embed  → frame-ancestors 'self' (same-origin)
  //   - all /overlay/* by default     → frame-ancestors 'none' (locked; keeps
  //     the interactive routes — /overlay/:id/participate (viewer login +
  //     points) and /overlay/:id/view (authenticated monitor) — un-framable,
  //     and any future overlay sub-route locked-by-default)
  //   - display-only OBS widgets      → frame-ancestors * (embeddable anywhere:
  //     OBS, Streamlabs, personal sites, third-party dashboards). These carry
  //     no auth or viewer identity, so clickjacking risk is negligible.
  // TODO(prod): replace script-src 'unsafe-inline' with a per-request nonce
  // (Next.js unstable_inline + generateNonce) once CSP-nonce middleware is in
  // place; 'unsafe-inline' is required now for Next dev + inline RSC chunks.
  async headers() {
    const cspBase = [
      "default-src 'self'",
      "img-src 'self' data: https: static-cdn.jtvnw.net yt3.ggpht.com cdn.7tv.app cdn.betterttv.net cdn.frankerfacez.com files.kick.com ui-avatars.com cdn.discordapp.com",
      // embed.twitch.tv hosts the Twitch Embed SDK used by the credits/clips
      // overlay route (audit #3); without it the SDK script is CSP-blocked and
      // clips never play. analytics.allch.at hosts the self-hosted Umami tracker
      // (components/Analytics.tsx); without it the tracker script is CSP-blocked
      // and no page views or custom events are recorded.
      "script-src 'self' 'unsafe-inline' https://embed.twitch.tv https://analytics.allch.at",
      // worker-src governs Web Worker/SharedWorker creation. The overlay
      // editor's self-hosted Monaco (/monaco/vs, ADR-0040) runs its CSS
      // language services — validation, autocomplete — in workers that Monaco
      // instantiates from same-origin blob: URLs. Without an explicit
      // worker-src these fall back to script-src (no blob:) and are blocked,
      // so the editor loads but its language features silently die. 'self'
      // covers direct same-origin workers; blob: covers Monaco's blob workers
      // (blobs are built by same-origin script, so this is far narrower than a
      // third-party script-src host).
      "worker-src 'self' blob:",
      "style-src 'self' 'unsafe-inline'",
      "connect-src 'self' wss: ws: https:",
      // media-src governs <video>/<audio> sources. Chat message video attachments
      // (Discord uploads, Tenor/Giphy link previews) are served from third-party
      // https hosts; without this they fall back to default-src 'self' and are
      // blocked. img-src already permits https: for image/GIF attachments.
      "media-src 'self' https: data: blob:",
      "font-src 'self' data:",
      // frame-src governs which iframes a page may embed. It MUST list 'self' (the
      // editor's same-origin /overlays/:id/preview/embed iframe) plus the Twitch
      // player domains the Embed SDK injects (audit #3) — once frame-src is
      // specified it replaces the default-src 'self' fallback for frames.
      "frame-src 'self' https://embed.twitch.tv https://player.twitch.tv https://www.twitch.tv https://clips.twitch.tv",
      "object-src 'none'",
      "base-uri 'self'",
      "form-action 'self'",
    ]

    const hardening = [
      { key: 'X-Content-Type-Options', value: 'nosniff' },
      { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
      {
        key: 'Permissions-Policy',
        value: 'camera=(), microphone=(), geolocation=(), interest-cohort=()',
      },
      // HSTS is honored by browsers only over HTTPS; safe to emit in dev (ignored on http).
      { key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
      // Legacy framing fallback. CSP frame-ancestors is the authoritative defense
      // (set per-route below); this covers older browsers that ignore CSP.
      // Overridden to DENY on non-embeddable overlay routes (audit L6 —
      // previously both DENY and SAMEORIGIN were emitted for overlay paths
      // because the catch-all also matched /overlay/:id*).
      { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
    ]

    const csp = (frameAncestors) => [...cspBase, `frame-ancestors ${frameAncestors}`].join('; ')

    // Display-only OBS browser-source widgets. Public, no auth, no viewer
    // identity — safe to embed anywhere. Single-segment `:id` (no `*`) so these
    // match ONLY the exact widget route and never the interactive sub-routes
    // (participate/view), which stay locked by the /overlay/:id* rule above.
    const embeddableWidgets = [
      '/overlay/:id', // OBS chat overlay
      '/overlay/:id/poll', // poll widget (aggregate only)
      '/overlay/:id/prediction', // prediction widget (aggregate only)
      '/overlay/:id/credits', // end-of-stream credit roll
    ]

    return [
      {
        // Editor embed (same-origin iframe) + everything else. X-Frame-Options
        // SAMEORIGIN comes from the shared hardening array.
        source: '/:path*',
        headers: [...hardening, { key: 'Content-Security-Policy', value: csp("'self'") }],
      },
      {
        // Lock every overlay route by default (participate/view + anything added
        // later). Listed AFTER the catch-all so its stricter headers override
        // for /overlay/* paths (audit L6 — previously both routes set
        // X-Frame-Options, emitting conflicting DENY + SAMEORIGIN). The explicit
        // DENY overrides the hardening default (SAMEORIGIN) within this entry.
        source: '/overlay/:id*',
        headers: [
          ...hardening,
          { key: 'Content-Security-Policy', value: csp("'none'") },
          { key: 'X-Frame-Options', value: 'DENY' },
        ],
      },
      // Re-open the display-only widgets. Listed LAST so their headers override
      // the /overlay/:id* lockdown per matching path. frame-ancestors * makes
      // them embeddable; X-Frame-Options is reset to SAMEORIGIN (from the shared
      // hardening) so the legacy header no longer hard-blocks — modern browsers
      // ignore it anyway because frame-ancestors is present.
      ...embeddableWidgets.map((source) => ({
        source,
        headers: [...hardening, { key: 'Content-Security-Policy', value: csp('*') }],
      })),
    ]
  },

  // API rewrites - proxy to API Gateway
  // In development: localhost:8080
  // In production: api-gateway service (Docker/K8s networking)
  async rewrites() {
    const apiGatewayURL =
      process.env.NODE_ENV === 'production'
        ? process.env.API_GATEWAY_URL || 'http://api-gateway:8080'
        : 'http://localhost:8080'

    return [
      {
        source: '/api/:path*',
        destination: `${apiGatewayURL}/api/:path*`,
      },
      {
        source: '/ws/:path*',
        destination: `${apiGatewayURL}/ws/:path*`,
      },
    ]
  },
}

module.exports = withBundleAnalyzer(nextConfig)
