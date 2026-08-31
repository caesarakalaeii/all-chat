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
 * The public surface of the chat error catalog.
 *
 * errorMessages.ts was itself a catalog; issue #799 absorbed its copy into
 * `errors.*` but requires its Record shape and its two accessors to stay the
 * API, because errorParser.ts calls them from ten places and must not change.
 * These tests exercise that surface rather than the catalog, so they would fail
 * if the absorption altered what a caller receives -- an accessor that stopped
 * resolving keys, a dropped step, or a {platform} left unfilled.
 */

import { describe, expect, it } from 'vitest'

import { ERROR_MESSAGES, formatErrorMessage, getErrorMessage } from '../errorMessages'
import { ChatErrorType } from '../types/errors'

describe('getErrorMessage', () => {
  it('returns resolved copy, not catalog keys', () => {
    const template = getErrorMessage(ChatErrorType.RATE_LIMITED)
    expect(template.title).toBe('Rate Limit Reached')
    expect(template.message).toBe("You're sending messages too quickly. Please slow down.")
    expect(template.actionableSteps).toEqual([
      'Wait a moment before sending another message',
      'Avoid sending messages in rapid succession',
      'The rate limit will reset automatically',
    ])
  })

  it('covers every error type with a title, a message and at least one step', () => {
    for (const errorType of Object.values(ChatErrorType)) {
      const template = getErrorMessage(errorType)
      expect(template.title, errorType).not.toBe('')
      expect(template.message, errorType).not.toBe('')
      expect(template.actionableSteps.length, errorType).toBeGreaterThan(0)
      // A key that echoes itself is what t() returns for a missing message, so
      // this is the assertion that catches a namespace gap after the move.
      for (const step of template.actionableSteps) {
        expect(step, errorType).not.toMatch(/^errors\./)
      }
    }
  })
})

describe('formatErrorMessage', () => {
  it('fills the {platform} placeholder, capitalising the platform', () => {
    const template = formatErrorMessage(ChatErrorType.UNAUTHORIZED, 'twitch')
    expect(template.actionableSteps[0]).toBe('Click the "Sign in with Twitch" button')
  })

  it('leaves the placeholder visible when no platform is known', () => {
    // Deliberate: a visible {platform} is a bug report, whereas silently
    // dropping it produces a sentence that reads as if it were complete.
    const template = formatErrorMessage(ChatErrorType.UNAUTHORIZED)
    expect(template.actionableSteps[0]).toBe('Click the "Sign in with {platform}" button')
  })

  it('leaves steps without a placeholder untouched', () => {
    const template = formatErrorMessage(ChatErrorType.UNAUTHORIZED, 'kick')
    expect(template.actionableSteps[2]).toBe('Try sending your message again')
  })
})

describe('ERROR_MESSAGES', () => {
  it('is still keyed by every ChatErrorType', () => {
    // errorParser.ts indexes this record directly, so its shape is API.
    for (const errorType of Object.values(ChatErrorType)) {
      expect(ERROR_MESSAGES[errorType], errorType).toBeDefined()
    }
  })
})
