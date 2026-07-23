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
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { SettingsSearch } from '../SettingsSearch'

afterEach(() => {
  cleanup()
})

function typeQuery(value: string) {
  const input = screen.getByRole('combobox', { name: 'Search settings' })
  fireEvent.change(input, { target: { value } })
  return input
}

describe('SettingsSearch', () => {
  it('shows matching results while typing', () => {
    render(<SettingsSearch onNavigate={() => {}} />)
    typeQuery('badge')
    expect(screen.getByRole('listbox')).toBeDefined()
    expect(screen.getByText('Show badges')).toBeDefined()
    expect(screen.getByText('Show platform badge')).toBeDefined()
  })

  it('shows a breadcrumb (group › section) on each result', () => {
    render(<SettingsSearch onNavigate={() => {}} />)
    typeQuery('badge')
    expect(screen.getAllByText(/Appearance › Visibility/).length).toBeGreaterThan(0)
  })

  it('clicking a result calls onNavigate with section and anchor, then clears', () => {
    const onNavigate = vi.fn()
    render(<SettingsSearch onNavigate={onNavigate} />)
    const input = typeQuery('badge')
    fireEvent.click(screen.getByText('Show badges'))
    expect(onNavigate).toHaveBeenCalledWith('visibility', 'showBadges')
    expect((input as HTMLInputElement).value).toBe('')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('Enter selects the highlighted result (first by default)', () => {
    const onNavigate = vi.fn()
    render(<SettingsSearch onNavigate={onNavigate} />)
    const input = typeQuery('badge')
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onNavigate).toHaveBeenCalledTimes(1)
    expect(onNavigate.mock.calls[0][0]).toBe('visibility')
  })

  it('ArrowDown moves the highlight before Enter selects', () => {
    const onNavigate = vi.fn()
    render(<SettingsSearch onNavigate={onNavigate} />)
    const input = typeQuery('badge')
    const options = screen.getAllByRole('option')
    expect(options.length).toBeGreaterThan(1)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onNavigate).toHaveBeenCalledTimes(1)
    // Second result differs from the first
    expect(onNavigate.mock.calls[0][1]).not.toBe('showBadges')
  })

  it('Escape clears the query and closes the results', () => {
    render(<SettingsSearch onNavigate={() => {}} />)
    const input = typeQuery('badge')
    fireEvent.keyDown(input, { key: 'Escape' })
    expect((input as HTMLInputElement).value).toBe('')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('shows an empty state for a query with no matches', () => {
    render(<SettingsSearch onNavigate={() => {}} />)
    typeQuery('xyzzy-no-such-setting')
    expect(screen.getByText(/No settings match/)).toBeDefined()
  })
})
