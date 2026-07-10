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
      })
    })
  })

  if (positioned.length === 0) {
    return text
  }

  positioned.sort((a, b) => a.start - b.start)

  const nodes: React.ReactNode[] = []
  let cursor = 0

  positioned.forEach((emote, index) => {
    if (emote.start < cursor || emote.start >= text.length || emote.end >= text.length) {
      return
    }

    if (emote.start > cursor) {
      nodes.push(<span key={`text-${index}-${cursor}`}>{text.slice(cursor, emote.start)}</span>)
    }

    if (!emote.url) {
      nodes.push(
        <span key={`${emote.key}-text`} className="mx-[0.2em]">
          {text.slice(emote.start, emote.end + 1)}
        </span>
      )
    } else {
      nodes.push(
        <Image
          key={emote.key}
          src={emote.url}
          alt=""
          title={`${emote.code} (${emote.provider})`}
          width={28}
          height={28}
          className="mx-[0.2em] inline-block h-[1.4em] w-auto align-text-bottom"
        />
      )
    }

    cursor = emote.end + 1
  })

  if (cursor < text.length) {
    nodes.push(<span key={`text-tail-${cursor}`}>{text.slice(cursor)}</span>)
  }

  return nodes
}
