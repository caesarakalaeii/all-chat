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

import { describe, it, expect } from 'vitest'
import {
  EDITOR_SECTIONS,
  EDITOR_GROUPS,
  searchSettings,
  type EditorSectionId,
} from '../sectionRegistry'

describe('EDITOR_SECTIONS registry', () => {
  it('has unique section ids', () => {
    const ids = EDITOR_SECTIONS.map((s) => s.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('every section has a title, description, and at least one search entry', () => {
    for (const s of EDITOR_SECTIONS) {
      expect(s.title.length).toBeGreaterThan(0)
      expect(s.description.length).toBeGreaterThan(0)
      expect(s.entries.length).toBeGreaterThan(0)
    }
  })

  it('every section belongs to a declared group, and groups appear in declared order', () => {
    const groupOrder = EDITOR_GROUPS.map((g) => g.id)
    for (const s of EDITOR_SECTIONS) {
      expect(groupOrder).toContain(s.group)
    }
    // Sections are declared grouped: once a later group starts, an earlier one never reappears
    let maxSeen = 0
    for (const s of EDITOR_SECTIONS) {
      const idx = groupOrder.indexOf(s.group)
      expect(idx).toBeGreaterThanOrEqual(maxSeen === 0 ? 0 : 0)
      if (idx < maxSeen) {
        throw new Error(`section ${s.id} of group ${s.group} appears after a later group`)
      }
      maxSeen = Math.max(maxSeen, idx)
    }
  })

  it('the sections the onboarding guide spotlights exist', () => {
    const ids = EDITOR_SECTIONS.map((s) => s.id)
    for (const required of ['theme', 'sources', 'typography'] as EditorSectionId[]) {
      expect(ids).toContain(required)
    }
  })
})

describe('searchSettings', () => {
  it('finds both badge toggles for "badge"', () => {
    const hits = searchSettings('badge')
    const labels = hits.map((h) => h.entry?.label ?? h.section.title)
    expect(labels).toContain('Show badges')
    expect(labels).toContain('Show platform badge')
  })

  it('finds visibility toggles via synonym keywords ("remove")', () => {
    const hits = searchSettings('remove')
    const sections = hits.map((h) => h.section.id)
    expect(sections).toContain('visibility')
  })

  it('is case-insensitive', () => {
    const lower = searchSettings('badge')
    const upper = searchSettings('BADGE')
    expect(upper.map((h) => h.entry?.label)).toEqual(lower.map((h) => h.entry?.label))
  })

  it('returns nothing for empty or one-character queries', () => {
    expect(searchSettings('')).toEqual([])
    expect(searchSettings(' ')).toEqual([])
    expect(searchSettings('b')).toEqual([])
  })

  it('respects the limit', () => {
    const hits = searchSettings('show', EDITOR_SECTIONS, 3)
    expect(hits.length).toBeLessThanOrEqual(3)
  })

  it('ranks label matches above keyword-only matches', () => {
    // "duration" is the label of Message duration and at best a keyword elsewhere
    const hits = searchSettings('duration')
    expect(hits[0]?.entry?.label.toLowerCase()).toContain('duration')
  })

  it('matches section titles as section-level hits', () => {
    const hits = searchSettings('typography')
    const sectionHit = hits.find((h) => h.section.id === 'typography' && h.entry === undefined)
    expect(sectionHit).toBeDefined()
  })

  it('badge search resolves to anchored entries in the visibility section', () => {
    const hits = searchSettings('badge')
    const showBadges = hits.find((h) => h.entry?.label === 'Show badges')
    expect(showBadges?.section.id).toBe('visibility')
    expect(showBadges?.entry?.anchorId).toBe('showBadges')
  })
})
