'use client';

import { useEffect, useLayoutEffect, useRef, useCallback, useState } from 'react';
import type { ChatMessage } from '@/lib/types/message';
import { renderMessageContent } from '@/lib/renderMessage';

// ---- Constants -------------------------------------------------------------

const GAP_PX = 24;        // px gap between messages
const EXIT_DURATION = 280; // ms for the slide-out-left collapse animation
const ENTER_DURATION = 200; // ms for new message fade-in

// ---- Types -----------------------------------------------------------------

interface TickerEntry {
  message: ChatMessage;
  entryKey: string;
}

interface TickerOverlayProps {
  messages: ChatMessage[];
  speed: number;    // kept for API compat, not used for static layout
  fontSize: number;
  customCss?: string;
  showPlatformBadge: boolean;
  platformBadgeStyle: 'text' | 'icon';
  platformBadgePosition: 'before' | 'after';
}

// ---- Platform helpers ------------------------------------------------------

const PLATFORM_COLORS: Record<string, string> = {
  twitch: '#9146FF',
  youtube: '#FF0000',
  kick: '#00E701',
  tiktok: '#69C9D0',
};

function getPlatformLabel(platform: string): string {
  return platform.charAt(0).toUpperCase() + platform.slice(1);
}

// ---- TickerOverlay ---------------------------------------------------------

export default function TickerOverlay({
  messages,
  fontSize,
  customCss,
  showPlatformBadge,
  platformBadgeStyle,
  platformBadgePosition,
}: TickerOverlayProps) {
  const [entries, setEntries] = useState<TickerEntry[]>([]);
  const seenIdsRef = useRef<Set<string>>(new Set());
  const containerRef = useRef<HTMLDivElement>(null);
  // Map of entryKey → DOM element for each visible message
  const itemRefsMap = useRef<Map<string, HTMLDivElement>>(new Map());
  // Keys currently mid-exit animation — don't re-trigger
  const evictingRef = useRef<Set<string>>(new Set());

  // Queue newly arrived messages
  useEffect(() => {
    const newMessages = messages.filter(m => !seenIdsRef.current.has(m.id));
    if (newMessages.length === 0) return;

    newMessages.forEach(m => seenIdsRef.current.add(m.id));
    setEntries(prev => [
      ...prev,
      ...newMessages.map(m => ({
        message: m,
        entryKey: `${m.id}-${Date.now()}`,
      })),
    ]);
  }, [messages]);

  // After each render: evict oldest entries if the row overflows the container
  useLayoutEffect(() => {
    if (!containerRef.current || entries.length === 0) return;

    const containerWidth = containerRef.current.offsetWidth;

    // Sum widths of all non-evicting items
    let totalWidth = 0;
    for (const entry of entries) {
      if (evictingRef.current.has(entry.entryKey)) continue;
      const el = itemRefsMap.current.get(entry.entryKey);
      if (el) totalWidth += el.offsetWidth + GAP_PX;
    }

    if (totalWidth <= containerWidth) return;

    // Evict oldest entries until we fit
    let excess = totalWidth - containerWidth;

    for (const entry of entries) {
      if (excess <= 0) break;
      if (evictingRef.current.has(entry.entryKey)) continue;

      const el = itemRefsMap.current.get(entry.entryKey);
      if (!el) continue;

      const w = el.offsetWidth;
      evictingRef.current.add(entry.entryKey);
      excess -= w + GAP_PX;

      // Collapse the element: shrink its width to 0 while fading out.
      // The flex gap disappears automatically as the element collapses.
      const key = entry.entryKey;
      el.animate(
        [
          { maxWidth: `${w}px`, opacity: 1, overflow: 'hidden' },
          { maxWidth: '0px',    opacity: 0, overflow: 'hidden' },
        ],
        { duration: EXIT_DURATION, easing: 'ease-in', fill: 'forwards' },
      ).onfinish = () => {
        evictingRef.current.delete(key);
        itemRefsMap.current.delete(key);
        setEntries(prev => prev.filter(e => e.entryKey !== key));
      };
    }
  }, [entries]);

  const setItemRef = useCallback((key: string) => (el: HTMLDivElement | null) => {
    if (el) {
      itemRefsMap.current.set(key, el);
      // Fade-in for new messages
      el.animate(
        [{ opacity: 0 }, { opacity: 1 }],
        { duration: ENTER_DURATION, easing: 'ease-out', fill: 'forwards' },
      );
    } else {
      itemRefsMap.current.delete(key);
    }
  }, []);

  return (
    <>
      <style dangerouslySetInnerHTML={{
        __html: `
          body {
            overflow: hidden !important;
            background: transparent !important;
          }
          body::-webkit-scrollbar { display: none !important; }
          * { scrollbar-width: none !important; -ms-overflow-style: none !important; }
        `,
      }} />
      {customCss && customCss.trim().length > 0 && (
        <style dangerouslySetInnerHTML={{ __html: customCss }} />
      )}

      <div
        ref={containerRef}
        className="ticker-container"
        style={{
          position: 'fixed',
          bottom: 0,
          left: 0,
          right: 0,
          height: '60px',
          overflow: 'hidden',
          background: 'rgba(0, 0, 0, 0.75)',
          display: 'flex',
          alignItems: 'center',
          gap: `${GAP_PX}px`,
          padding: '0 12px',
        }}
      >
        {entries.map(entry => (
          <div
            key={entry.entryKey}
            ref={setItemRef(entry.entryKey)}
            className="ticker-message"
            data-platform={entry.message.platform}
            style={{
              flexShrink: 0,
              whiteSpace: 'nowrap',
              fontSize: `${fontSize}px`,
              color: '#ffffff',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              opacity: 0, // starts invisible; fade-in applied via animate()
            }}
          >
            {showPlatformBadge && platformBadgePosition === 'before' && (
              <span style={{
                color: PLATFORM_COLORS[entry.message.platform] ?? '#aaa',
                fontSize: `${Math.max(10, fontSize - 2)}px`,
                fontWeight: 700,
                textTransform: 'uppercase',
                flexShrink: 0,
              }}>
                {platformBadgeStyle === 'text'
                  ? getPlatformLabel(entry.message.platform)
                  : getPlatformLabel(entry.message.platform).slice(0, 1)}
              </span>
            )}

            <span style={{ color: entry.message.user?.color || '#ffffff', fontWeight: 600, flexShrink: 0 }}>
              {entry.message.user?.display_name || entry.message.user?.username}:
            </span>

            <span className="ticker-message-text">
              {renderMessageContent(entry.message)}
            </span>

            {showPlatformBadge && platformBadgePosition === 'after' && (
              <span style={{
                color: PLATFORM_COLORS[entry.message.platform] ?? '#aaa',
                fontSize: `${Math.max(10, fontSize - 2)}px`,
                fontWeight: 700,
                textTransform: 'uppercase',
                flexShrink: 0,
              }}>
                {platformBadgeStyle === 'text'
                  ? getPlatformLabel(entry.message.platform)
                  : getPlatformLabel(entry.message.platform).slice(0, 1)}
              </span>
            )}
          </div>
        ))}
      </div>
    </>
  );
}
