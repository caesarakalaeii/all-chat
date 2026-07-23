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
import { EditorNav } from '../EditorNav'
import { EDITOR_SECTIONS, EDITOR_GROUPS } from '../sectionRegistry'

afterEach(() => {
  cleanup()
})

describe('EditorNav', () => {
  it('renders every group label and one button per section', () => {
    render(<EditorNav activeId="theme" onSelect={() => {}} />)
    for (const group of EDITOR_GROUPS) {
      expect(screen.getAllByText(group.label).length).toBeGreaterThan(0)
    }
    for (const section of EDITOR_SECTIONS) {
      expect(
        screen.getByRole('button', { name: section.navLabel ?? section.title })
      ).toBeDefined()
    }
  })

  it('marks the active section with aria-current', () => {
    render(<EditorNav activeId="visibility" onSelect={() => {}} />)
    const active = screen.getByRole('button', { name: 'Visibility' })
    expect(active.getAttribute('aria-current')).toBe('true')
    const inactive = screen.getByRole('button', { name: 'Theme' })
    expect(inactive.getAttribute('aria-current')).toBeNull()
  })

  it('calls onSelect with the section id when a nav item is clicked', () => {
    const onSelect = vi.fn()
    render(<EditorNav activeId="theme" onSelect={onSelect} />)
    fireEvent.click(screen.getByRole('button', { name: 'Visibility' }))
    expect(onSelect).toHaveBeenCalledWith('visibility')
  })

  it('is a labeled navigation landmark', () => {
    render(<EditorNav activeId="theme" onSelect={() => {}} />)
    expect(screen.getByRole('navigation', { name: 'Overlay settings' })).toBeDefined()
  })
})
