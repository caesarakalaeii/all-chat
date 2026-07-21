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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { FeaturedAmbassadors } from '../FeaturedAmbassadors'

const HEADING = /Streamers who run on All-Chat/i

describe('FeaturedAmbassadors', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    cleanup()
  })

  it('renders a card per opted-in ambassador', async () => {
    ;(fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => [
        {
          username: 'alice',
          display_name: 'Alice',
          avatar_url: '',
          platform: 'twitch',
          tagline: 'Multistreams everywhere',
        },
        { username: 'bob', display_name: 'Bob', avatar_url: '', platform: 'kick', tagline: null },
      ],
    })

    render(<FeaturedAmbassadors />)

    expect(await screen.findByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
    expect(screen.getByText('Multistreams everywhere')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: HEADING })).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/v1/ambassadors')
  })

  it('renders nothing when there are no opted-in ambassadors', async () => {
    ;(fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => [],
    })

    render(<FeaturedAmbassadors />)

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1))
    expect(screen.queryByText(HEADING)).not.toBeInTheDocument()
  })

  it('fails silently and renders nothing when the request errors', async () => {
    ;(fetch as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('network'))

    render(<FeaturedAmbassadors />)

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1))
    expect(screen.queryByText(HEADING)).not.toBeInTheDocument()
  })
})
