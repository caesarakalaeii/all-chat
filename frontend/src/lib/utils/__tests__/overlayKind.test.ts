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

import { describe, expect, it } from 'vitest'

import { resolveOverlayKind } from '@/lib/utils/overlayKind'

// overlay_type (ADR-0064): every overlay is one of four kinds, and every
// surface that reads the column has to agree on what an absent or unknown
// value means. These are the rules the backend's Validate()/Kind() pair
// already enforces; the frontend cannot assume a fresh deploy ships together.
describe('resolveOverlayKind', () => {
  it('keeps every supported kind', () => {
    expect(resolveOverlayKind('chat')).toBe('chat')
    expect(resolveOverlayKind('alerts')).toBe('alerts')
    expect(resolveOverlayKind('goal')).toBe('goal')
    expect(resolveOverlayKind('list')).toBe('list')
  })

  it('resolves an absent kind to chat', () => {
    // The public config comes from a 30s-refreshed fetch; the first render
    // always sees null, and an overlay created before the column existed
    // never carries a value at all.
    expect(resolveOverlayKind(undefined)).toBe('chat')
    expect(resolveOverlayKind(null)).toBe('chat')
    expect(resolveOverlayKind('')).toBe('chat')
  })

  it('resolves an unknown kind to chat rather than refusing to render', () => {
    // The DB CHECK constraint rejects unknown values, so this only happens
    // if a future backend ships a new kind before this frontend. Falling back
    // to the chat renderer keeps today's behaviour for every overlay instead
    // of a blank OBS source.
    expect(resolveOverlayKind('credits')).toBe('chat')
  })
})
