// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

afterEach(() => { cleanup() })

// Mock localStorage before any imports that might reference it
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value }),
    removeItem: vi.fn((key: string) => { delete store[key] }),
    clear: vi.fn(() => { store = {} }),
  }
})()

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
  writable: true,
})

import { CollapsibleSection } from '../CollapsibleSection'

describe('CollapsibleSection', () => {
  beforeEach(() => {
    localStorageMock.clear()
    vi.clearAllMocks()
  })

  it('renders with title and children', () => {
    render(
      <CollapsibleSection id="typography" title="Typography">
        <span>child content</span>
      </CollapsibleSection>
    )
    expect(screen.getByText('Typography')).toBeDefined()
    expect(screen.getByText('child content')).toBeDefined()
  })

  it.todo('clicking trigger toggles open state')

  it('open state is written to localStorage key appearance-panel-sections-v1 when trigger is clicked', () => {
    render(
      <CollapsibleSection id="typography" title="Typography">
        <span>child content</span>
      </CollapsibleSection>
    )
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    // Verify localStorage.setItem was called with the correct key
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      'appearance-panel-sections-v1',
      expect.any(String)
    )
    // Verify the value stored is valid JSON with the section id
    const calls = localStorageMock.setItem.mock.calls
    const lastCall = calls[calls.length - 1] as [string, string]
    const stored = JSON.parse(lastCall[1]) as Record<string, boolean>
    expect('typography' in stored).toBe(true)
  })
})
