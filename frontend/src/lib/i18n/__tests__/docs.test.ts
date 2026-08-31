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
 * Copy lock for the documentation surfaces.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. A rendered-output
 * diff across 229 files is not reviewable, so the strings that were at the
 * render sites are pinned here instead, transcribed from the pre-migration
 * source. If a key's text drifts, this fails and names the key.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('docs field table copy', () => {
  it('keeps the three column headers', () => {
    // FieldTable is the shared field-reference table used by both /docs pages.
    expect(t('docs.fieldTable.columnField')).toBe('Field')
    expect(t('docs.fieldTable.columnType')).toBe('Type')
    expect(t('docs.fieldTable.columnDescription')).toBe('Description')
  })
})

describe('theme contrast harness copy', () => {
  it('keeps the dev harness header', () => {
    // /dev/theme-contrast is a developer tool, so its copy sits with the other
    // developer-facing surfaces rather than in a user namespace.
    expect(t('docs.themeContrast.heading')).toBe('Theme contrast harness')
    // One sentence with the count as a param: 'themes.' was a separate JSX run
    // after the number, which a language that puts the noun first cannot move.
    expect(t('docs.themeContrast.intro', { count: 34 })).toBe(
      'Dev-only. Renders every bundled theme for the message-text WCAG gate. 34 themes.'
    )
  })
})
