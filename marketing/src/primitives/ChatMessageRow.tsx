/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * Faithful reproduction of the default chat overlay row from
 * `frontend/src/app/overlay/[id]/page.tsx:917-1117`. Default visual settings
 * (no theme applied) renders each message as:
 *
 *   <div class="backdrop-blur-sm rounded-lg p-3 shadow-lg bg-gray-900/90">
 *     <div class="flex items-start gap-3">
 *       <UserAvatar size={40} />
 *       <div class="flex-1 min-w-0">
 *         <div class="flex items-center gap-2 mb-1 flex-wrap">
 *           <PlatformBadge platform={p} />     {/* "● TWITCH" pill }
 *           <span class="font-semibold text-sm" style={color: user.color}>
 *             {display_name}
 *           </span>
 *         </div>
 *         <div class="text-white break-words">{...}</div>
 *         <div class="text-xs text-gray-500 mt-1">{timestamp}</div>
 *       </div>
 *     </div>
 *   </div>
 *
 * Differences from the live page:
 *   - `next/image` → `remotion`'s `<Img>` (deterministic preloading)
 *   - Message timestamps are rendered as a stable wall-clock format (the live
 *     page uses `new Date().toLocaleTimeString()` which is per-render-host;
 *     for a video we want determinism so the timestamp is computed from the
 *     mock message's index and a base time).
 */

import React from 'react';
import { Img } from 'remotion';
import { PlatformBadge } from './PlatformBadge';
import { PLATFORM_HEX } from './platform-colors';
import type { ChatMessage } from './types';

type PositionedEmote = {
  start: number;
  end: number;
  url?: string;
  code: string;
  provider: string;
  key: string;
};

/* Adapted from frontend/src/lib/renderMessage.tsx — same behaviour. */
function renderMessageContent(message: ChatMessage): React.ReactNode {
  const text = message.message?.text ?? '';
  const emotes = message.message?.emotes ?? [];
  if (!text || emotes.length === 0) return text;

  const positioned: PositionedEmote[] = [];
  emotes.forEach((emote, emoteIndex) => {
    if (!emote.positions || emote.positions.length === 0) return;
    emote.positions.forEach((pos, occurrenceIndex) => {
      if (!Array.isArray(pos) || pos.length !== 2) return;
      const [start, end] = pos;
      if (typeof start !== 'number' || typeof end !== 'number') return;
      positioned.push({
        start,
        end,
        url: emote.url,
        code: emote.code,
        provider: emote.provider,
        key: `${emote.code}-${emoteIndex}-${occurrenceIndex}-${start}`,
      });
    });
  });
  if (positioned.length === 0) return text;
  positioned.sort((a, b) => a.start - b.start);

  const nodes: React.ReactNode[] = [];
  let cursor = 0;
  positioned.forEach((emote, index) => {
    if (emote.start < cursor || emote.start >= text.length || emote.end >= text.length) return;
    if (emote.start > cursor) {
      nodes.push(<span key={`text-${index}-${cursor}`}>{text.slice(cursor, emote.start)}</span>);
    }
    if (!emote.url) {
      nodes.push(
        <span key={`${emote.key}-text`} style={{ margin: '0 2px' }}>
          {text.slice(emote.start, emote.end + 1)}
        </span>
      );
    } else {
      nodes.push(
        <Img
          key={emote.key}
          src={emote.url}
          alt={emote.code}
          style={{
            display: 'inline-block',
            height: '1.4em',
            width: 'auto',
            verticalAlign: 'text-bottom',
            margin: '0 2px',
          }}
        />
      );
    }
    cursor = emote.end + 1;
  });
  if (cursor < text.length) {
    nodes.push(<span key={`text-tail-${cursor}`}>{text.slice(cursor)}</span>);
  }
  return nodes;
}

export interface ChatMessageRowProps {
  message: ChatMessage;
  /** 0..1 — slide-in / fade-in amount. 1 = fully visible. */
  enter: number;
  /** Tints the username if `message.user.color` is missing. */
  fallbackColor?: string;
  /** Fixed wall-clock-style timestamp. Defaults to "20:25". */
  timestampLabel?: string;
  /**
   * Multiplier applied to avatar size, padding, and font sizes. Defaults to 1.
   * Portrait scenes pass ~1.4 to keep chat rows legible on a 1080-wide canvas.
   */
  scale?: number;
}

/* Avatar fallback (matches UserAvatar with no avatar_url). */
const AvatarFallback: React.FC<{ displayName: string; size: number }> = ({ displayName, size }) => {
  const initial = displayName.charAt(0).toUpperCase() || '?';
  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        background: 'var(--color-surface-2)',
        color: 'var(--color-text-sub)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: Math.round(size * 0.4),
        fontWeight: 500,
        flexShrink: 0,
      }}
    >
      {initial}
    </div>
  );
};

export const ChatMessageRow: React.FC<ChatMessageRowProps> = ({
  message,
  enter,
  fallbackColor,
  timestampLabel = '20:25',
  scale = 1,
}) => {
  const userColor = message.user.color ?? fallbackColor ?? '#FFFFFF';
  const translateX = (1 - enter) * 60;
  const opacity = enter;
  const avatarSize = Math.round(40 * scale);

  return (
    <div
      style={{
        /* Matches: backdrop-blur-sm rounded-lg p-3 shadow-lg bg-gray-900/90 */
        background: 'rgba(17, 24, 39, 0.9)' /* tailwind gray-900 at 90% */,
        backdropFilter: 'blur(8px)',
        WebkitBackdropFilter: 'blur(8px)',
        borderRadius: 12 * scale,
        padding: 16 * scale,
        boxShadow: '0 10px 15px -3px rgba(0,0,0,0.3), 0 4px 6px -4px rgba(0,0,0,0.25)',
        transform: `translateX(${translateX}px)`,
        opacity,
      }}
    >
      <div
        style={{ display: 'flex', alignItems: 'flex-start', gap: 12 * scale }}
      >
        {/* Avatar */}
        {message.user.avatar_url ? (
          <Img
            src={message.user.avatar_url}
            alt={message.user.display_name}
            style={{
              width: avatarSize,
              height: avatarSize,
              borderRadius: '50%',
              objectFit: 'cover',
              flexShrink: 0,
            }}
          />
        ) : (
          <AvatarFallback displayName={message.user.display_name} size={avatarSize} />
        )}

        {/* Content */}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          {/* Username row: PlatformBadge + display_name */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8 * scale,
              marginBottom: 4 * scale,
              flexWrap: 'wrap',
            }}
          >
            <PlatformBadge platform={message.platform} size={scale > 1.2 ? 'default' : 'default'} />
            <span
              style={{
                fontWeight: 600,
                fontSize: 14 * scale,
                color: userColor,
              }}
            >
              {message.user.display_name}
            </span>
          </div>

          {/* Message body — visualSettings default --chat-font-size is 16px */}
          <div
            style={{
              color: '#FFFFFF',
              fontSize: 16 * scale,
              lineHeight: 1.5,
              wordBreak: 'break-word',
              overflowWrap: 'break-word',
            }}
          >
            {renderMessageContent(message)}
          </div>

          {/* Timestamp */}
          <div
            style={{
              fontSize: 11 * scale,
              color: '#6b7280' /* tailwind gray-500 */,
              marginTop: 4 * scale,
              fontFamily: 'var(--font-mono)',
            }}
          >
            {timestampLabel}
          </div>
        </div>
      </div>
    </div>
  );
};

/* Suppress lint warning for the unused PLATFORM_HEX import — kept available
   for downstream variants that might want platform-tinted accents. */
void PLATFORM_HEX;
