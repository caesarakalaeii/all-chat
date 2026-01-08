/**
 * Error Message Catalog
 *
 * Centralized user-friendly messages and actionable guidance for each error type.
 * Designed for easy internationalization in the future.
 */

import { ChatErrorType } from './types/errors';

export interface ErrorMessageTemplate {
  title: string;
  message: string;
  actionableSteps: string[];
}

/**
 * Error message templates for each error type
 */
export const ERROR_MESSAGES: Record<ChatErrorType, ErrorMessageTemplate> = {
  [ChatErrorType.UNAUTHORIZED]: {
    title: 'Authentication Required',
    message: 'You need to sign in to send messages.',
    actionableSteps: [
      'Click the "Sign in with {platform}" button',
      'Authorize the application to send messages on your behalf',
      'Try sending your message again',
    ],
  },

  [ChatErrorType.TOKEN_EXPIRED]: {
    title: 'Session Expired',
    message: 'Your authentication session has expired.',
    actionableSteps: [
      'Sign in again to refresh your session',
      'Make sure to authorize the application',
      'Try sending your message again',
    ],
  },

  [ChatErrorType.RATE_LIMITED]: {
    title: 'Rate Limit Reached',
    message: 'You\'re sending messages too quickly. Please slow down.',
    actionableSteps: [
      'Wait a moment before sending another message',
      'Avoid sending messages in rapid succession',
      'The rate limit will reset automatically',
    ],
  },

  [ChatErrorType.BANNED]: {
    title: 'Unable to Send Messages',
    message: 'You are currently banned from sending messages.',
    actionableSteps: [
      'Check the reason for the ban below',
      'Contact the streamer or moderators if you believe this is an error',
      'Wait for the ban to expire if it\'s temporary',
    ],
  },

  [ChatErrorType.STREAMER_OFFLINE]: {
    title: 'Stream Not Live',
    message: 'This streamer is not currently live.',
    actionableSteps: [
      'Check if the stream has ended or not started yet',
      'Try refreshing the page to update the stream status',
      'You can only send messages when the stream is live',
    ],
  },

  [ChatErrorType.PLATFORM_API_ERROR]: {
    title: 'Platform Error',
    message: 'The streaming platform encountered an error.',
    actionableSteps: [
      'This is likely a temporary issue with the platform',
      'Wait a moment and try again',
      'Check if the platform is experiencing outages',
      'Try sending your message again in a few moments',
    ],
  },

  [ChatErrorType.NETWORK_ERROR]: {
    title: 'Connection Error',
    message: 'Failed to connect to the server.',
    actionableSteps: [
      'Check your internet connection',
      'Try refreshing the page',
      'If the problem persists, the server may be experiencing issues',
    ],
  },

  [ChatErrorType.VALIDATION_ERROR]: {
    title: 'Invalid Message',
    message: 'Your message did not meet the requirements.',
    actionableSteps: [
      'Check that your message is not empty',
      'Make sure your message is not too long',
      'Avoid using prohibited characters or content',
    ],
  },

  [ChatErrorType.UNKNOWN_ERROR]: {
    title: 'Unexpected Error',
    message: 'An unexpected error occurred while sending your message.',
    actionableSteps: [
      'Try sending your message again',
      'Refresh the page if the problem persists',
      'Contact support if this error continues',
    ],
  },
};

/**
 * Get error message template for a specific error type
 */
export function getErrorMessage(errorType: ChatErrorType): ErrorMessageTemplate {
  return ERROR_MESSAGES[errorType];
}

/**
 * Get error message with platform placeholder replaced
 */
export function formatErrorMessage(
  errorType: ChatErrorType,
  platform?: string
): ErrorMessageTemplate {
  const template = ERROR_MESSAGES[errorType];

  if (!platform) {
    return template;
  }

  // Replace {platform} placeholder in actionable steps
  const formattedSteps = template.actionableSteps.map((step) =>
    step.replace('{platform}', capitalizeFirst(platform))
  );

  return {
    ...template,
    actionableSteps: formattedSteps,
  };
}

/**
 * Capitalize first letter of a string
 */
function capitalizeFirst(str: string): string {
  if (!str) return str;
  return str.charAt(0).toUpperCase() + str.slice(1);
}
