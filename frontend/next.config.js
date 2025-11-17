/** @type {import('next').NextConfig} */
const nextConfig = {
  // Use standalone output for Docker
  output: 'standalone',

  // Strict mode for better development
  reactStrictMode: true,

  // Image optimization
  images: {
    unoptimized: true,
    domains: [
      'static-cdn.jtvnw.net', // Twitch CDN
      'yt3.ggpht.com', // YouTube avatars
      'cdn.7tv.app', // 7TV emotes
      'cdn.betterttv.net', // BTTV emotes
      'cdn.frankerfacez.com', // FFZ emotes
      'ui-avatars.com' // Generated avatar fallbacks
    ]
  },

  // API rewrites for local development only
  // In production, Nginx handles /api/* and /ws/* proxying
  async rewrites() {
    // Only apply rewrites in development mode
    if (process.env.NODE_ENV === 'development') {
      return [
        {
          source: '/api/:path*',
          destination: 'http://localhost:8080/api/:path*'
        },
        {
          source: '/ws/:path*',
          destination: 'http://localhost:8080/ws/:path*'
        }
      ];
    }
    return [];
  }
};

module.exports = nextConfig;
