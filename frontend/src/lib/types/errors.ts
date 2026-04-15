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
 * Chat Error Type System
 *
 * Provides type-safe error handling with discriminated unions for chat message sending errors.
 * Each error type includes specific metadata and user-friendly messages.
 */

export enum ChatErrorType {
  // Authentication errors
  UNAUTHORIZED = 'UNAUTHORIZED',
  TOKEN_EXPIRED = 'TOKEN_EXPIRED',

  // Rate limiting
  RATE_LIMITED = 'RATE_LIMITED',

  // User status
  BANNED = 'BANNED',

  // Stream status
  STREAMER_OFFLINE = 'STREAMER_OFFLINE',

  // Platform errors
  PLATFORM_API_ERROR = 'PLATFORM_API_ERROR',

  // Network errors
  NETWORK_ERROR = 'NETWORK_ERROR',

  // Validation errors
  VALIDATION_ERROR = 'VALIDATION_ERROR',

  // Unknown/unexpected errors
  UNKNOWN_ERROR = 'UNKNOWN_ERROR',
}

/**
 * Base error interface with common fields
 */
interface BaseChatError {
  type: ChatErrorType
  message: string
  userMessage: string // User-friendly message
  actionableSteps: string[] // What the user can do
  technicalDetails?: string // Optional technical details for debugging
}

/**
 * Unauthorized - User needs to authenticate
 */
export interface UnauthorizedError extends BaseChatError {
  type: ChatErrorType.UNAUTHORIZED
  platform: string
}

/**
 * Token Expired - User needs to re-authenticate
 */
export interface TokenExpiredError extends BaseChatError {
  type: ChatErrorType.TOKEN_EXPIRED
  platform: string
}

/**
 * Rate Limited - User sent too many messages
 */
export interface RateLimitedError extends BaseChatError {
  type: ChatErrorType.RATE_LIMITED
  resetTime?: string // ISO timestamp when rate limit resets
  retryAfter?: number // Seconds until retry allowed
}

/**
 * Banned - User is banned from sending messages
 */
export interface BannedError extends BaseChatError {
  type: ChatErrorType.BANNED
  reason?: string
  expiresAt?: string // ISO timestamp when ban expires (if temporary)
}

/**
 * Streamer Offline - Stream is not currently live
 */
export interface StreamerOfflineError extends BaseChatError {
  type: ChatErrorType.STREAMER_OFFLINE
  platform: string
  streamerName: string
}

/**
 * Platform API Error - The platform's API returned an error
 */
export interface PlatformApiError extends BaseChatError {
  type: ChatErrorType.PLATFORM_API_ERROR
  platform: string
  platformMessage?: string // Error message from the platform
  statusCode?: number
}

/**
 * Network Error - Request failed due to network issues
 */
export interface NetworkError extends BaseChatError {
  type: ChatErrorType.NETWORK_ERROR
  statusCode?: number
}

/**
 * Validation Error - Message failed validation
 */
export interface ValidationError extends BaseChatError {
  type: ChatErrorType.VALIDATION_ERROR
  field?: string
  constraint?: string // What validation rule was violated
}

/**
 * Unknown Error - Unexpected error occurred
 */
export interface UnknownError extends BaseChatError {
  type: ChatErrorType.UNKNOWN_ERROR
  originalError?: string
  statusCode?: number
}

/**
 * Discriminated union of all error types
 */
export type ChatError =
  | UnauthorizedError
  | TokenExpiredError
  | RateLimitedError
  | BannedError
  | StreamerOfflineError
  | PlatformApiError
  | NetworkError
  | ValidationError
  | UnknownError

/**
 * Type guard to check if an error is a specific type
 */
export function isChatError(error: any): error is ChatError {
  return error && typeof error === 'object' && 'type' in error && error.type in ChatErrorType
}

/**
 * Type guard for rate limited error
 */
export function isRateLimitedError(error: ChatError): error is RateLimitedError {
  return error.type === ChatErrorType.RATE_LIMITED
}

/**
 * Type guard for banned error
 */
export function isBannedError(error: ChatError): error is BannedError {
  return error.type === ChatErrorType.BANNED
}

/**
 * Type guard for authentication errors (unauthorized or token expired)
 */
export function isAuthError(error: ChatError): error is UnauthorizedError | TokenExpiredError {
  return error.type === ChatErrorType.UNAUTHORIZED || error.type === ChatErrorType.TOKEN_EXPIRED
}

/**
 * Type guard for platform API error
 */
export function isPlatformApiError(error: ChatError): error is PlatformApiError {
  return error.type === ChatErrorType.PLATFORM_API_ERROR
}
