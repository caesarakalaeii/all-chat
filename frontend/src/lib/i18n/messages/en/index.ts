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
 * The English UI string catalog, composed from one file per namespace.
 *
 * One file per namespace rather than one catalog file: the migration in #799
 * touched every surface in the app, and a single file would have made every
 * batch conflict with every other one. See docs/frontend/I18N.md for which
 * namespace a string belongs in.
 *
 * `as const` on each namespace file plus a literal object here keeps the composed
 * type fully literal, which is what `MessageKey` in ../../translate.ts is derived
 * from. Spreading, `satisfies MessageCatalog` or an explicit annotation would all
 * widen it to `string` and silently turn the compile-time key check off.
 *
 * This path is `./messages/en` to its importers, so splitting the file left
 * ../../translate.ts and ../../index.ts untouched.
 */

import { a11y } from './a11y'
import { admin } from './admin'
import { auth } from './auth'
import { common } from './common'
import { dashboard } from './dashboard'
import { docs } from './docs'
import { errors } from './errors'
import { legal } from './legal'
import { maintenanceBanner } from './maintenanceBanner'
import { marketing } from './marketing'
import { metadata } from './metadata'
import { moderation } from './moderation'
import { onboarding } from './onboarding'
import { overlayEditor } from './overlayEditor'
import { settings } from './settings'
import { viewerOverlay } from './viewerOverlay'

export const enMessages = {
  a11y,
  admin,
  auth,
  common,
  dashboard,
  docs,
  errors,
  legal,
  maintenanceBanner,
  marketing,
  metadata,
  moderation,
  onboarding,
  overlayEditor,
  settings,
  viewerOverlay,
} as const

export type EnMessages = typeof enMessages
