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
 * English UI string catalog.
 *
 * Namespaces are camelCase, nested at most three levels, and keyed by surface.
 * Placeholders are single braces, `{name}`, matching the convention already in
 * src/lib/errorMessages.ts.
 *
 * `as const` is load bearing: MessageKey in ../translate.ts is derived from this
 * object's literal key types, so widening it would silently turn the compile-time
 * key check into `string`.
 *
 * Only the two pilot surfaces live here. See docs/frontend/I18N.md before adding
 * a namespace.
 */

// `common.*` is deliberately absent until a string is genuinely shared by two
// surfaces. A namespace holding one key nobody reads is dead weight, and the
// convention is documented rather than reserved by an empty object.
export const enMessages = {
  a11y: {
    skipToMainContent: 'Skip to main content',
  },
  maintenanceBanner: {
    activeLabel: 'Maintenance in progress:',
    expectedCompletion: 'Expected completion:',
    scheduledLabel: 'Scheduled maintenance:',
    rangeSeparator: 'to',
    dismissLabel: 'Dismiss maintenance banner: {title}',
  },
} as const

export type EnMessages = typeof enMessages
