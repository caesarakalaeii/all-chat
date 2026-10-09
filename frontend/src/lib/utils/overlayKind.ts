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
 * Overlay kind helpers (ADR-0064).
 *
 * `overlay_type` comes from three places that deploy at different times: the
 * overlays table (NOT NULL since migration 100, but the column did not exist
 * for overlays created before it), the overlay responses, and the public
 * overlay config the render page fetches — where it is simply absent until the
 * first fetch resolves. Every reader funnels through resolveOverlayKind so
 * those cases all mean "chat", the kind every overlay was before ADR-0064.
 */

import type { OverlayType } from '@/lib/types/overlay'

/** The four kinds, in create-form order. */
export const OVERLAY_KINDS: readonly OverlayType[] = ['chat', 'alerts', 'goal', 'list']

/**
 * The kinds a user may create today. Alerts, goal and list have no renderer
 * yet, so offering them would only put a "not supported yet" placeholder on
 * stream; existing overlays of those kinds still resolve and render as before.
 * overlay-manager refuses the same kinds on create (unreleasedOverlayTypes);
 * release a kind by adding it here and removing it there.
 */
export const CREATABLE_OVERLAY_KINDS: readonly OverlayType[] = ['chat']

/**
 * Resolve a raw overlay_type value to a known kind, defaulting to chat.
 *
 * An unknown non-empty value also resolves to chat: the DB CHECK constraint
 * rejects those, so it can only mean a backend ahead of this frontend, and the
 * chat renderer is the behaviour every overlay had before kinds existed — a
 * blank OBS source is the worse failure.
 */
export function resolveOverlayKind(value: string | null | undefined): OverlayType {
  return OVERLAY_KINDS.includes(value as OverlayType) ? (value as OverlayType) : 'chat'
}
