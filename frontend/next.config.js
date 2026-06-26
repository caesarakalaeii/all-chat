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

  // Security headers (M10). Per-route CSP + standard hardening. The overlay
  // public route (/overlay/:id) gets frame-ancestors 'none' so it cannot be
  // iframed by third parties; everything else allows 'self' framing (editor
  // SplitView embeds /overlays/:id/preview/embed same-origin).
  // TODO(prod): replace script-src 'unsafe-inline' with a per-request nonce
  // (Next.js unstable_inline + generateNonce) once CSP-nonce middleware is in
  // place; 'unsafe-inline' is required now for Next dev + inline RSC chunks.
  async headers() {
    const cspBase = [
      "default-src 'self'",
      "img-src 'self' data: https: static-cdn.jtvnw.net yt3.ggpht.com cdn.7tv.app cdn.betterttv.net cdn.frankerfacez.com files.kick.com ui-avatars.com cdn.discordapp.com",
      // embed.twitch.tv hosts the Twitch Embed SDK used by the credits/clips
      // overlay route (audit #3); without it the SDK script is CSP-blocked and
      // clips never play.
      "script-src 'self' 'unsafe-inline' https://embed.twitch.tv",
      "style-src 'self' 'unsafe-inline'",
      "connect-src 'self' wss: ws: https:",
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
      { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=(), interest-cohort=()' },
      // HSTS is honored by browsers only over HTTPS; safe to emit in dev (ignored on http).
      { key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
      // Legacy framing fallback. CSP frame-ancestors is the authoritative defense
      // (set per-route below); this covers older browsers that ignore CSP.
      // Overridden to DENY on non-embeddable overlay routes (audit L6 —
      // previously both DENY and SAMEORIGIN were emitted for overlay paths
      // because the catch-all also matched /overlay/:id*).
      { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
    ]

    return [
      {
        // Editor embed (same-origin iframe) + everything else. X-Frame-Options
        // SAMEORIGIN comes from the shared hardening array.
        source: '/:path*',
        headers: [
          ...hardening,
          { key: 'Content-Security-Policy', value: [...cspBase, "frame-ancestors 'self'"].join('; ') },
        ],
      },
      {
        // Public overlay route: prevent third-party iframing. Listed AFTER the
        // catch-all so its stricter headers override for /overlay/* paths
        // (audit L6 — previously both routes set X-Frame-Options, emitting
        // conflicting DENY + SAMEORIGIN). The explicit DENY below overrides
        // the hardening default (SAMEORIGIN) within this entry.
        source: '/overlay/:id*',
        headers: [
          ...hardening,
          { key: 'Content-Security-Policy', value: [...cspBase, "frame-ancestors 'none'"].join('; ') },
          { key: 'X-Frame-Options', value: 'DENY' },
        ],
      },
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
