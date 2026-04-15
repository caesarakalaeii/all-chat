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
import { render, screen, cleanup } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { TypographyGroup } from '../TypographyGroup'

afterEach(() => { cleanup() })

describe('TypographyGroup', () => {
  const defaultSettings: Partial<VisualSettings> = {}

  it('renders font family labels for body, username, and timestamp', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/body font/i)).toBeDefined()
    expect(screen.getByText(/username font/i)).toBeDefined()
    expect(screen.getByText(/timestamp font/i)).toBeDefined()
  })

  it('renders font weight label', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/font weight/i)).toBeDefined()
  })

  it('renders font size labels for body, username, and timestamp', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/body size/i)).toBeDefined()
    expect(screen.getByText(/username size/i)).toBeDefined()
    expect(screen.getByText(/timestamp size/i)).toBeDefined()
  })

  it('renders line height and letter spacing labels', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/line height/i)).toBeDefined()
    expect(screen.getByText(/letter spacing/i)).toBeDefined()
  })

  it.todo('onChange called with fontFamily patch when font selection changes')

  it.todo('onChange called with fontWeight patch on select change')

  it.todo('onChange called with lineHeight patch on slider change')
})
