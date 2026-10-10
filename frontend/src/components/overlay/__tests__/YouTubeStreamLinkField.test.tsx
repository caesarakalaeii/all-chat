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
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { YouTubeStreamLinkField } from '@/components/overlay/YouTubeStreamLinkField'
import { ApiError } from '@/lib/api/client'
import { overlaysApi } from '@/lib/api/overlays'
import { getTranslations } from '@/lib/i18n'
import type { ChatSource } from '@/lib/types/overlay'

vi.mock('@/lib/api/overlays', () => ({
  overlaysApi: { updateSourceConfig: vi.fn() },
}))

const t = getTranslations()
const updateSourceConfig = vi.mocked(overlaysApi.updateSourceConfig)

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function makeSource(config: Record<string, unknown>): ChatSource {
  return {
    id: 'src-1',
    overlay_id: 'ovl-1',
    platform: 'youtube',
    channel_id: 'UCabc',
    config,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    is_active: true,
  }
}

describe('YouTubeStreamLinkField', () => {
  it('saves the trimmed link merged into the existing source config', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(
      <YouTubeStreamLinkField
        source={makeSource({ stream_select: 'most_viewers' })}
        overlayId="ovl-1"
        onSaved={onSaved}
      />
    )

    fireEvent.change(screen.getByLabelText(t('overlayEditor.streamLink.label')), {
      target: { value: '  https://youtu.be/dQw4w9WgXcQ  ' },
    })
    fireEvent.click(screen.getByRole('button', { name: t('overlayEditor.streamLink.save') }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
      stream_select: 'most_viewers',
      stream_id: 'https://youtu.be/dQw4w9WgXcQ',
    })
  })

  it('clears the stored pin by sending an empty stream_id', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(
      <YouTubeStreamLinkField
        source={makeSource({ stream_select: 'all', stream_id: 'dQw4w9WgXcQ' })}
        overlayId="ovl-1"
        onSaved={onSaved}
      />
    )

    expect(
      screen.getByText(t('overlayEditor.streamLink.currentPin', { videoId: 'dQw4w9WgXcQ' }))
    ).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: t('overlayEditor.streamLink.clear') }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
      stream_select: 'all',
      stream_id: '',
    })
  })

  it('renders the backend message when the link is rejected with 422', async () => {
    const message = 'stream link belongs to a different channel'
    updateSourceConfig.mockRejectedValue(new ApiError(422, message, { error: message }))
    const onSaved = vi.fn()
    render(<YouTubeStreamLinkField source={makeSource({})} overlayId="ovl-1" onSaved={onSaved} />)

    const input = screen.getByLabelText(t('overlayEditor.streamLink.label'))
    fireEvent.change(input, { target: { value: 'https://youtu.be/xxxxxxxxxxx' } })
    fireEvent.click(screen.getByRole('button', { name: t('overlayEditor.streamLink.save') }))

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(onSaved).not.toHaveBeenCalled()
  })

  it('falls back to generic copy for errors that carry no user-facing message', async () => {
    updateSourceConfig.mockRejectedValue(new ApiError(503, 'Service Unavailable'))
    render(<YouTubeStreamLinkField source={makeSource({})} overlayId="ovl-1" onSaved={vi.fn()} />)

    fireEvent.change(screen.getByLabelText(t('overlayEditor.streamLink.label')), {
      target: { value: 'dQw4w9WgXcQ' },
    })
    fireEvent.click(screen.getByRole('button', { name: t('overlayEditor.streamLink.save') }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      t('overlayEditor.streamLink.saveFailed')
    )
  })
})
