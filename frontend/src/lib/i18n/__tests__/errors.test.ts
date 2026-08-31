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
 * Copy lock for the chat error catalog, absorbed from src/lib/errorMessages.ts.
 *
 * That module was a second, parallel catalog: a Record<ChatErrorType, ...> of
 * nine error types, each with a title, a message and an actionableSteps array.
 * Its own header said it was "designed for easy internationalization in the
 * future", and it already used the same single-brace {platform} placeholder
 * syntax the real catalog uses. This lock pins all nine before the move, so the
 * absorption cannot reword a single step.
 *
 * The steps are separate numbered keys rather than one joined string, because a
 * joined string is the fragment concatenation docs/frontend/I18N.md forbids: a
 * translator needs each step whole and reorderable.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('unauthorized', () => {
  it('keeps its copy and its {platform} placeholder', () => {
    expect(t('errors.unauthorized.title')).toBe('Authentication Required')
    expect(t('errors.unauthorized.message')).toBe('You need to sign in to send messages.')
    // The placeholder syntax carried over from errorMessages.ts unchanged, so it
    // resolves through t() now rather than through a bespoke .replace().
    expect(t('errors.unauthorized.step1', { platform: 'Twitch' })).toBe(
      'Click the "Sign in with Twitch" button'
    )
    expect(t('errors.unauthorized.step2')).toBe(
      'Authorize the application to send messages on your behalf'
    )
    expect(t('errors.unauthorized.step3')).toBe('Try sending your message again')
  })
})

describe('token expired', () => {
  it('keeps its copy', () => {
    expect(t('errors.tokenExpired.title')).toBe('Session Expired')
    expect(t('errors.tokenExpired.message')).toBe('Your authentication session has expired.')
    expect(t('errors.tokenExpired.step1')).toBe('Sign in again to refresh your session')
    expect(t('errors.tokenExpired.step2')).toBe('Make sure to authorize the application')
    expect(t('errors.tokenExpired.step3')).toBe('Try sending your message again')
  })
})

describe('rate limited', () => {
  it('keeps its copy', () => {
    expect(t('errors.rateLimited.title')).toBe('Rate Limit Reached')
    expect(t('errors.rateLimited.message')).toBe(
      "You're sending messages too quickly. Please slow down."
    )
    expect(t('errors.rateLimited.step1')).toBe('Wait a moment before sending another message')
    expect(t('errors.rateLimited.step2')).toBe('Avoid sending messages in rapid succession')
    expect(t('errors.rateLimited.step3')).toBe('The rate limit will reset automatically')
  })
})

describe('banned', () => {
  it('keeps its copy', () => {
    expect(t('errors.banned.title')).toBe('Unable to Send Messages')
    expect(t('errors.banned.message')).toBe('You are currently banned from sending messages.')
    expect(t('errors.banned.step1')).toBe('Check the reason for the ban below')
    expect(t('errors.banned.step2')).toBe(
      'Contact the streamer or moderators if you believe this is an error'
    )
    expect(t('errors.banned.step3')).toBe("Wait for the ban to expire if it's temporary")
  })
})

describe('streamer offline', () => {
  it('keeps its copy', () => {
    expect(t('errors.streamerOffline.title')).toBe('Stream Not Live')
    expect(t('errors.streamerOffline.message')).toBe('This streamer is not currently live.')
    expect(t('errors.streamerOffline.step1')).toBe(
      'Check if the stream has ended or not started yet'
    )
    expect(t('errors.streamerOffline.step2')).toBe(
      'Try refreshing the page to update the stream status'
    )
    expect(t('errors.streamerOffline.step3')).toBe(
      'You can only send messages when the stream is live'
    )
  })
})

describe('platform API error', () => {
  it('keeps its copy, including its fourth step', () => {
    // The only error type with four steps. Numbered keys keep that difference
    // explicit rather than hiding it inside a joined string.
    expect(t('errors.platformApiError.title')).toBe('Platform Error')
    expect(t('errors.platformApiError.message')).toBe(
      'The streaming platform encountered an error.'
    )
    expect(t('errors.platformApiError.step1')).toBe(
      'This is likely a temporary issue with the platform'
    )
    expect(t('errors.platformApiError.step2')).toBe('Wait a moment and try again')
    expect(t('errors.platformApiError.step3')).toBe('Check if the platform is experiencing outages')
    expect(t('errors.platformApiError.step4')).toBe(
      'Try sending your message again in a few moments'
    )
  })
})

describe('network error', () => {
  it('keeps its copy', () => {
    expect(t('errors.networkError.title')).toBe('Connection Error')
    expect(t('errors.networkError.message')).toBe('Failed to connect to the server.')
    expect(t('errors.networkError.step1')).toBe('Check your internet connection')
    expect(t('errors.networkError.step2')).toBe('Try refreshing the page')
    expect(t('errors.networkError.step3')).toBe(
      'If the problem persists, the server may be experiencing issues'
    )
  })
})

describe('validation error', () => {
  it('keeps its copy', () => {
    expect(t('errors.validationError.title')).toBe('Invalid Message')
    expect(t('errors.validationError.message')).toBe('Your message did not meet the requirements.')
    expect(t('errors.validationError.step1')).toBe('Check that your message is not empty')
    expect(t('errors.validationError.step2')).toBe('Make sure your message is not too long')
    expect(t('errors.validationError.step3')).toBe('Avoid using prohibited characters or content')
  })
})

describe('unknown error', () => {
  it('keeps its copy', () => {
    expect(t('errors.unknownError.title')).toBe('Unexpected Error')
    expect(t('errors.unknownError.message')).toBe(
      'An unexpected error occurred while sending your message.'
    )
    expect(t('errors.unknownError.step1')).toBe('Try sending your message again')
    expect(t('errors.unknownError.step2')).toBe('Refresh the page if the problem persists')
    expect(t('errors.unknownError.step3')).toBe('Contact support if this error continues')
  })
})

describe('error display chrome copy', () => {
  it('keeps the icon and dismiss labels', () => {
    expect(t('errors.display.iconLabel')).toBe('Error icon')
    expect(t('errors.display.dismissLabel')).toBe('Dismiss error')
  })

  it('keeps the per-error-kind detail lines', () => {
    expect(t('errors.display.rateLimitCountdown', { countdown: '1:05' })).toBe(
      'You can send another message in 1:05'
    )
    expect(t('errors.display.reasonLabel')).toBe('Reason:')
    expect(t('errors.display.expiresLabel')).toBe('Expires:')
    expect(t('errors.display.platformMessage', { message: 'slow down' })).toBe(
      'Platform message: slow down'
    )
  })

  it('keeps the actionable-steps heading and the action buttons', () => {
    expect(t('errors.display.whatYouCanDo')).toBe('What you can do:')
    expect(t('errors.display.tryAgain')).toBe('Try Again')
    expect(t('errors.display.signInWith', { platform: 'Twitch' })).toBe('Sign in with Twitch')
    // Two words are swapped in place today; each whole label is its own key so a
    // translator is not asked to make "Show" and "Details" agree separately.
    expect(t('errors.display.showDetails')).toBe('Show Details')
    expect(t('errors.display.hideDetails')).toBe('Hide Details')
  })
})
