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

interface ThemePreviewProps {
  css: string
  messages: ChatMessagePreview[]
  themeId: string
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

export default function ThemePreview({ css, messages, themeId }: ThemePreviewProps) {
  const [scopedCss, setScopedCss] = useState('')
  const uniqueId = `theme-preview-${themeId}`

  // Scope CSS when it changes
  useEffect(() => {
    const scoped = scopeCustomCss(css, `.${uniqueId}`, `.${uniqueId} .theme-preview-body`)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setScopedCss(scoped)
  }, [css, uniqueId])

  return (
    <div className="theme-preview-wrapper overflow-hidden rounded-t-lg border border-slate-700 bg-slate-800">
      {/* Scoped styles - use data attribute to create unique scope */}
      {scopedCss && (
        <style dangerouslySetInnerHTML={{ __html: scopedCss }} data-theme-id={themeId} />
      )}

      {/* Preview container with unique data attribute */}
      <div
        className={uniqueId}
        data-theme-preview={themeId}
        style={{
          height: '180px',
          background: 'black',
          overflow: 'hidden',
          position: 'relative',
          isolation: 'isolate',
        }}
      >
        <div className="theme-preview-body h-full space-y-3 overflow-y-auto p-2">
          {messages.map((msg) => (
            <div key={msg.id} className="rounded-lg bg-slate-900 p-3 shadow-lg">
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
                  <div className="mb-1 flex items-center gap-2">
                    {/* Platform indicator */}
                    <span
                      className={clsx(
                        'text-xs font-semibold uppercase',
                        PLATFORM_COLORS[msg.platform as Platform]?.text ??
                          PLATFORM_COLORS.system.text
                      )}
                    >
                      {msg.platform}
                    </span>

                    {/* Username */}
                    <span className="text-sm font-semibold" style={{ color: msg.user.color }}>
                      {msg.user.display_name}
                    </span>

                    {/* Badges */}
                    {msg.user.badges.length > 0 &&
                      msg.user.badges.some((b) => b.icon_url && !b.icon_url.startsWith('/')) && (
                        <div className="flex gap-1">
                          {msg.user.badges
                            .filter((b) => b.icon_url && !b.icon_url.startsWith('/'))
                            .map((badge, idx) => (
                              /* eslint-disable-next-line @next/next/no-img-element */
                              <img
                                key={idx}
                                src={badge.icon_url}
                                alt={badge.name}
                                className="h-4 w-4"
                                onError={(e) => {
                                  ;(e.target as HTMLImageElement).style.display = 'none'
                                }}
                              />
                            ))}
                        </div>
                      )}
                  </div>

                  {/* Message text */}
                  <div className="break-words text-white" style={{ fontSize: '16px' }}>
                    {msg.message.text}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
