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
 * The streamer dashboard and the widgets it owns.
 */

export const dashboard = {
  overlays: {
    heading: 'Overlays',
    newOverlay: 'New Overlay',
    loading: 'Loading overlays',
    extensionBadge: 'Extension',
    deactivateExtension: 'Deactivate Extension',
    setAsExtension: 'Set as Extension Overlay',
    deleteLabel: 'Delete {name}',
    // Two keys rather than a concatenated 's': see the comment in
    // __tests__/dashboard.test.ts. The caller picks by count.
    sourceCountOne: '{count} source',
    sourceCountOther: '{count} sources',
  },
  empty: {
    heading: 'No overlays yet',
    body: 'Create your first overlay to see chat from all your platforms in one place.',
    createFirst: 'Create your first overlay',
  },
  deleteOverlay: {
    // The render site spelled the quotes &ldquo;/&rdquo;. A catalog string is
    // not HTML, so they are the characters themselves.
    title: 'Delete “{name}”?',
    description: 'This action cannot be undone. All sources will be removed.',
    cancel: 'Cancel',
    confirm: 'Delete',
  },
  toasts: {
    overlayDeleted: 'Overlay deleted',
    overlayDeleteFailed: 'Failed to delete overlay',
    tryAgain: 'Please try again.',
    extensionOverlayUpdated: 'Extension overlay updated',
    extensionOverlayDeactivated: 'Extension overlay deactivated',
    overlayUpdateFailed: 'Failed to update overlay',
  },
} as const
