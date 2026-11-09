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

// @vitest-environment jsdom

import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { renderMessageContent } from '@/lib/renderMessage'
import type { ChatMessage, Emote } from '@/lib/types/message'

// next/image validates remote hosts against next.config.js, which vitest never loads.
vi.mock('next/image', () => ({
  __esModule: true,
  default: ({ src, alt, ...rest }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} {...rest} />
  ),
}))

afterEach(cleanup)

function emote(code: string, text: string, zeroWidth = false): Emote {
  const positions: number[][] = []
  for (let i = text.indexOf(code); i !== -1; i = text.indexOf(code, i + 1)) {
    positions.push([i, i + code.length - 1])
  }
  return {
    code,
    provider: '7tv',
    url: `https://cdn.7tv.app/emote/${code}/1x.webp`,
    positions,
    ...(zeroWidth && { zero_width: true }),
  }
}

function renderText(text: string, emotes: Emote[]) {
  const message = { message: { text, emotes } } as unknown as ChatMessage
  return render(<p>{renderMessageContent(message)}</p>).container.querySelector('p')!
}

const code = (img: Element) => img.getAttribute('title')!.split(' ')[0]

/** Each top-level child as the emote codes it draws, or its text. */
function layout(p: HTMLElement): string[] {
  return Array.from(p.childNodes).map((node) => {
    const el = node as HTMLElement
    if (el.tagName === 'IMG') return code(el)
    const imgs = el.querySelectorAll?.('img') ?? []
    if (imgs.length > 0) {
      return Array.from(imgs).map(code).join('+')
    }
    return el.textContent ?? ''
  })
}

describe('renderMessageContent zero-width emotes', () => {
  it('stacks a zero-width emote on the emote before it', () => {
    const text = 'hi KEKW PUNCH ok'
    const p = renderText(text, [emote('KEKW', text), emote('PUNCH', text, true)])
    expect(layout(p)).toEqual(['hi ', 'KEKW+PUNCH', ' ok'])
  })

  it('stacks several zero-width emotes on one base', () => {
    const text = 'KEKW PUNCH Blush'
    const p = renderText(text, [
      emote('KEKW', text),
      emote('PUNCH', text, true),
      emote('Blush', text, true),
    ])
    expect(layout(p)).toEqual(['KEKW+PUNCH+Blush'])
  })

  it('draws a zero-width emote inline when text separates it from the last emote', () => {
    const text = 'KEKW wow PUNCH'
    const p = renderText(text, [emote('KEKW', text), emote('PUNCH', text, true)])
    expect(layout(p)).toEqual(['KEKW', ' wow ', 'PUNCH'])
  })

  it('draws a zero-width emote inline when nothing precedes it', () => {
    const text = 'PUNCH hi'
    const p = renderText(text, [emote('PUNCH', text, true)])
    expect(layout(p)).toEqual(['PUNCH', ' hi'])
  })

  it('keeps ordinary emotes side by side', () => {
    const text = 'KEKW KEKW'
    const p = renderText(text, [emote('KEKW', text)])
    expect(layout(p)).toEqual(['KEKW', ' ', 'KEKW'])
  })
})
