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

import { shouldOfferModLogOptIn } from '../modLogOptIn'

describe('shouldOfferModLogOptIn', () => {
  it('offers the opt-in to an owner with a Twitch source and no grant', () => {
    expect(
      shouldOfferModLogOptIn({ isOwner: true, hasTwitchSource: true, modLogGranted: false })
    ).toBe(true)
  })

  // The fix for issue #815: a streamer who completed the nine-scope consent was shown the
  // same banner again, which reads as "your grant did not take" and invites a re-consent
  // they do not need.
  it('hides the opt-in once the grant exists', () => {
    expect(
      shouldOfferModLogOptIn({ isOwner: true, hasTwitchSource: true, modLogGranted: true })
    ).toBe(false)
  })

  // An older backend, or one that could not read the credential, sends no flag. That is
  // "cannot tell", and cannot-tell has to keep the CTA reachable.
  it('offers the opt-in when the backend sends no flag', () => {
    expect(
      shouldOfferModLogOptIn({ isOwner: true, hasTwitchSource: true, modLogGranted: undefined })
    ).toBe(true)
  })

  // The consent re-authorizes the STREAMER's broadcaster credential, which is not a
  // moderator's to give.
  it('never offers the opt-in to a non-owner', () => {
    expect(
      shouldOfferModLogOptIn({ isOwner: false, hasTwitchSource: true, modLogGranted: false })
    ).toBe(false)
  })

  it('never offers the opt-in without a Twitch source', () => {
    expect(
      shouldOfferModLogOptIn({ isOwner: true, hasTwitchSource: false, modLogGranted: false })
    ).toBe(false)
  })
})
