// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { EventsGroup } from '../EventsGroup'

afterEach(() => { cleanup() })

describe('EventsGroup', () => {
  it('renders 5 event type labels', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Super Chat')).toBeDefined()
    expect(screen.getByText('Subscriptions')).toBeDefined()
    expect(screen.getByText('Raids')).toBeDefined()
    expect(screen.getByText('Bits')).toBeDefined()
    expect(screen.getByText('Membership Gift')).toBeDefined()
  })

  it('renders 5 toggles (one per event type)', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches).toHaveLength(5)
  })

  it('renders 5 size modifier sliders (one per event type)', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const sliders = screen.getAllByRole('slider')
    expect(sliders).toHaveLength(5)
  })

  it('when showSuperChat is undefined, toggle renders checked (default-visible)', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('true')
  })

  it('when showSuperChat is "none", toggle renders unchecked', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{ showSuperChat: 'none' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('false')
  })

  it('toggling Super Chat ON (was none) emits { showSuperChat: "block" }', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{ showSuperChat: 'none' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showSuperChat: 'block' })
  })

  it('toggling Super Chat OFF (was undefined/block) emits { showSuperChat: "none" }', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showSuperChat: 'none' })
  })

  it('toggling Membership Gift OFF emits { showMembershipGift: "none" }', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    // Membership Gift is the 5th row (index 4)
    fireEvent.click(switches[4])
    expect(onChange).toHaveBeenCalledWith({ showMembershipGift: 'none' })
  })

  it('size modifier slider for Super Chat fires onChange with unitless string', () => {
    const onChange = vi.fn()
    render(<EventsGroup visualSettings={{}} onChange={onChange} />)
    const sliders = screen.getAllByRole('slider')
    fireEvent.change(sliders[0], { target: { value: '1.5' } })
    expect(onChange).toHaveBeenCalledWith({ superChatSizeModifier: '1.5' })
  })
})
