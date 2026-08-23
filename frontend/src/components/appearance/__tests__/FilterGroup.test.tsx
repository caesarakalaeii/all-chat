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
import { FilterGroup, SAY_HI_TOGGLE_LABEL } from '../FilterGroup'
import type { FilterSettings } from '@/lib/types/overlay'

afterEach(() => {
  cleanup()
})

const SAY_HI_PHRASE_PLACEHOLDER = 'Type phrase, press Enter'

describe('FilterGroup', () => {
  it('renders "Blocked usernames" label and an input with placeholder "Type username, press Enter"', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Blocked usernames')).toBeDefined()
    expect(screen.getByPlaceholderText('Type username, press Enter')).toBeDefined()
  })

  it('renders "Blocked keywords" label and an input with placeholder "Type keyword or regex, press Enter"', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Blocked keywords')).toBeDefined()
    expect(screen.getByPlaceholderText('Type keyword or regex, press Enter')).toBeDefined()
  })

  it('renders "Hide bot commands (!)" toggle switch', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Hide bot commands (!)')).toBeDefined()
  })

  it('renders "Min message length" slider', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Min message length')).toBeDefined()
  })

  it('renders "Add common bots" button', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Add common bots')).toBeDefined()
  })

  // The tag inputs keep a fixed 120px floor so a long tag list cannot squeeze the
  // caret out of sight. It must stay an arbitrary px value: the rem-relative
  // min-w-30 the Tailwind plugin suggests would grow with the user's font size and
  // push the input onto its own row inside the fixed-width customiser panel.
  // Asserted on the rendered class attribute, because the risk being guarded is an
  // edit that changes the emitted class list without touching the source literal.
  it('floors the tag inputs at min-w-[120px] so the caret stays visible', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    for (const placeholder of [
      'Type username, press Enter',
      'Type keyword or regex, press Enter',
    ]) {
      const input = screen.getByPlaceholderText(placeholder)
      expect(input.className.split(/\s+/)).toContain('min-w-[120px]')
    }
  })

  it('typing "nightbot" + pressing Enter calls onChange with banned_users containing "nightbot"', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    const input = screen.getByPlaceholderText('Type username, press Enter')
    fireEvent.change(input, { target: { value: 'nightbot' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith({ banned_users: ['nightbot'] })
  })

  it('clicking X on an existing banned_users tag calls onChange without that username', () => {
    const onChange = vi.fn()
    const filterSettings: FilterSettings = { banned_users: ['nightbot', 'streamelements'] }
    render(<FilterGroup filterSettings={filterSettings} onChange={onChange} />)
    const removeBtn = screen.getByLabelText('Remove nightbot')
    fireEvent.click(removeBtn)
    expect(onChange).toHaveBeenCalledWith({ banned_users: ['streamelements'] })
  })

  it('typing "spam" + pressing Enter calls onChange with banned_words containing "spam"', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    const input = screen.getByPlaceholderText('Type keyword or regex, press Enter')
    fireEvent.change(input, { target: { value: 'spam' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith({ banned_words: ['spam'] })
  })

  it('clicking X on an existing banned_words tag calls onChange without that keyword', () => {
    const onChange = vi.fn()
    const filterSettings: FilterSettings = { banned_words: ['spam', 'offensive'] }
    render(<FilterGroup filterSettings={filterSettings} onChange={onChange} />)
    const removeBtn = screen.getByLabelText('Remove spam')
    fireEvent.click(removeBtn)
    expect(onChange).toHaveBeenCalledWith({ banned_words: ['offensive'] })
  })

  it('toggling hide_commands calls onChange with { hide_commands: true }', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_commands: false }} onChange={onChange} />)
    const toggle = screen.getByRole('switch', { name: 'Hide bot commands (!)' })
    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith({ hide_commands: true })
  })

  it('toggling hide_commands OFF calls onChange with { hide_commands: false }', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_commands: true }} onChange={onChange} />)
    const toggle = screen.getByRole('switch', { name: 'Hide bot commands (!)' })
    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith({ hide_commands: false })
  })

  it('renders the YouTube "said hi" toggle with its exact label', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    expect(SAY_HI_TOGGLE_LABEL).toBe('Hide YouTube "said hi" greetings')
    expect(screen.getByText(SAY_HI_TOGGLE_LABEL)).toBeDefined()
  })

  it('toggling the "said hi" switch calls onChange with { hide_youtube_say_hi: true }', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_youtube_say_hi: false }} onChange={onChange} />)
    fireEvent.click(screen.getByRole('switch', { name: SAY_HI_TOGGLE_LABEL }))
    expect(onChange).toHaveBeenCalledWith({ hide_youtube_say_hi: true })
  })

  it('hides the extra-phrases input while hide_youtube_say_hi is off', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_youtube_say_hi: false }} onChange={onChange} />)
    expect(screen.queryByPlaceholderText(SAY_HI_PHRASE_PLACEHOLDER)).toBeNull()
  })

  it('shows the extra-phrases input while hide_youtube_say_hi is on', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_youtube_say_hi: true }} onChange={onChange} />)
    expect(screen.getByPlaceholderText(SAY_HI_PHRASE_PLACEHOLDER)).toBeDefined()
  })

  it('typing a German greeting + Enter calls onChange with say_hi_extra_phrases', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_youtube_say_hi: true }} onChange={onChange} />)
    const input = screen.getByPlaceholderText(SAY_HI_PHRASE_PLACEHOLDER)
    fireEvent.change(input, { target: { value: 'hat hallo gesagt' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith({ say_hi_extra_phrases: ['hat hallo gesagt'] })
  })

  it('removing the only extra phrase calls onChange with an empty say_hi_extra_phrases', () => {
    const onChange = vi.fn()
    const filterSettings: FilterSettings = {
      hide_youtube_say_hi: true,
      say_hi_extra_phrases: ['hat hallo gesagt'],
    }
    render(<FilterGroup filterSettings={filterSettings} onChange={onChange} />)
    expect(screen.getByText('hat hallo gesagt')).toBeDefined()
    fireEvent.click(screen.getByLabelText('Remove hat hallo gesagt'))
    expect(onChange).toHaveBeenCalledWith({ say_hi_extra_phrases: [] })
  })

  it('changing min_message_length slider calls onChange with { min_message_length: value }', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ min_message_length: 0 }} onChange={onChange} />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '10' } })
    expect(onChange).toHaveBeenCalledWith({ min_message_length: 10 })
  })

  it('clicking "Add common bots" calls onChange with banned_users containing at least "nightbot", "streamelements", "moobot", "fossabot", "soundalerts"', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{}} onChange={onChange} />)
    fireEvent.click(screen.getByText('Add common bots'))
    expect(onChange).toHaveBeenCalled()
    const call = onChange.mock.calls[0][0] as { banned_users: string[] }
    expect(call.banned_users).toContain('nightbot')
    expect(call.banned_users).toContain('streamelements')
    expect(call.banned_users).toContain('moobot')
    expect(call.banned_users).toContain('fossabot')
    expect(call.banned_users).toContain('soundalerts')
  })

  it('"Add common bots" does not add duplicates if some bots already exist in banned_users', () => {
    const onChange = vi.fn()
    const filterSettings: FilterSettings = { banned_users: ['nightbot'] }
    render(<FilterGroup filterSettings={filterSettings} onChange={onChange} />)
    fireEvent.click(screen.getByText('Add common bots'))
    expect(onChange).toHaveBeenCalled()
    const call = onChange.mock.calls[0][0] as { banned_users: string[] }
    const nightbotCount = call.banned_users.filter((u) => u === 'nightbot').length
    expect(nightbotCount).toBe(1)
  })

  it('duplicate username entry is prevented (typing "nightbot" when "nightbot" already in list does not add it again)', () => {
    const onChange = vi.fn()
    const filterSettings: FilterSettings = { banned_users: ['nightbot'] }
    render(<FilterGroup filterSettings={filterSettings} onChange={onChange} />)
    const input = screen.getByPlaceholderText('Type username, press Enter')
    fireEvent.change(input, { target: { value: 'nightbot' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).not.toHaveBeenCalled()
  })
})
