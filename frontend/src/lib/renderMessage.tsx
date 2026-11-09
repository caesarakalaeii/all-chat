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

import Image from 'next/image'
import React from 'react'

import type { ChatMessage } from '@/lib/types/message'

type PositionedEmote = {
  start: number
  end: number
  url?: string
  code: string
  provider: string
  key: string
  zeroWidth: boolean
}

/** An emote image plus the zero-width (7TV overlay) emotes drawn on top of it. */
type EmoteStack = {
  base: PositionedEmote & { url: string }
  overlays: PositionedEmote[]
}

function emoteImage(emote: PositionedEmote & { url: string }, className: string) {
  return (
    <Image
      key={emote.key}
      src={emote.url}
      alt=""
      title={`${emote.code} (${emote.provider})`}
      width={28}
      height={28}
      className={className}
    />
  )
}

function renderStack({ base, overlays }: EmoteStack): React.ReactNode {
  if (overlays.length === 0) {
    return emoteImage(base, 'mx-[0.2em] inline-block h-[1.4em] w-auto align-text-bottom')
  }
  // The base sets the box; overlays are centred on it and may overhang it, like
  // 7TV's own renderer, so a wide overlay does not push the text apart.
  return (
    <span key={`${base.key}-stack`} className="relative mx-[0.2em] inline-block align-text-bottom">
      {emoteImage(base, 'block h-[1.4em] w-auto')}
      {overlays.map((overlay) =>
        overlay.url
          ? emoteImage(
              { ...overlay, url: overlay.url },
              'absolute top-0 left-1/2 h-full w-auto max-w-none -translate-x-1/2'
            )
          : null
      )}
    </span>
  )
}

export function renderMessageContent(message: ChatMessage): React.ReactNode {
  const text = message.message?.text ?? ''
  const emotes = message.message?.emotes ?? []

  if (!text || emotes.length === 0) {
    return text
  }

  const positioned: PositionedEmote[] = []

  emotes.forEach((emote, emoteIndex) => {
    if (!emote.positions || emote.positions.length === 0) {
      return
    }

    emote.positions.forEach((pos, occurrenceIndex) => {
      if (!Array.isArray(pos) || pos.length != 2) {
        return
      }

      const [start, end] = pos
      if (typeof start !== 'number' || typeof end !== 'number') {
        return
      }

      positioned.push({
        start,
        end,
        url: emote.url,
        code: emote.code,
        provider: emote.provider,
        key: `${emote.code}-${emoteIndex}-${occurrenceIndex}-${start}`,
        zeroWidth: emote.zero_width === true,
      })
    })
  })

  if (positioned.length === 0) {
    return text
  }

  positioned.sort((a, b) => a.start - b.start)

  const parts: (React.ReactNode | EmoteStack)[] = []
  // The stack a following zero-width emote attaches to; anything else in between
  // (text, a text-only emote) ends it.
  let openStack: EmoteStack | null = null
  let cursor = 0

  positioned.forEach((emote, index) => {
    if (emote.start < cursor || emote.start >= text.length || emote.end >= text.length) {
      return
    }

    const gap = text.slice(cursor, emote.start)
    if (emote.zeroWidth && emote.url && openStack && gap.trim() === '') {
      openStack.overlays.push(emote)
      cursor = emote.end + 1
      return
    }

    if (gap) {
      parts.push(<span key={`text-${index}-${cursor}`}>{gap}</span>)
    }

    if (!emote.url) {
      parts.push(
        <span key={`${emote.key}-text`} className="mx-[0.2em]">
          {text.slice(emote.start, emote.end + 1)}
        </span>
      )
      openStack = null
    } else {
      openStack = { base: { ...emote, url: emote.url }, overlays: [] }
      parts.push(openStack)
    }

    cursor = emote.end + 1
  })

  if (cursor < text.length) {
    parts.push(<span key={`text-tail-${cursor}`}>{text.slice(cursor)}</span>)
  }

  return parts.map((part) => (isEmoteStack(part) ? renderStack(part) : part))
}

function isEmoteStack(part: React.ReactNode | EmoteStack): part is EmoteStack {
  return typeof part === 'object' && part !== null && 'base' in part && 'overlays' in part
}
