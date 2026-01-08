/**
 * Theme Preview Component
 *
 * Renders a preview of a theme with sample messages.
 */

'use client';

import { useEffect, useState, useMemo, useId } from 'react';
import type { ChatMessagePreview } from '@/lib/theme-marketplace/types';

interface ThemePreviewProps {
  css: string;
  messages: ChatMessagePreview[];
  themeId: string;
}

/**
 * Scope CSS to prevent leaking outside preview container
 * (Reused from preview page)
 */
const scopeCustomCss = (
  css: string,
  scopeSelector: string,
  bodySelector: string
): string => {
  if (!css.trim()) {
    return '';
  }

  const replaceBody = css
    .replace(/:root/gi, scopeSelector)
    .replace(/\bbody\b/gi, bodySelector);

  return replaceBody.replace(
    /(^|}|{)\s*([^@}{]+)\s*{/g,
    (match, prefix, selectorGroup) => {
      const trimmed = selectorGroup.trim();
      if (!trimmed) {
        return match;
      }

      const isKeyframeStep =
        ['from', 'to'].includes(trimmed.toLowerCase()) ||
        /^\d+\.?\d*%$/i.test(trimmed);
      if (isKeyframeStep) {
        return `${prefix} ${trimmed} {`;
      }

      const scopedSelectors = trimmed
        .split(',')
        .map((selector: string) => {
          const sel = selector.trim();
          if (
            !sel ||
            sel.startsWith(scopeSelector) ||
            sel.startsWith(bodySelector)
          ) {
            return sel;
          }
          return `${scopeSelector} ${sel}`;
        })
        .filter(Boolean)
        .join(', ');

      return `${prefix} ${scopedSelectors} {`;
    }
  );
};

/**
 * Get platform color class
 */
function getPlatformColor(platform: string): string {
  switch (platform) {
    case 'twitch':
      return 'text-purple-400';
    case 'youtube':
      return 'text-red-400';
    case 'kick':
      return 'text-green-400';
    case 'tiktok':
      return 'text-gray-400';
    default:
      return 'text-gray-400';
  }
}

export default function ThemePreview({ css, messages, themeId }: ThemePreviewProps) {
  const [scopedCss, setScopedCss] = useState('');
  const uniqueId = `theme-preview-${themeId}`;

  // Scope CSS when it changes
  useEffect(() => {
    const scoped = scopeCustomCss(
      css,
      `.${uniqueId}`,
      `.${uniqueId} .theme-preview-body`
    );
    setScopedCss(scoped);
  }, [css, uniqueId]);

  return (
    <div className="theme-preview-wrapper bg-gray-800 border border-gray-700 rounded-t-lg overflow-hidden">
      {/* Scoped styles */}
      {scopedCss && (
        <style dangerouslySetInnerHTML={{ __html: scopedCss }} />
      )}

      {/* Preview container */}
      <div
        className={uniqueId}
        style={{
          height: '180px',
          background: 'black',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        <div className="theme-preview-body h-full overflow-y-auto p-2 space-y-3">
          {messages.map((msg) => (
            <div
              key={msg.id}
              className="backdrop-blur-sm rounded-lg p-3 shadow-lg bg-gray-900/90"
            >
              <div className="flex items-start gap-3">
                {/* Avatar */}
                <div className="flex-shrink-0">
                  <img
                    src={msg.user.avatar_url}
                    alt={msg.user.display_name}
                    className="w-10 h-10 rounded-full"
                  />
                </div>

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    {/* Platform indicator */}
                    <span
                      className={`text-xs font-semibold uppercase ${getPlatformColor(
                        msg.platform
                      )}`}
                    >
                      {msg.platform}
                    </span>

                    {/* Username */}
                    <span
                      className="font-semibold text-sm"
                      style={{ color: msg.user.color }}
                    >
                      {msg.user.display_name}
                    </span>

                    {/* Badges */}
                    {msg.user.badges.length > 0 && (
                      <div className="flex gap-1">
                        {msg.user.badges.map((badge, idx) => (
                          <img
                            key={idx}
                            src={badge.icon_url}
                            alt={badge.name}
                            className="w-4 h-4"
                          />
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Message text */}
                  <div
                    className="text-white break-words"
                    style={{ fontSize: '16px' }}
                  >
                    {msg.message.text}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
