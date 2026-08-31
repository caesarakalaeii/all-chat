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
 * Copy lock for the dashboard surface.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. A rendered-output
 * diff across 229 files is not reviewable, so the strings that were at the
 * render sites are pinned here instead, transcribed from the pre-migration
 * source. If a key's text drifts, this fails and names the key.
 *
 * Values built from a placeholder are asserted through `t()` so the
 * interpolation is covered too, not just the template.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('dashboard copy', () => {
  it('keeps the overlay list chrome', () => {
    expect(t('dashboard.overlays.heading')).toBe('Overlays')
    expect(t('dashboard.overlays.newOverlay')).toBe('New Overlay')
    expect(t('dashboard.overlays.loading')).toBe('Loading overlays')
    expect(t('dashboard.overlays.extensionBadge')).toBe('Extension')
    expect(t('dashboard.overlays.deactivateExtension')).toBe('Deactivate Extension')
    expect(t('dashboard.overlays.setAsExtension')).toBe('Set as Extension Overlay')
  })

  it('keeps the empty state', () => {
    expect(t('dashboard.empty.heading')).toBe('No overlays yet')
    expect(t('dashboard.empty.body')).toBe(
      'Create your first overlay to see chat from all your platforms in one place.'
    )
    expect(t('dashboard.empty.createFirst')).toBe('Create your first overlay')
  })

  it('keeps the source count in one key per grammatical number', () => {
    // Was `{n} source{n !== 1 ? 's' : ''}` at the render site. Two keys rather
    // than a concatenated suffix: a language that inflects the noun cannot
    // rebuild it from an English 's'. There are no ICU plurals here, so the
    // caller picks the key.
    expect(t('dashboard.overlays.sourceCountOne', { count: 1 })).toBe('1 source')
    expect(t('dashboard.overlays.sourceCountOther', { count: 0 })).toBe('0 sources')
    expect(t('dashboard.overlays.sourceCountOther', { count: 3 })).toBe('3 sources')
  })

  it('keeps the delete dialog, typographic quotes included', () => {
    // The render site spelled these &ldquo;/&rdquo;; a catalog string is not
    // HTML, so they are the characters themselves and must render the same.
    expect(t('dashboard.deleteOverlay.title', { name: 'Main chat' })).toBe(
      'Delete \u201cMain chat\u201d?'
    )
    expect(t('dashboard.deleteOverlay.description')).toBe(
      'This action cannot be undone. All sources will be removed.'
    )
    expect(t('dashboard.deleteOverlay.cancel')).toBe('Cancel')
    expect(t('dashboard.deleteOverlay.confirm')).toBe('Delete')
    expect(t('dashboard.overlays.deleteLabel', { name: 'Main chat' })).toBe('Delete Main chat')
  })

  it('keeps the overlay mutation toasts', () => {
    expect(t('dashboard.toasts.overlayDeleted')).toBe('Overlay deleted')
    expect(t('dashboard.toasts.overlayDeleteFailed')).toBe('Failed to delete overlay')
    expect(t('dashboard.toasts.tryAgain')).toBe('Please try again.')
    expect(t('dashboard.toasts.extensionOverlayUpdated')).toBe('Extension overlay updated')
    expect(t('dashboard.toasts.extensionOverlayDeactivated')).toBe('Extension overlay deactivated')
    expect(t('dashboard.toasts.overlayUpdateFailed')).toBe('Failed to update overlay')
  })
})
