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
 * Chat error copy, keyed by ChatErrorType.
 *
 * Backs src/lib/errorMessages.ts, which keeps its Record shape and accessors as
 * the public API. `actionableSteps` is a list of keys resolved one at a time, not
 * a joined string, so word order stays the catalog's business.
 */

export const errors = {
  unauthorized: {
    title: 'Authentication Required',
    message: 'You need to sign in to send messages.',
    step1: 'Click the "Sign in with {platform}" button',
    step2: 'Authorize the application to send messages on your behalf',
    step3: 'Try sending your message again',
  },
  tokenExpired: {
    title: 'Session Expired',
    message: 'Your authentication session has expired.',
    step1: 'Sign in again to refresh your session',
    step2: 'Make sure to authorize the application',
    step3: 'Try sending your message again',
  },
  rateLimited: {
    title: 'Rate Limit Reached',
    message: "You're sending messages too quickly. Please slow down.",
    step1: 'Wait a moment before sending another message',
    step2: 'Avoid sending messages in rapid succession',
    step3: 'The rate limit will reset automatically',
  },
  banned: {
    title: 'Unable to Send Messages',
    message: 'You are currently banned from sending messages.',
    step1: 'Check the reason for the ban below',
    step2: 'Contact the streamer or moderators if you believe this is an error',
    step3: "Wait for the ban to expire if it's temporary",
  },
  streamerOffline: {
    title: 'Stream Not Live',
    message: 'This streamer is not currently live.',
    step1: 'Check if the stream has ended or not started yet',
    step2: 'Try refreshing the page to update the stream status',
    step3: 'You can only send messages when the stream is live',
  },
  // The only error type with four steps.
  platformApiError: {
    title: 'Platform Error',
    message: 'The streaming platform encountered an error.',
    step1: 'This is likely a temporary issue with the platform',
    step2: 'Wait a moment and try again',
    step3: 'Check if the platform is experiencing outages',
    step4: 'Try sending your message again in a few moments',
  },
  networkError: {
    title: 'Connection Error',
    message: 'Failed to connect to the server.',
    step1: 'Check your internet connection',
    step2: 'Try refreshing the page',
    step3: 'If the problem persists, the server may be experiencing issues',
  },
  validationError: {
    title: 'Invalid Message',
    message: 'Your message did not meet the requirements.',
    step1: 'Check that your message is not empty',
    step2: 'Make sure your message is not too long',
    step3: 'Avoid using prohibited characters or content',
  },
  unknownError: {
    title: 'Unexpected Error',
    message: 'An unexpected error occurred while sending your message.',
    step1: 'Try sending your message again',
    step2: 'Refresh the page if the problem persists',
    step3: 'Contact support if this error continues',
  },

  // Chrome around a rendered ChatError: the labels, the detail lines and the
  // recovery buttons. The nine error bodies above are the copy itself.
  display: {
    iconLabel: 'Error icon',
    dismissLabel: 'Dismiss error',

    rateLimitCountdown: 'You can send another message in {countdown}',
    reasonLabel: 'Reason:',
    expiresLabel: 'Expires:',
    platformMessage: 'Platform message: {message}',

    whatYouCanDo: 'What you can do:',
    tryAgain: 'Try Again',
    signInWith: 'Sign in with {platform}',
    // Two whole labels rather than a swapped word plus ' Details': a translator
    // is not asked to make the verb and the noun agree separately.
    showDetails: 'Show Details',
    hideDetails: 'Hide Details',
  },
} as const
