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
import type { VisualSettings } from '@/lib/types/visual-settings'
import { SizingGroup } from '../SizingGroup'

afterEach(() => { cleanup() })

describe('SizingGroup', () => {
  it('renders label "Avatar size"', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Avatar size')).toBeDefined()
  })

  it('renders label "Badge size"', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Badge size')).toBeDefined()
  })

  it('renders label "Emote scale"', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Emote scale')).toBeDefined()
  })

  it('Avatar size input has min=16, max=64, step=2', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const avatarSlider = sliders[0] as HTMLInputElement
    expect(avatarSlider.min).toBe('16')
    expect(avatarSlider.max).toBe('64')
    expect(avatarSlider.step).toBe('2')
  })

  it('Badge size input has min=12, max=32, step=2', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const badgeSlider = sliders[1] as HTMLInputElement
    expect(badgeSlider.min).toBe('12')
    expect(badgeSlider.max).toBe('32')
    expect(badgeSlider.step).toBe('2')
  })

  it('Emote scale input has min=0.5, max=3, step=0.1', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const emoteSlider = sliders[2] as HTMLInputElement
    expect(emoteSlider.min).toBe('0.5')
    expect(emoteSlider.max).toBe('3')
    expect(emoteSlider.step).toBe('0.1')
  })

  it('Avatar size slider value defaults to 32 when avatarSize is undefined', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const avatarSlider = sliders[0] as HTMLInputElement
    expect(avatarSlider.value).toBe('32')
  })

  it('Badge size slider value defaults to 18 when badgeSize is undefined', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const badgeSlider = sliders[1] as HTMLInputElement
    expect(badgeSlider.value).toBe('18')
  })

  it('Emote scale slider value defaults to 1 when emoteScale is undefined', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    const emoteSlider = sliders[2] as HTMLInputElement
    expect(emoteSlider.value).toBe('1')
  })

  it('onChange fires with { avatarSize: "40px" } format (px suffix, not bare number) when avatar slider fires', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    fireEvent.change(sliders[0], { target: { value: '40' } })
    expect(onChange).toHaveBeenCalledWith({ avatarSize: '40px' })
  })

  it('onChange fires with { emoteScale: "1.5" } (unitless — no px) when emote scale slider fires', () => {
    const onChange = vi.fn()
    render(<SizingGroup visualSettings={{}} onChange={onChange} />)
    const sliders = document.querySelectorAll('input[type="range"]')
    fireEvent.change(sliders[2], { target: { value: '1.5' } })
    expect(onChange).toHaveBeenCalledWith({ emoteScale: '1.5' })
  })
})
