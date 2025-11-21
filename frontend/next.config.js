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

  // API rewrites - proxy to API Gateway
  // In development: localhost:8080
  // In production: api-gateway service (Docker/K8s networking)
  async rewrites() {
    const apiGatewayURL = process.env.NODE_ENV === 'production'
      ? process.env.API_GATEWAY_URL || 'http://api-gateway:8080'
      : 'http://localhost:8080';

    return [
      {
        source: '/api/:path*',
        destination: `${apiGatewayURL}/api/:path*`
      },
      {
        source: '/ws/:path*',
        destination: `${apiGatewayURL}/ws/:path*`
      }
    ];
  }
};

module.exports = nextConfig;
