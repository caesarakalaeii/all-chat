// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { FilterGroup } from '../FilterGroup'
import type { FilterSettings } from '@/lib/types/overlay'

afterEach(() => { cleanup() })

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
    const toggle = screen.getByRole('switch')
    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith({ hide_commands: true })
  })

  it('toggling hide_commands OFF calls onChange with { hide_commands: false }', () => {
    const onChange = vi.fn()
    render(<FilterGroup filterSettings={{ hide_commands: true }} onChange={onChange} />)
    const toggle = screen.getByRole('switch')
    fireEvent.click(toggle)
    expect(onChange).toHaveBeenCalledWith({ hide_commands: false })
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
