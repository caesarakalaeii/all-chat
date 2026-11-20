/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * SEO metadata & structured-data guarantees (search-visibility CI gate).
 *
 * Locks the on-page SEO signals we rely on so a refactor cannot silently drop
 * them: the keyword-led homepage title, the crawlable content pages in the
 * sitemap, Discord + "OBS chat overlay" in the site description, and the
 * HowTo / TechArticle / BreadcrumbList JSON-LD on the docs pages.
 *
 * Follows the repo convention (see token-contrast.test.ts) of parsing source
 * as text rather than importing the page modules, which pull in client
 * components and next/font and do not load cleanly under vitest. The sitemap
 * is a pure module, so it is exercised functionally.
 *
 * The titles and descriptions now live in the i18n catalog (#799), so the
 * assertions about them read messages/en/metadata.ts. A gate must follow the
 * copy it guards: pointed at a file the words have left, it passes for the
 * wrong reason. The JSON-LD assertions still read the page sources, because
 * that is where the structured data is emitted.
 */

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import sitemap from '../sitemap'

const read = (rel: string) => readFileSync(join(__dirname, '..', rel), 'utf-8')
const metadataCatalog = () =>
  readFileSync(join(__dirname, '..', '..', 'lib', 'i18n', 'messages', 'en', 'metadata.ts'), 'utf-8')

describe('sitemap', () => {
  const urls = sitemap().map((entry) => entry.url)

  it('lists the homepage', () => {
    expect(urls).toContain('https://allch.at')
  })

  it('includes the docs, developer API and upgrade pages', () => {
    expect(urls).toContain('https://allch.at/docs')
    expect(urls).toContain('https://allch.at/docs/api')
    expect(urls).toContain('https://allch.at/upgrade')
  })

  it('keeps the legal pages indexable', () => {
    expect(urls).toContain('https://allch.at/legal/privacy')
  })
})

describe('homepage metadata', () => {
  const src = metadataCatalog()

  it('sets a keyword-led absolute title', () => {
    // page.tsx must still apply it as `absolute`, or the layout template would
    // append '| All-Chat' to a title that already ends in it.
    expect(read('page.tsx')).toMatch(/absolute:\s*t\('metadata\.home\.title'\)/)
    expect(src).toMatch(/title:\s*'Multi-Platform Chat Overlay/)
  })

  it('leads the homepage description with the search phrase and all five platforms', () => {
    // Distinctive phrases from the homepage description, so this cannot pass on
    // the site description, which also mentions OBS and the same platforms.
    expect(src).toContain('multi-platform chat overlay for OBS')
    expect(src).toMatch(/Kick, TikTok, and Discord chat/)
  })
})

describe('blog metadata', () => {
  // The layout's titleTemplate appends '| All-Chat'; a blog title that adds it
  // itself renders as '... | All-Chat | All-Chat'.
  it('leaves the brand suffix to the layout template', () => {
    expect(read('blog/[slug]/page.tsx')).not.toMatch(/\| All-Chat/)
    for (const locale of ['en', 'de']) {
      const catalog = readFileSync(
        join(__dirname, '..', '..', 'lib', 'i18n', 'messages', locale, 'blog.ts'),
        'utf-8'
      )
      expect(catalog).not.toMatch(/metaTitle:.*\| All-Chat/)
    }
  })
})

describe('site description', () => {
  const src = metadataCatalog()

  it('names all five platforms including Discord in the description', () => {
    expect(src).toMatch(/Kick, TikTok, and Discord chat/)
  })

  it('describes an "OBS chat overlay" (the primary search term)', () => {
    expect(src).toContain('one OBS chat overlay')
  })

  it('is the description layout.tsx actually applies', () => {
    // The pairing the two assertions above cannot see on their own: correct
    // copy in the catalog is worthless if the route stops reading it.
    expect(read('layout.tsx')).toContain("description: t('metadata.site.description')")
  })
})

describe('docs structured data (JSON-LD)', () => {
  it('the streamer guide emits HowTo + BreadcrumbList', () => {
    const src = read('docs/page.tsx')
    expect(src).toContain("'HowTo'")
    expect(src).toContain("'BreadcrumbList'")
  })

  it('the developer API page emits TechArticle + BreadcrumbList', () => {
    const src = read('docs/api/page.tsx')
    expect(src).toContain("'TechArticle'")
    expect(src).toContain("'BreadcrumbList'")
  })
})

describe('login-gated pages', () => {
  // Google only honours a noindex it may crawl. Blocking these in robots.txt
  // instead got /dashboard "indexed, though blocked by robots.txt".
  const gated = ['dashboard', 'settings', 'admin', 'overlays', 'translate', 'moderate', 'link']
  const robots = read('robots.ts')

  it.each(gated)('/%s sends noindex from its layout', (segment) => {
    expect(read(`${segment}/layout.tsx`)).toMatch(/robots:\s*\{\s*index:\s*false/)
  })

  it.each(gated)('/%s is not disallowed in robots.txt', (segment) => {
    expect(robots).not.toMatch(new RegExp(`'/${segment}'`))
  })

  it('does not let the /overlay/ rule swallow /overlays by prefix', () => {
    expect(robots).toContain("'/overlay/'")
    expect(robots).not.toContain("'/overlay',")
  })
})

describe('alerts and chat commands are stated as shipped', () => {
  // Search engines and language models quote these pages as fact. While they
  // said "alerts are in development", assistants answered that All-Chat has no
  // alerts and no chat bot, months after both were released.
  const lib = (rel: string) => readFileSync(join(__dirname, '..', '..', 'lib', rel), 'utf-8')

  it.each(['llms.ts', 'i18n/messages/en/guides.ts', 'i18n/messages/de/guides.ts'])(
    '%s no longer calls alerts unreleased',
    (rel) => {
      expect(lib(rel)).not.toMatch(/in development|in Entwicklung|not available yet/i)
    }
  )

  it('llms.txt describes alert overlays, chat commands and timers', () => {
    const src = lib('llms.ts')
    expect(src).toMatch(/alert overlays/i)
    expect(src).toMatch(/chat commands/i)
    expect(src).toMatch(/timers/i)
  })

  it('the homepage description and structured data name alerts and chat commands', () => {
    expect(metadataCatalog()).toMatch(/home:[\s\S]*?alerts, chat commands/)
    expect(read('page.tsx')).toMatch(/Alert overlays/)
    expect(read('page.tsx')).toMatch(/Chat commands/)
  })

  it('the FAQ answers whether there are alerts and a chat bot', () => {
    expect(lib('faq.ts')).toContain("'alertsBot'")
  })

  it('the comparison lists Multistream with a source', () => {
    expect(read('compare/page.tsx')).toContain('https://multistream.gg/')
  })
})
