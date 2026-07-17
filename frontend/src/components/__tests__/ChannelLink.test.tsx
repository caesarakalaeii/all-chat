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

import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import { ChannelLink } from '../ChannelLink'

describe('ChannelLink', () => {
  afterEach(cleanup)

  it('links a Twitch source to its channel page and opens safely in a new tab', () => {
    const { getByRole } = render(
      <ChannelLink platform="twitch" channelId="xqc" channelName="xQc" />
    )
    const link = getByRole('link') as HTMLAnchorElement
    expect(link.getAttribute('href')).toBe('https://twitch.tv/xqc')
    expect(link.getAttribute('target')).toBe('_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
    expect(link.textContent).toContain('xQc')
  })

  it('renders plain text (no link) for a non-addressable platform', () => {
    const { queryByRole, getByText } = render(
      <ChannelLink platform="shared_overlay" channelId="overlay-uuid" channelName="Shared" />
    )
    expect(queryByRole('link')).toBeNull()
    expect(getByText('Shared')).toBeTruthy()
  })
})
