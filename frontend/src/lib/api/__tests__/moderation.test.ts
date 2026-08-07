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
import {
  boundInviteAccount,
  delegationErrorCode,
  isModerationReauthError,
} from '@/lib/api/moderation'

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

describe('delegationErrorCode', () => {
  it('reads the machine-readable code off the error body', () => {
    // The UI must switch on `code`, never on the prose in `error` — the copy is
    // free to change and is localized per role.
    const err = new ApiError(409, 'this overlay already has the maximum number of moderators', {
      error: 'this overlay already has the maximum number of moderators',
      code: 'moderator_cap_reached',
      cap: 10,
    })
    expect(delegationErrorCode(err)).toBe('moderator_cap_reached')
  })

  it('is undefined when there is no code (so callers fall back to generic copy)', () => {
    expect(
      delegationErrorCode(new ApiError(500, 'internal error', { error: 'internal error' }))
    ).toBeUndefined()
    expect(delegationErrorCode(new TypeError('Failed to fetch'))).toBeUndefined()
    expect(delegationErrorCode(null)).toBeUndefined()
  })

  // An unknown overlay, an unauthorized one, and a delegated moderator reaching for an
  // owner power all return the SAME 403 with no code. The UI must not infer a role from it.
  it('is undefined for the deliberately indistinguishable owner-only 403', () => {
    const err = new ApiError(403, 'not authorized for this overlay', {
      error: 'not authorized for this overlay',
    })
    expect(delegationErrorCode(err)).toBeUndefined()
  })
})

describe('boundInviteAccount', () => {
  it('names the account a pre-bound invite belongs to', () => {
    // Turning a dead end into an instruction: "this is @sarah's invite, sign in as her".
    const err = new ApiError(409, 'this invite was created for a different account', {
      error: 'this invite was created for a different account',
      code: 'invite_bound_to_other_account',
      expected_account: '@sarah',
      expected_platform: 'twitch',
    })
    expect(boundInviteAccount(err)).toEqual({ account: '@sarah', platform: 'twitch' })
  })

  it('is null for any other failure', () => {
    const err = new ApiError(409, 'you already moderate this overlay', {
      error: 'you already moderate this overlay',
      code: 'already_moderator',
    })
    expect(boundInviteAccount(err)).toBeNull()
  })
})
