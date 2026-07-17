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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'

afterEach(() => {
  cleanup()
})

// Mock localStorage before any imports that might reference it
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key]
    }),
    clear: vi.fn(() => {
      store = {}
    }),
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

  // New tests for storageKey prop
  it('when storageKey is provided, localStorage.setItem is called with that key', () => {
    render(
      <CollapsibleSection id="themes" title="Themes" storageKey="editor-panel-sections-v1">
        <span>child content</span>
      </CollapsibleSection>
    )
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      'editor-panel-sections-v1',
      expect.any(String)
    )
    // Must NOT use the default key
    expect(localStorageMock.setItem).not.toHaveBeenCalledWith(
      'appearance-panel-sections-v1',
      expect.any(String)
    )
  })

  it('when no storageKey is passed, uses appearance-panel-sections-v1 (backward compat)', () => {
    render(
      <CollapsibleSection id="colors" title="Colors">
        <span>child content</span>
      </CollapsibleSection>
    )
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      'appearance-panel-sections-v1',
      expect.any(String)
    )
  })

  it('when defaultOpen=true and no stored value, section starts open', () => {
    render(
      <CollapsibleSection id="themes" title="Themes" defaultOpen={true}>
        <span>themes content</span>
      </CollapsibleSection>
    )
    // The trigger should have aria-expanded="true" or the panel should be open
    // We can verify by checking that clicking again closes (setItem called with false)
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    // Section was open, click closes it — stored value should be false
    const calls = localStorageMock.setItem.mock.calls
    const lastCall = calls[calls.length - 1] as [string, string]
    const stored = JSON.parse(lastCall[1]) as Record<string, boolean>
    expect(stored['themes']).toBe(false)
  })

  it('when defaultOpen is not passed and no stored value, section starts closed', () => {
    render(
      <CollapsibleSection id="sources" title="Sources">
        <span>sources content</span>
      </CollapsibleSection>
    )
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    // Section was closed, click opens it — stored value should be true
    const calls = localStorageMock.setItem.mock.calls
    const lastCall = calls[calls.length - 1] as [string, string]
    const stored = JSON.parse(lastCall[1]) as Record<string, boolean>
    expect(stored['sources']).toBe(true)
  })

  it('stored value overrides defaultOpen (stored false wins over defaultOpen=true)', () => {
    // Pre-store false for this section
    localStorageMock.setItem('appearance-panel-sections-v1', JSON.stringify({ themes: false }))
    vi.clearAllMocks()
    // Re-mock getItem to return the pre-stored value
    localStorageMock.getItem.mockImplementation((key: string): string => {
      if (key === 'appearance-panel-sections-v1') {
        return JSON.stringify({ themes: false })
      }
      return ''
    })

    render(
      <CollapsibleSection id="themes" title="Themes" defaultOpen={true}>
        <span>themes content</span>
      </CollapsibleSection>
    )
    const trigger = screen.getByRole('button')
    fireEvent.click(trigger)
    // If stored false overrode defaultOpen=true, section was closed, click opens → stored true
    const calls = localStorageMock.setItem.mock.calls
    const lastCall = calls[calls.length - 1] as [string, string]
    const stored = JSON.parse(lastCall[1]) as Record<string, boolean>
    expect(stored['themes']).toBe(true)
  })
})

describe('CollapsibleSection forceOpen (onboarding spotlight)', () => {
  beforeEach(() => {
    localStorageMock.clear()
    vi.clearAllMocks()
  })

  it('forceOpen=true opens regardless of stored preference', () => {
    localStorageMock.setItem('appearance-panel-sections-v1', JSON.stringify({ sources: false }))
    localStorageMock.setItem.mockClear()
    render(
      <CollapsibleSection id="sources" title="Sources" forceOpen>
        <span>child content</span>
      </CollapsibleSection>
    )
    expect(screen.getByRole('button').getAttribute('aria-expanded')).toBe('true')
  })

  it('forceOpen=false closes even when the stored preference is open', () => {
    localStorageMock.setItem('appearance-panel-sections-v1', JSON.stringify({ sources: true }))
    localStorageMock.setItem.mockClear()
    render(
      <CollapsibleSection id="sources" title="Sources" forceOpen={false}>
        <span>child content</span>
      </CollapsibleSection>
    )
    expect(screen.getByRole('button').getAttribute('aria-expanded')).toBe('false')
  })

  it('toggling while forced never writes the forced state to localStorage', () => {
    render(
      <CollapsibleSection id="sources" title="Sources" forceOpen>
        <span>child content</span>
      </CollapsibleSection>
    )
    fireEvent.click(screen.getByRole('button'))
    expect(localStorageMock.setItem).not.toHaveBeenCalled()
    expect(screen.getByRole('button').getAttribute('aria-expanded')).toBe('true')
  })
})
