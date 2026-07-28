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
 * Auth-funnel analytics helpers.
 *
 * The provider-agnostic OAuth callback (`/auth/callback`) historically attributed
 * `signin_completed` from `user.auth_provider ?? 'unknown'`, which was empty ~58%
 * of the time (the field is optional and often unset), so most completions could
 * not be tied to a platform. `resolveSigninPlatform` fixes that with a precedence
 * chain: the platform the user just clicked (stashed in sessionStorage before the
 * OAuth redirect) → `auth_provider` when it's a known value → inference from the
 * provider id fields on the exchanged user → `'unknown'`.
 *
 * The viewer helpers keep the viewer/extension auth funnel within the analytics
 * privacy contract (see lib/analytics.ts): platform is validated against a known
 * allow-list and the free-form backend error string is bucketed into a bounded
 * enum, so raw backend text (which could carry detail) never reaches Umami.
 */

/** Platforms a streamer can sign in with (the three OAuth login buttons). */
export const STREAMER_PLATFORMS = ['twitch', 'youtube', 'kick'] as const
export type StreamerPlatform = (typeof STREAMER_PLATFORMS)[number]

/** Platforms a viewer auth can name (superset — TikTok/Discord viewer flows too). */
export const VIEWER_PLATFORMS = ['twitch', 'youtube', 'kick', 'tiktok', 'discord'] as const

/** sessionStorage key holding the platform the user just clicked to sign in with. */
export const SIGNIN_PLATFORM_KEY = 'allchat:signin_platform'

/** The subset of `User` we need to attribute a sign-in to a platform. */
type ProviderIdentity =
  | {
      auth_provider?: string | null
      twitch_id?: string | null
      google_id?: string | null
      kick_id?: string | null
    }
  | null
  | undefined

function isStreamerPlatform(v: string | null | undefined): v is StreamerPlatform {
  return !!v && (STREAMER_PLATFORMS as readonly string[]).includes(v)
}

/**
 * Resolve the platform for a completed sign-in. Precedence: stashed click →
 * known `auth_provider` → provider-id inference → `'unknown'`. Pure/deterministic.
 */
export function resolveSigninPlatform(user: ProviderIdentity, stashed?: string | null): string {
  if (isStreamerPlatform(stashed)) return stashed
  if (isStreamerPlatform(user?.auth_provider)) return user!.auth_provider as string
  // Inference fallback from linked ids — only when unambiguous. A multi-linked
  // account (e.g. Twitch + YouTube) with no stash and no known auth_provider is
  // genuinely ambiguous, so report 'unknown' rather than guess and skew the
  // per-platform sign-in stats toward whichever id we happen to check first.
  const linked = [
    user?.twitch_id ? 'twitch' : null,
    user?.google_id ? 'youtube' : null, // YouTube auth populates google_id
    user?.kick_id ? 'kick' : null,
  ].filter((p): p is string => p !== null)
  return linked.length === 1 ? linked[0] : 'unknown'
}

/**
 * Remember which platform the user is signing in with, right before the OAuth
 * redirect. sessionStorage survives the same-origin return from the provider and
 * is per-tab, so it is the most accurate "which button did they click" signal.
 * Best-effort: no-ops when storage is unavailable (SSR, private mode).
 */
export function stashSigninPlatform(platform: StreamerPlatform): void {
  try {
    if (typeof sessionStorage !== 'undefined') {
      sessionStorage.setItem(SIGNIN_PLATFORM_KEY, platform)
    }
  } catch {
    // storage unavailable — attribution just falls back to id inference
  }
}

/** Read and immediately clear the stashed platform (so a later callback can't reuse it). */
export function readAndClearSigninPlatform(): string | null {
  try {
    if (typeof sessionStorage === 'undefined') return null
    const v = sessionStorage.getItem(SIGNIN_PLATFORM_KEY)
    if (v) sessionStorage.removeItem(SIGNIN_PLATFORM_KEY)
    return v
  } catch {
    return null
  }
}

/** Validate a viewer platform query param against the known allow-list. */
export function sanitizeViewerPlatform(raw: string | null | undefined): string {
  return raw && (VIEWER_PLATFORMS as readonly string[]).includes(raw) ? raw : 'unknown'
}

/**
 * Bucket a free-form backend auth-error string into a bounded, non-PII enum slug.
 * An unrecognized message returns `'other'` — it is NEVER echoed verbatim, so raw
 * backend text can't leak into analytics.
 */
export function bucketViewerAuthError(raw: string | null | undefined): string {
  if (!raw) return 'unknown'
  const s = raw.toLowerCase()
  if (s.includes('denied') || s.includes('cancel')) return 'access_denied'
  if (s.includes('expire')) return 'code_expired'
  if (s.includes('scope') || s.includes('permission')) return 'insufficient_scope'
  if (s.includes('no_code') || s.includes('no authentication code')) return 'no_code'
  // Word-boundary match so 'abandoned' / 'banner' aren't classified as bans.
  if (/\bban(ned)?\b/.test(s) || s.includes('suspend')) return 'banned'
  if (s.includes('exchange') || s.includes('token')) return 'exchange_failed'
  return 'other'
}
