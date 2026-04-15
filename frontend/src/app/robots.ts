import type { MetadataRoute } from 'next'

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: '*',
        allow: ['/', '/legal/', '/chat/'],
        disallow: [
          '/dashboard/',
          '/admin/',
          '/auth/',
          '/settings/',
          '/overlays/',
          '/overlay/',
          '/api/',
        ],
      },
    ],
    sitemap: 'https://allch.at/sitemap.xml',
  }
}
