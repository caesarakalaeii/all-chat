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

import { beforeEach, describe, expect, it, vi } from 'vitest'

// vi.mock's factory is hoisted above every import, so anything it closes over must be
// created with vi.hoisted() or the suite fails to collect.
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/client')>('@/lib/api/client')
  return { ...actual, apiClient: client }
})

import { ApiError } from '@/lib/api/client'
import {
  boundInviteAccount,
  delegationErrorCode,
  isModerationReauthError,
  moderationActionCode,
  moderationApi,
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

describe('moderator-side endpoints', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reads the delegation list from the moderator-scoped route', async () => {
    client.get.mockResolvedValue({ delegations: [] })
    await moderationApi.listDelegations()
    expect(client.get).toHaveBeenCalledWith('/api/v1/moderation/delegations')
  })

  // The secret must never reach a URL: a path parameter would be captured by every
  // access log, proxy log and Referer header between here and the service.
  it('sends the invite secret in the body, never the path', async () => {
    client.post.mockResolvedValue({})
    await moderationApi.previewInvite('SEEKRIT')
    const [path, body] = client.post.mock.calls[0]
    expect(path).toBe('/api/v1/moderation/invites/preview')
    expect(path).not.toContain('SEEKRIT')
    expect(body).toEqual({ token: 'SEEKRIT' })
  })

  it('accepts an invite with the secret in the body and an idempotency key', async () => {
    client.post.mockResolvedValue({})
    await moderationApi.acceptInvite('SEEKRIT')
    const [path, body, headers] = client.post.mock.calls[0]
    expect(path).toBe('/api/v1/moderation/invites/accept')
    expect(path).not.toContain('SEEKRIT')
    expect(body).toEqual({ token: 'SEEKRIT' })
    expect(headers).toHaveProperty('Idempotency-Key')
  })

  // A moderator's consent carries no overlay id — Twitch/Kick moderation scopes are
  // role-based, so one consent serves every streamer who delegated that platform — and
  // requests only the delegated actions, so a volunteer is never shown a wider screen.
  it('requests mod consent without an overlay and with only the delegated actions', async () => {
    client.get.mockResolvedValue({ auth_url: 'https://id.twitch.tv/authorize?x' })
    const url = await moderationApi.getModConsentUrl('twitch', ['delete', 'timeout'])
    expect(client.get).toHaveBeenCalledWith(
      '/api/v1/auth/twitch/mod-consent?actions=delete,timeout'
    )
    expect(url).toBe('https://id.twitch.tv/authorize?x')
  })
})

// The three action-failure codes (ADR-0048) differ in WHO can fix them, which is the only reason
// the UI needs to tell them apart: the moderator, the streamer, or nobody yet.
describe('moderationActionCode', () => {
  it('reads the code off a failed action', () => {
    const err = new ApiError(422, 'connect your own twitch account to moderate here', {
      error: 'connect your own twitch account to moderate here',
      code: 'connect_required',
    })
    expect(moderationActionCode(err)).toBe('connect_required')
  })

  it('distinguishes the streamer-side failure from the moderator-side one', () => {
    const owner = new ApiError(403, 'not connected', {
      error: 'not connected',
      code: 'owner_channel_unverified',
    })
    expect(moderationActionCode(owner)).toBe('owner_channel_unverified')
  })

  it('is undefined for a plain failure, so callers keep their generic copy', () => {
    expect(
      moderationActionCode(
        new ApiError(502, 'failed to apply moderation', { error: 'failed to apply moderation' })
      )
    ).toBeUndefined()
    expect(moderationActionCode(new TypeError('Failed to fetch'))).toBeUndefined()
  })
})
