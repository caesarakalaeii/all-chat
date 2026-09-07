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
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { DockNoticeBar } from '@/components/overlay/DockNoticeBar'
import { DockOverflowMenu } from '@/components/overlay/DockOverflowMenu'

afterEach(() => cleanup())

describe('DockOverflowMenu', () => {
  it('keeps its controls out of the header row until it is opened', () => {
    render(
      <DockOverflowMenu>
        <button type="button">Details</button>
      </DockOverflowMenu>
    )
    const trigger = screen.getByRole('button', { name: 'Monitor controls' })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('button', { name: 'Details' })).not.toBeInTheDocument()

    fireEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('button', { name: 'Details' })).toBeInTheDocument()
  })

  it('closes again on a second click of the trigger', () => {
    render(
      <DockOverflowMenu>
        <button type="button">Details</button>
      </DockOverflowMenu>
    )
    const trigger = screen.getByRole('button', { name: 'Monitor controls' })
    fireEvent.click(trigger)
    fireEvent.click(trigger)
    expect(screen.queryByRole('button', { name: 'Details' })).not.toBeInTheDocument()
  })
})

describe('DockNoticeBar', () => {
  it('renders nothing at all when there is no notice to show', () => {
    const { container } = render(<DockNoticeBar count={0}>{null}</DockNoticeBar>)
    expect(container).toBeEmptyDOMElement()
  })

  // A single notice is short enough to read in place; summarising it as "1
  // notice" would cost a click to learn something already known.
  it('shows a single notice expanded, with no summary to click', () => {
    render(
      <DockNoticeBar count={1}>
        <div>Still reconnecting</div>
      </DockNoticeBar>
    )
    expect(screen.getByText('Still reconnecting')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  // Eight stacked full-width strips would leave a dock panel with no room for
  // chat, so several notices collapse behind one row — but none is dropped.
  it('collapses several notices behind a summary that counts them', () => {
    render(
      <DockNoticeBar count={3}>
        <div>Still reconnecting</div>
        <div>Replay truncated</div>
        <div>Moderation is disabled</div>
      </DockNoticeBar>
    )
    expect(screen.queryByText('Still reconnecting')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '3 notices' }))
    expect(screen.getByText('Still reconnecting')).toBeInTheDocument()
    expect(screen.getByText('Replay truncated')).toBeInTheDocument()
    expect(screen.getByText('Moderation is disabled')).toBeInTheDocument()
  })
})
