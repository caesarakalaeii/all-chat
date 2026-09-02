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
import { DiscoveryPausedNotice, hasPausedDiscovery } from '@/components/DiscoveryPausedNotice'
import { getTranslations } from '@/lib/i18n'
import type { ChatSource } from '@/lib/types/overlay'

const t = getTranslations()

function source(overrides: Partial<ChatSource>): ChatSource {
  return {
    id: Math.random().toString(36),
    overlay_id: 'overlay-1',
    platform: 'youtube',
    channel_id: 'UCparked',
    created_at: '',
    updated_at: '',
    is_active: true,
    ...overrides,
  }
}

afterEach(() => cleanup())

describe('hasPausedDiscovery', () => {
  it('is true when a source reports paused discovery', () => {
    expect(hasPausedDiscovery([source({ discovery_status: 'paused' })])).toBe(true)
  })

  it('is false when discovery_status is absent', () => {
    // Absence means the state is unknown, not healthy: the snapshot may have
    // expired or never been written. Unknown must not raise an alarm either.
    expect(hasPausedDiscovery([source({})])).toBe(false)
  })

  it('is false for an empty or missing source list', () => {
    expect(hasPausedDiscovery([])).toBe(false)
    expect(hasPausedDiscovery(undefined)).toBe(false)
  })

  it('is true when only one source among several is paused', () => {
    expect(
      hasPausedDiscovery([
        source({ platform: 'twitch', channel_id: 'caesarlp' }),
        source({ discovery_status: 'paused' }),
      ])
    ).toBe(true)
  })
})

describe('DiscoveryPausedNotice', () => {
  it('names the problem and links to the monitor for a paused source', () => {
    render(
      <DiscoveryPausedNotice
        overlayId="overlay-1"
        sources={[source({ discovery_status: 'paused' })]}
      />
    )

    expect(screen.getByRole('status')).toHaveTextContent(t('dashboard.discoveryPaused.title'))
    expect(screen.getByRole('status')).toHaveTextContent(t('dashboard.discoveryPaused.body'))

    const link = screen.getByRole('link', { name: t('dashboard.discoveryPaused.action') })
    expect(link).toHaveAttribute('href', '/overlay/overlay-1/view')
  })

  it('renders nothing when no source reports paused discovery', () => {
    const { container } = render(
      <DiscoveryPausedNotice overlayId="overlay-1" sources={[source({})]} />
    )

    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing while the source list has not loaded', () => {
    const { container } = render(
      <DiscoveryPausedNotice overlayId="overlay-1" sources={undefined} />
    )

    expect(container).toBeEmptyDOMElement()
  })
})
