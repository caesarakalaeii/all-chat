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

import { YouTubeOfficialApiToggle } from '@/components/overlay/YouTubeOfficialApiToggle'
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

function toggle() {
  return screen.getByRole('switch', { name: t('overlayEditor.officialApi.label') })
}

describe('YouTubeOfficialApiToggle', () => {
  it('turning it on sends official_api true merged into the existing config', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({ stream_select: 'all', stream_id: 'dQw4w9WgXcQ' })}
        overlayId="ovl-1"
        isPremium
        onSaved={onSaved}
      />
    )

    expect(toggle()).not.toBeChecked()
    fireEvent.click(toggle())

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
      stream_select: 'all',
      stream_id: 'dQw4w9WgXcQ',
      official_api: true,
    })
    expect(screen.queryByRole('link', { name: /upgrade/i })).not.toBeInTheDocument()
  })

  it('is locked behind an upgrade link for non-premium users', () => {
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({})}
        overlayId="ovl-1"
        isPremium={false}
        onSaved={vi.fn()}
      />
    )

    expect(toggle()).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByRole('link', { name: /upgrade/i })).toHaveAttribute('href', '/upgrade')
    fireEvent.click(toggle())
    expect(updateSourceConfig).not.toHaveBeenCalled()
  })

  it('lets a lapsed user turn a stored opt-in off', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({ official_api: true, stream_select: 'most_viewers' })}
        overlayId="ovl-1"
        isPremium={false}
        onSaved={onSaved}
      />
    )

    expect(toggle()).toBeChecked()
    expect(toggle()).not.toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByText(t('overlayEditor.officialApi.lapsed'))).toBeInTheDocument()
    fireEvent.click(toggle())

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
      stream_select: 'most_viewers',
      official_api: false,
    })
  })

  it('renders the backend message when the channel is not connected (409)', async () => {
    // The server's 409 body carries only `error` (handlers/sources.go), so this exercises
    // the err.message fallback production actually takes.
    const message = 'connect this YouTube channel first'
    updateSourceConfig.mockRejectedValue(new ApiError(409, message, { error: message }))
    const onSaved = vi.fn()
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({})}
        overlayId="ovl-1"
        isPremium
        onSaved={onSaved}
      />
    )

    fireEvent.click(toggle())

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
    expect(toggle()).not.toBeChecked()
    expect(onSaved).not.toHaveBeenCalled()
  })

  it('renders the premium message when the server refuses with 403', async () => {
    const message = 'This is a premium feature. Upgrade your account to access this functionality.'
    updateSourceConfig.mockRejectedValue(
      new ApiError(403, 'Premium feature required', { error: 'Premium feature required', message })
    )
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({})}
        overlayId="ovl-1"
        isPremium
        onSaved={vi.fn()}
      />
    )

    fireEvent.click(toggle())

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  })

  it('falls back to generic copy for errors that carry no user-facing message', async () => {
    updateSourceConfig.mockRejectedValue(new ApiError(503, 'Service Unavailable'))
    render(
      <YouTubeOfficialApiToggle
        source={makeSource({})}
        overlayId="ovl-1"
        isPremium
        onSaved={vi.fn()}
      />
    )

    fireEvent.click(toggle())

    expect(await screen.findByRole('alert')).toHaveTextContent(
      t('overlayEditor.officialApi.saveFailed')
    )
  })
})
