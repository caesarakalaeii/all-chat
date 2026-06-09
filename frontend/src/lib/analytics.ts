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
 * Analytics event tracking
 *
 * A thin, typed wrapper over Umami's `window.umami.track(...)`. The Umami script
 * is loaded by `components/Analytics.tsx` (production only, `/overlay/*` excluded),
 * so this wrapper must **no-op gracefully** whenever the tracker isn't present:
 * local dev, OBS overlay views, or before the deferred script has loaded.
 *
 * Privacy contract: event payloads stay **non-PII and aggregate** — platform names,
 * counts, enum-like surface/section labels, booleans. Never pass usernames, tokens,
 * message content, overlay IDs, or emails. This keeps analytics within the cookieless,
 * no-consent legal basis documented in the privacy policy and cookie banner.
 */

/** Allowed event names. Keep this union the single source of truth for tracked events. */
export type AnalyticsEvent =
  // Activation funnel
  | 'signin_started'
  | 'signin_completed'
  | 'signin_failed'
  | 'overlay_created'
  | 'source_added'
  | 'source_add_failed'
  | 'obs_url_copied'
  // Feature adoption
  | 'theme_applied'
  | 'custom_css_saved'
  | 'tts_enabled'
  | 'sound_enabled'
  | 'share_requested'
  | 'share_accepted'
  | 'yt_stream_strategy_set'
  // Outbound & CTA
  | 'outbound_click'
  | 'cta_click'
  // Errors & friction
  | 'capacity_limit_hit'

/** Umami only accepts flat primitive values in event data. */
export type EventData = Record<string, string | number | boolean>

declare global {
  interface Window {
    umami?: {
      track: (eventName: string, eventData?: EventData) => void
    }
  }
}

/**
 * Record a custom analytics event. Safe to call from anywhere — it silently does
 * nothing when Umami isn't loaded, and never throws into the calling UX path.
 */
export function trackEvent(name: AnalyticsEvent, data?: EventData): void {
  if (typeof window === 'undefined') return
  try {
    window.umami?.track(name, data)
  } catch {
    // Analytics is best-effort and must never break the user experience.
  }
}
