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

import { shouldOfferAdBreaksOptIn } from '../adBreaksOptIn'

describe('shouldOfferAdBreaksOptIn', () => {
  it('offers the opt-in to an owner with a Twitch source and no grant', () => {
    expect(
      shouldOfferAdBreaksOptIn({ isOwner: true, hasTwitchSource: true, adBreaksGranted: false })
    ).toBe(true)
  })

  it('hides the opt-in once the grant exists', () => {
    expect(
      shouldOfferAdBreaksOptIn({ isOwner: true, hasTwitchSource: true, adBreaksGranted: true })
    ).toBe(false)
  })

  // An older backend, or one that could not read the credential, sends no flag. That is
  // "cannot tell", and cannot-tell has to keep the CTA reachable.
  it('offers the opt-in when the backend sends no flag', () => {
    expect(
      shouldOfferAdBreaksOptIn({ isOwner: true, hasTwitchSource: true, adBreaksGranted: undefined })
    ).toBe(true)
  })

  // The consent re-authorizes the STREAMER's broadcaster credential, which is not a
  // moderator's to give.
  it('never offers the opt-in to a non-owner', () => {
    expect(
      shouldOfferAdBreaksOptIn({ isOwner: false, hasTwitchSource: true, adBreaksGranted: false })
    ).toBe(false)
  })

  it('never offers the opt-in without a Twitch source', () => {
    expect(
      shouldOfferAdBreaksOptIn({ isOwner: true, hasTwitchSource: false, adBreaksGranted: false })
    ).toBe(false)
  })
})
