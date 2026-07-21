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

import { ApiError } from '@/lib/api/client'
import { isModerationReauthError } from '@/lib/api/moderation'

describe('isModerationReauthError', () => {
  it('is true when the moderation endpoint asks for re-consent', () => {
    // The moderation-service returns 403 with requires_reauth when the streamer's
    // stored broadcaster token can no longer perform the action (missing scope, or a
    // Helix 401 that a refresh could not fix) — the streamer must re-authorize.
    const err = new ApiError(403, 'moderation re-consent required', {
      error: 'moderation re-consent required',
      requires_reauth: true,
      missing_scopes: [],
    })
    expect(isModerationReauthError(err)).toBe(true)
  })

  it('is false for a generic moderation failure (no requires_reauth flag)', () => {
    const err = new ApiError(502, 'failed to apply moderation', {
      error: 'failed to apply moderation',
    })
    expect(isModerationReauthError(err)).toBe(false)
  })

  it('is false for the "no credential" case (not the broadcaster) — 422, no reauth', () => {
    // A user who does not hold moderator credentials for the channel gets 422; re-consent
    // would not help them, so we must NOT prompt a re-auth.
    const err = new ApiError(422, 'you do not hold moderator credentials for this channel', {
      error: 'you do not hold moderator credentials for this channel',
    })
    expect(isModerationReauthError(err)).toBe(false)
  })

  it('is false for a non-ApiError (e.g. a network TypeError)', () => {
    expect(isModerationReauthError(new TypeError('Failed to fetch'))).toBe(false)
    expect(isModerationReauthError(undefined)).toBe(false)
    expect(isModerationReauthError(null)).toBe(false)
  })
})
