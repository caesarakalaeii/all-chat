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
 * TikTok username validation.
 *
 * TikTok usernames consist of letters, digits, underscores, and dots, with a
 * length of 2–24 characters. This mirrors the constraints enforced by the
 * TikTok platform and prevents malformed or hostile input from reaching the
 * WebSocket connection layer (audit L24).
 */

/** Maximum length of a TikTok username. */
export const TIKTOK_USERNAME_MAX_LENGTH = 24;

/** Minimum length of a TikTok username. */
export const TIKTOK_USERNAME_MIN_LENGTH = 2;

/**
 * Regular expression matching structurally-valid TikTok usernames.
 *
 * Accepts {@link TIKTOK_USERNAME_MIN_LENGTH}–{@link TIKTOK_USERNAME_MAX_LENGTH}
 * characters from the set `[letters, digits, underscore, dot]`. The length bounds
 * are derived from the exported constants so the documented and enforced bounds
 * cannot silently diverge.
 *
 * This is a permissive char-class + length check, not a full TikTok-handle
 * validator: it intentionally rejects whitespace, `@`, `/`, backslashes, control
 * characters and any non-ASCII/unicode (which is what matters for safely passing
 * the value to the downstream live-status lookup). It does NOT enforce TikTok's
 * cosmetic rules (e.g. no leading/trailing dot), so a structurally-odd handle
 * like `..` passes here and simply fails the lookup. (Underscores and dots are
 * allowed at any position, including the edges — matching TikTok, which permits
 * leading/trailing underscores.)
 */
export const TIKTOK_USERNAME_PATTERN = new RegExp(
  `^[a-zA-Z0-9_.]{${TIKTOK_USERNAME_MIN_LENGTH},${TIKTOK_USERNAME_MAX_LENGTH}}$`,
);

/**
 * Result of TikTok username validation.
 */
export interface TikTokUsernameValidationResult {
  /** Whether the username is valid. */
  valid: boolean;
  /** Human-readable reason when invalid. */
  reason?: string;
}

/**
 * Validate a TikTok username.
 *
 * Rejects empty strings, values with leading/trailing whitespace, values that
 * exceed the length bounds, and any character outside the allowed set. The
 * check is defensive: it is not a guarantee that the username exists on TikTok,
 * only that it is structurally plausible and safe to pass downstream.
 *
 * @param username TikTok username (without leading @).
 * @returns validation result with `valid` flag and optional `reason`.
 */
export function validateTikTokUsername(username: string): TikTokUsernameValidationResult {
  if (typeof username !== 'string') {
    return { valid: false, reason: 'username must be a string' };
  }
  if (username.length === 0) {
    return { valid: false, reason: 'username is empty' };
  }
  if (!TIKTOK_USERNAME_PATTERN.test(username)) {
    return {
      valid: false,
      reason: `username must match ${TIKTOK_USERNAME_PATTERN.source} (2-24 chars: letters, digits, underscore, dot)`,
    };
  }
  return { valid: true };
}

/**
 * Validate a TikTok username, throwing on invalid input.
 *
 * Use at trust boundaries where an invalid username is a caller bug and should
 * abort the operation rather than be logged and skipped.
 *
 * @param username TikTok username (without leading @).
 * @throws {Error} when the username is invalid.
 */
export function assertValidTikTokUsername(username: string): void {
  const result = validateTikTokUsername(username);
  if (!result.valid) {
    throw new Error(`invalid TikTok username: ${result.reason}`);
  }
}
