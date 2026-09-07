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
