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
      "script-src 'self' 'unsafe-inline'",
      "style-src 'self' 'unsafe-inline'",
      "connect-src 'self' wss: ws: https:",
      "font-src 'self' data:",
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
    ]

    return [
      {
        // Public overlay route: prevent third-party iframing.
        source: '/overlay/:id*',
        headers: [
          ...hardening,
          { key: 'Content-Security-Policy', value: [...cspBase, "frame-ancestors 'none'"].join('; ') },
          { key: 'X-Frame-Options', value: 'DENY' },
        ],
      },
      {
        // Editor embed (same-origin iframe) + everything else.
        source: '/:path*',
        headers: [
          ...hardening,
          { key: 'Content-Security-Policy', value: [...cspBase, "frame-ancestors 'self'"].join('; ') },
          { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
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
