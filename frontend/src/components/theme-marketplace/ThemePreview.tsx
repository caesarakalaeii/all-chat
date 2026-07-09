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
 * Theme Preview Component
 *
 * Renders a preview of a theme with sample messages.
 */

'use client'

import { useEffect, useState } from 'react'
import clsx from 'clsx'
import type { ChatMessagePreview } from '@/lib/theme-marketplace/types'
import { PLATFORM_COLORS, type Platform } from '@/lib/platform-colors'
import { rewriteThemeFontImports } from '@/lib/theme-marketplace/font-proxy'
import { PlatformGlyph } from '@/components/overlay/PlatformGlyph'

/** Static sample time for the preview timestamp — deterministic (no Date() in render). */
const SAMPLE_TIME = '12:34'

interface ThemePreviewProps {
  css: string
  messages: ChatMessagePreview[]
  themeId: string
  /** Preview viewport height in px. Defaults to the marketplace card size. */
  height?: number
  /** Platform-badge style, mirroring the overlay's setting. Defaults to text. */
  platformBadge?: 'text' | 'icon'
  /** Render the per-message timestamp. Defaults to true (matches the live overlay). */
  showTimestamp?: boolean
  /** Size the preview to its content instead of a fixed `height` (no scrollbar, no
   *  empty padding) — used by the landing carousel so each theme shows at its true
   *  height. Defaults to false (fixed-height, scrollable marketplace card). */
  fit?: boolean
}

/**
 * Scope CSS to prevent leaking outside preview container
 * (Reused from preview page)
 */
const scopeCustomCss = (css: string, scopeSelector: string, bodySelector: string): string => {
  if (!css.trim()) {
    return ''
  }

  const replaceBody = css.replace(/:root/gi, scopeSelector).replace(/\bbody\b/gi, bodySelector)

  return replaceBody.replace(/(^|}|{)\s*([^@}{]+)\s*{/g, (match, prefix, selectorGroup) => {
    const trimmed = selectorGroup.trim()
    if (!trimmed) {
      return match
    }

    const isKeyframeStep =
      ['from', 'to'].includes(trimmed.toLowerCase()) || /^\d+\.?\d*%$/i.test(trimmed)
    if (isKeyframeStep) {
      return `${prefix} ${trimmed} {`
    }

    const scopedSelectors = trimmed
      .split(',')
      .map((selector: string) => {
        const sel = selector.trim()
        if (!sel || sel.startsWith(scopeSelector) || sel.startsWith(bodySelector)) {
          return sel
        }
        return `${scopeSelector} ${sel}`
      })
      .filter(Boolean)
      .join(', ')

    return `${prefix} ${scopedSelectors} {`
  })
}

export default function ThemePreview({
  css,
  messages,
  themeId,
  height = 180,
  platformBadge = 'text',
  showTimestamp = true,
  fit = false,
}: ThemePreviewProps) {
  const [scopedCss, setScopedCss] = useState('')
  const uniqueId = `theme-preview-${themeId}`

  // Scope CSS when it changes. Route Google-Fonts @imports through the same-origin
  // /api/fonts/css proxy first (like the real overlay/embed render paths do) — the
  // site CSP blocks direct fonts.googleapis.com loads, so without this the preview
  // silently falls back to a system font. The proxy also keeps viewer IPs off Google
  // (DSGVO). See lib/theme-marketplace/font-proxy.ts.
  useEffect(() => {
    const scoped = scopeCustomCss(
      rewriteThemeFontImports(css),
      `.${uniqueId}`,
      `.${uniqueId} .theme-preview-body`
    )
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setScopedCss(scoped)
  }, [css, uniqueId])

  return (
    <div className="theme-preview-wrapper overflow-hidden rounded-t-lg border border-border bg-surface">
      {/* Scoped styles - use data attribute to create unique scope */}
      {scopedCss && (
        <style dangerouslySetInnerHTML={{ __html: scopedCss }} data-theme-id={themeId} />
      )}

      {/* Preview container with unique data attribute */}
      <div
        className={uniqueId}
        data-theme-preview={themeId}
        style={{
          height: fit ? 'auto' : `${height}px`,
          background: 'black',
          overflow: 'hidden',
          position: 'relative',
          isolation: 'isolate',
        }}
      >
        {/*
          Sample markup mirrors the live overlay renderer (app/overlay/[id]/page.tsx):
          same class hooks (.overlay-live-body, .chat-message, .platform-badge[-text|-icon],
          .chat-username, .break-words), data-platform attribute, badge order (platform →
          badges → username) and timestamp — so a theme previews exactly as it renders live.
          text-left keeps it faithful even when a centered container (e.g. the hero) wraps it.
        */}
        <div
          className={clsx(
            'theme-preview-body overlay-live-body space-y-3 p-2 text-left',
            fit ? 'overflow-hidden' : 'h-full overflow-y-auto'
          )}
        >
          {messages.map((msg) => (
            <div
              key={msg.id}
              data-platform={msg.platform}
              data-username={msg.user.username}
              className="chat-message rounded-lg bg-slate-900/90 p-3 shadow-lg backdrop-blur-sm"
            >
              <div className="flex items-start gap-3">
                {/* Avatar */}
                <div className="flex-shrink-0">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={msg.user.avatar_url}
                    alt={msg.user.display_name}
                    className="h-10 w-10 rounded-full"
                  />
                </div>

                {/* Content */}
                <div className="min-w-0 flex-1">
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    {/* Platform badge — text or icon, matching the overlay's setting */}
                    {platformBadge === 'icon' ? (
                      <span
                        className="platform-badge platform-badge-icon flex items-center gap-0.5"
                        title={msg.platform}
                      >
                        <PlatformGlyph platform={msg.platform} />
                      </span>
                    ) : (
                      <span
                        className={clsx(
                          'platform-badge platform-badge-text text-xs font-semibold uppercase',
                          PLATFORM_COLORS[msg.platform as Platform]?.text ??
                            PLATFORM_COLORS.system.text
                        )}
                      >
                        {msg.platform}
                      </span>
                    )}

                    {/* User badges */}
                    {msg.user.badges.some((b) => b.icon_url && !b.icon_url.startsWith('/')) && (
                      <div className="flex items-center gap-1">
                        {msg.user.badges
                          .filter((b) => b.icon_url && !b.icon_url.startsWith('/'))
                          .map((badge, idx) => (
                            /* eslint-disable-next-line @next/next/no-img-element */
                            <img
                              key={idx}
                              src={badge.icon_url}
                              alt={badge.name}
                              className="h-[1em] w-auto object-contain"
                              onError={(e) => {
                                ;(e.target as HTMLImageElement).style.display = 'none'
                              }}
                            />
                          ))}
                      </div>
                    )}

                    {/* Username */}
                    <span
                      className="chat-username text-sm font-semibold"
                      style={{ color: msg.user.color }}
                    >
                      {msg.user.display_name}
                    </span>
                  </div>

                  {/* Message text */}
                  <div className="break-words text-white" style={{ fontSize: '16px' }}>
                    {msg.message.text}
                  </div>

                  {/* Timestamp */}
                  {showTimestamp && (
                    <div className="mt-1 text-xs text-slate-500">{SAMPLE_TIME}</div>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
