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
 * The German UI string catalog (ADR-0066), loaded on demand by
 * ../../active-locale.ts only when a user picks German.
 *
 * Each namespace file is annotated with LocaleNamespace, so a key missing from
 * or added to a German file is a compile error. `legal` is absent on purpose:
 * the privacy policy and terms are binding text, and a machine translation of
 * them must not be published without a legal review, so they resolve to
 * English. __tests__/de.test.ts pins that list.
 */

import type { LocaleMessages } from '../../translate'
import { a11y } from './a11y'
import { admin } from './admin'
import { auth } from './auth'
import { blog } from './blog'
import { common } from './common'
import { dashboard } from './dashboard'
import { docs } from './docs'
import { errors } from './errors'
import { guides } from './guides'
import { maintenanceBanner } from './maintenanceBanner'
import { marketing } from './marketing'
import { metadata } from './metadata'
import { moderation } from './moderation'
import { onboarding } from './onboarding'
import { overlayEditor } from './overlayEditor'
import { settings } from './settings'
import { translate } from './translate'
import { viewerOverlay } from './viewerOverlay'

export const deMessages: LocaleMessages = {
  a11y,
  admin,
  auth,
  blog,
  common,
  dashboard,
  docs,
  errors,
  guides,
  maintenanceBanner,
  marketing,
  metadata,
  moderation,
  onboarding,
  overlayEditor,
  settings,
  translate,
  viewerOverlay,
}
