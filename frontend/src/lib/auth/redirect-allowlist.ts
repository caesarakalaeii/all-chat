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
 * Client-side allowlist for external OAuth / bot-invite redirects
 * (defense-in-depth — audit L32).
 *
 * The backend returns an `auth_url` (or `bot_invite_url`) pointing to a known
 * OAuth provider. This helper validates the destination host before
 * navigating, so a compromised or rogue backend response cannot redirect the
 * user to an attacker-controlled site.
 *
 * Allowed external hosts: Twitch OAuth (id.twitch.tv, twitch.tv), Google/YouTube
 * OAuth (accounts.google.com), Kick OAuth (id.kick.com), Patreon (patreon.com),
 * Discord (discord.com), and the app's own origin. Relative same-origin paths
 * (starting with `/` but not `//`) are always allowed.
 */

const ALLOWED_EXTERNAL_HOSTS = new Set([
  'twitch.tv',
  'id.twitch.tv',
  // YouTube/streamer + viewer login: oauth2 google.Endpoint → accounts.google.com
  'accounts.google.com',
  // Kick streamer + viewer login: kickAuthURL → id.kick.com/oauth/authorize
  'id.kick.com',
  'patreon.com',
  'discord.com',
])

/**
 * Returns true if `url` is safe to navigate to as an external redirect:
 * same-origin, relative path, or an allowlisted OAuth provider host
 * (including subdomains).
 */
export function isAllowedExternalRedirect(url: string): boolean {
  if (!url) return false

  // Reject backslashes (audit M1): browsers normalize \ → /, so a URL like
  // /\evil.com passes the startsWith('/') && !startsWith('//') guard but
  // navigates to evil.com. Reject any backslash before the relative-path
  // shortcut.
  if (url.indexOf('\\') !== -1) return false

  // Relative same-origin path (but not protocol-relative "//evil.com").
  if (url.startsWith('/') && !url.startsWith('//')) return true

  try {
    const base = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    const parsed = new URL(url, base)

    // Same-origin is always allowed.
    if (typeof window !== 'undefined' && parsed.origin === window.location.origin) {
      return true
    }

    const host = parsed.hostname
    if (ALLOWED_EXTERNAL_HOSTS.has(host)) return true

    // Allow subdomains of an allowlisted host (e.g. www.patreon.com).
    for (const allowed of ALLOWED_EXTERNAL_HOSTS) {
      if (host.endsWith('.' + allowed)) return true
    }

    return false
  } catch {
    return false
  }
}

/**
 * Validates `url` against the allowlist and, if trusted, navigates the browser
 * to it via `window.location.href`. Returns true if the redirect was issued,
 * false if the URL was blocked (also logs a warning).
 */
export function safeExternalRedirect(url: string): boolean {
  if (!isAllowedExternalRedirect(url)) {
    console.warn('[AllChat] Blocked redirect to untrusted host:', url)
    return false
  }
  if (typeof window !== 'undefined') {
    window.location.href = url
  }
  return true
}
