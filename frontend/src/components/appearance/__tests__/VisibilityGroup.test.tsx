// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { VisibilityGroup } from '../VisibilityGroup'

afterEach(() => { cleanup() })

describe('VisibilityGroup', () => {
  it('renders 6 labels', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Show avatars')).toBeDefined()
    expect(screen.getByText('Show badges')).toBeDefined()
    expect(screen.getByText('Show timestamps')).toBeDefined()
    expect(screen.getByText('Show platform badge')).toBeDefined()
    expect(screen.getByText('Show emotes')).toBeDefined()
    expect(screen.getByText('Show username')).toBeDefined()
  })

  it('each row has a button with role="switch"', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches).toHaveLength(6)
  })

  it('clicking an ON switch calls onChange with none (showAvatars ON→OFF)', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'inline' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showAvatars: 'none' })
  })

  it('clicking an OFF switch calls onChange with inline (showAvatars OFF→ON)', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'none' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showAvatars: 'inline' })
  })

  it('showTimestamps emits "block" (not "inline") when switching to ON', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showTimestamps: 'none' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    // showTimestamps is the 3rd row (index 2)
    fireEvent.click(switches[2])
    expect(onChange).toHaveBeenCalledWith({ showTimestamps: 'block' })
  })

  it('visualSettings.showAvatars="none" overrides visibilityDefaults value of "inline"', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'none' }}
        onChange={onChange}
        visibilityDefaults={{ showAvatars: 'inline' }}
      />
    )
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('false')
  })

  it('when visualSettings.showAvatars is undefined and visibilityDefaults.showAvatars is "none", toggle renders unchecked', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{}}
        onChange={onChange}
        visibilityDefaults={{ showAvatars: 'none' }}
      />
    )
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('false')
  })

  it('when both visualSettings and visibilityDefaults are undefined for a field, toggle defaults to checked', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('true')
  })
})
