/** @type {import('next').NextConfig} */
const nextConfig = {
  // Use standalone output for Docker
  output: 'standalone',

  // Strict mode for better development
  reactStrictMode: true,

  // Image optimization
  images: {
    domains: [
      'static-cdn.jtvnw.net', // Twitch CDN
      'yt3.ggpht.com', // YouTube avatars
      'cdn.7tv.app', // 7TV emotes
      'cdn.betterttv.net', // BTTV emotes
      'cdn.frankerfacez.com' // FFZ emotes
    ]
  },

  // API rewrites for development
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/api/:path*'
      }
    ];
  },

  // Environment variables available to browser
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
  }
};

module.exports = nextConfig;
