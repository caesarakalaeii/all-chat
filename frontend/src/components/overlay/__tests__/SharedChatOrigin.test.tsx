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

import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { SharedChatOrigin } from '@/components/overlay/SharedChatOrigin'

afterEach(() => cleanup())

describe('SharedChatOrigin avatar sizing', () => {
  // Asserting on the inline style declaration, not getComputedStyle: jsdom resolves a
  // relative length such as 1em to a pixel value, which would hide whether the prop
  // reached the element at all.
  it('sizes the avatar at 14px when size is omitted', () => {
    render(<SharedChatOrigin avatarUrl="https://cdn.example/origin.png" displayName="Origin" />)
    const avatar = screen.getByRole('img', { name: 'Origin' })
    expect(avatar.style.width).toBe('14px')
    expect(avatar.style.height).toBe('14px')
  })

  it('sizes the avatar at the given size', () => {
    render(
      <SharedChatOrigin
        avatarUrl="https://cdn.example/origin.png"
        displayName="Origin"
        size="1em"
      />
    )
    const avatar = screen.getByRole('img', { name: 'Origin' })
    expect(avatar.style.width).toBe('1em')
    expect(avatar.style.height).toBe('1em')
  })

  it('renders the text pill and no sized box without an avatar url', () => {
    render(<SharedChatOrigin displayName="Origin" size="1em" />)
    expect(screen.getByText('shared')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })
})
