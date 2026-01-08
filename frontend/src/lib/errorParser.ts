/**
 * Error Parser Module
 *
 * Smart parsing logic to convert API responses into typed ChatError objects.
 * Detects error types from status codes, message content, and response metadata.
 */

import {
  ChatError,
  ChatErrorType,
  UnauthorizedError,
  TokenExpiredError,
  RateLimitedError,
  BannedError,
  StreamerOfflineError,
  PlatformApiError,
  NetworkError,
  ValidationError,
  UnknownError,
} from './types/errors';
import { getErrorMessage, formatErrorMessage } from './errorMessages';

/**
 * Parse an API error response into a typed ChatError
 */
export function parseApiError(response: Response, data?: any): ChatError {
  const statusCode = response.status;
  const statusText = response.statusText;

  // Extract error message and details from response data
  let errorMessage = '';
  let errorDetails = '';
  let platform = '';

  if (data) {
    errorMessage = data.error || data.message || '';
    errorDetails = data.details || '';
    platform = data.platform || '';
  }

  // Combine message sources for pattern matching
  const fullErrorText = `${errorMessage} ${errorDetails}`.toLowerCase();

  // 1. Check for authentication errors (401, 403)
  if (statusCode === 401 || fullErrorText.includes('unauthorized') || fullErrorText.includes('not authenticated')) {
    // Check if it's a token expiration
    if (fullErrorText.includes('token expired') || fullErrorText.includes('session expired')) {
      return createTokenExpiredError(errorMessage, platform, statusCode);
    }
    return createUnauthorizedError(errorMessage, platform, statusCode);
  }

  // 2. Check for rate limiting (429)
  if (statusCode === 429 || fullErrorText.includes('rate limit') || fullErrorText.includes('too many requests')) {
    const resetTime = data?.reset_time;
    const retryAfter = data?.retry_after || response.headers.get('Retry-After');
    return createRateLimitedError(errorMessage, resetTime, retryAfter, statusCode);
  }

  // 3. Check for banned status (403 with ban reason)
  if ((statusCode === 403 && fullErrorText.includes('banned')) ||
      fullErrorText.includes('you are banned') ||
      fullErrorText.includes('user is banned')) {
    const reason = data?.reason || data?.ban_reason;
    const expiresAt = data?.expires_at || data?.ban_expires_at;
    return createBannedError(errorMessage, reason, expiresAt, statusCode);
  }

  // 4. Check for streamer offline / not live
  if (fullErrorText.includes('not currently live') ||
      fullErrorText.includes('stream is offline') ||
      fullErrorText.includes('not live on') ||
      fullErrorText.includes('streamer is offline')) {
    const streamerName = data?.streamer || data?.channel || extractStreamerName(fullErrorText);
    return createStreamerOfflineError(errorMessage, platform, streamerName, statusCode);
  }

  // 5. Check for platform API errors (502, 503, 504)
  if (statusCode === 502 || statusCode === 503 || statusCode === 504 ||
      fullErrorText.includes('platform api') ||
      fullErrorText.includes('youtube api') ||
      fullErrorText.includes('twitch api') ||
      fullErrorText.includes('kick api')) {
    const platformMessage = data?.platform_error || data?.api_error;
    return createPlatformApiError(errorMessage, platform, platformMessage, statusCode);
  }

  // 6. Check for validation errors (400)
  if (statusCode === 400 ||
      fullErrorText.includes('invalid') ||
      fullErrorText.includes('validation failed') ||
      fullErrorText.includes('message too long') ||
      fullErrorText.includes('message is empty')) {
    const field = data?.field;
    const constraint = data?.constraint;
    return createValidationError(errorMessage, field, constraint, statusCode);
  }

  // 7. Check for network errors (0, 5xx)
  if (statusCode === 0 || statusCode >= 500 ||
      fullErrorText.includes('network error') ||
      fullErrorText.includes('connection failed') ||
      fullErrorText.includes('timeout')) {
    return createNetworkError(errorMessage, statusCode);
  }

  // 8. Unknown error (fallback)
  return createUnknownError(errorMessage || statusText, data, statusCode);
}

/**
 * Create an unauthorized error
 */
function createUnauthorizedError(message: string, platform: string, statusCode: number): UnauthorizedError {
  const template = formatErrorMessage(ChatErrorType.UNAUTHORIZED, platform);
  return {
    type: ChatErrorType.UNAUTHORIZED,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    platform: platform || 'unknown',
    technicalDetails: `Status ${statusCode}: ${message}`,
  };
}

/**
 * Create a token expired error
 */
function createTokenExpiredError(message: string, platform: string, statusCode: number): TokenExpiredError {
  const template = formatErrorMessage(ChatErrorType.TOKEN_EXPIRED, platform);
  return {
    type: ChatErrorType.TOKEN_EXPIRED,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    platform: platform || 'unknown',
    technicalDetails: `Status ${statusCode}: ${message}`,
  };
}

/**
 * Create a rate limited error
 */
function createRateLimitedError(
  message: string,
  resetTime?: string,
  retryAfter?: string | number,
  statusCode?: number
): RateLimitedError {
  const template = getErrorMessage(ChatErrorType.RATE_LIMITED);

  // Parse retry after if it's a string
  let retryAfterSeconds: number | undefined;
  if (retryAfter) {
    retryAfterSeconds = typeof retryAfter === 'number' ? retryAfter : parseInt(retryAfter, 10);
  }

  return {
    type: ChatErrorType.RATE_LIMITED,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    resetTime,
    retryAfter: retryAfterSeconds,
    technicalDetails: statusCode ? `Status ${statusCode}: ${message}` : message,
  };
}

/**
 * Create a banned error
 */
function createBannedError(
  message: string,
  reason?: string,
  expiresAt?: string,
  statusCode?: number
): BannedError {
  const template = getErrorMessage(ChatErrorType.BANNED);
  return {
    type: ChatErrorType.BANNED,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    reason,
    expiresAt,
    technicalDetails: statusCode ? `Status ${statusCode}: ${message}` : message,
  };
}

/**
 * Create a streamer offline error
 */
function createStreamerOfflineError(
  message: string,
  platform: string,
  streamerName: string,
  statusCode: number
): StreamerOfflineError {
  const template = getErrorMessage(ChatErrorType.STREAMER_OFFLINE);
  return {
    type: ChatErrorType.STREAMER_OFFLINE,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    platform: platform || 'unknown',
    streamerName: streamerName || 'Unknown',
    technicalDetails: `Status ${statusCode}: ${message}`,
  };
}

/**
 * Create a platform API error
 */
function createPlatformApiError(
  message: string,
  platform: string,
  platformMessage?: string,
  statusCode?: number
): PlatformApiError {
  const template = getErrorMessage(ChatErrorType.PLATFORM_API_ERROR);
  return {
    type: ChatErrorType.PLATFORM_API_ERROR,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    platform: platform || 'unknown',
    platformMessage,
    statusCode,
    technicalDetails: `Status ${statusCode}: ${message}${platformMessage ? ` (Platform: ${platformMessage})` : ''}`,
  };
}

/**
 * Create a network error
 */
function createNetworkError(message: string, statusCode: number): NetworkError {
  const template = getErrorMessage(ChatErrorType.NETWORK_ERROR);
  return {
    type: ChatErrorType.NETWORK_ERROR,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    statusCode,
    technicalDetails: `Status ${statusCode}: ${message}`,
  };
}

/**
 * Create a validation error
 */
function createValidationError(
  message: string,
  field?: string,
  constraint?: string,
  statusCode?: number
): ValidationError {
  const template = getErrorMessage(ChatErrorType.VALIDATION_ERROR);
  return {
    type: ChatErrorType.VALIDATION_ERROR,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    field,
    constraint,
    technicalDetails: statusCode ? `Status ${statusCode}: ${message}` : message,
  };
}

/**
 * Create an unknown error
 */
function createUnknownError(message: string, originalData?: any, statusCode?: number): UnknownError {
  const template = getErrorMessage(ChatErrorType.UNKNOWN_ERROR);
  return {
    type: ChatErrorType.UNKNOWN_ERROR,
    message: message || template.message,
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    originalError: JSON.stringify(originalData),
    statusCode,
    technicalDetails: statusCode
      ? `Status ${statusCode}: ${message}`
      : message || 'No additional details available',
  };
}

/**
 * Extract streamer name from error message
 */
function extractStreamerName(errorText: string): string {
  // Try to extract streamer name from common patterns
  const patterns = [
    /not currently live on (\w+)/i,
    /streamer (\w+) is offline/i,
    /(\w+) is not live/i,
  ];

  for (const pattern of patterns) {
    const match = errorText.match(pattern);
    if (match && match[1]) {
      return match[1];
    }
  }

  return 'Unknown';
}

/**
 * Parse error from fetch exception (network failure, etc.)
 */
export function parseFetchError(error: any): ChatError {
  const template = getErrorMessage(ChatErrorType.NETWORK_ERROR);
  return {
    type: ChatErrorType.NETWORK_ERROR,
    message: error.message || 'Network request failed',
    userMessage: template.message,
    actionableSteps: template.actionableSteps,
    technicalDetails: error.message || 'Network request failed',
  };
}
