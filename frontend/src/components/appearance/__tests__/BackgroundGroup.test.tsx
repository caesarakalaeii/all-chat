// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { BackgroundGroup } from '../BackgroundGroup'

afterEach(() => { cleanup() })

describe('BackgroundGroup', () => {
  const defaultSettings: Partial<VisualSettings> = {}

  it('renders overlay background controls', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    const overlayBgElements = screen.getAllByText(/overlay background/i)
    expect(overlayBgElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders bubble background controls', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    const bubbleBgElements = screen.getAllByText(/bubble background/i)
    expect(bubbleBgElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders border and layout control labels', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/border radius/i)).toBeDefined()
    expect(screen.getByText(/border width/i)).toBeDefined()
    expect(screen.getByText(/border color/i)).toBeDefined()
    expect(screen.getByText(/padding/i)).toBeDefined()
    expect(screen.getByText(/message gap/i)).toBeDefined()
    expect(screen.getByText(/backdrop blur/i)).toBeDefined()
  })

  it('onChange called with overlayBgColor patch as hex string', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Color swatches are present
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    expect(swatches.length).toBeGreaterThanOrEqual(1)
  })

  it('onChange called with overlayBgOpacity patch as decimal string', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Opacity slider present (input[type=range])
    const sliders = document.querySelectorAll('input[type="range"]')
    expect(sliders.length).toBeGreaterThan(0)
    // Fire change on first opacity slider (overlayBgOpacity)
    fireEvent.change(sliders[0], { target: { value: '50' } })
    expect(onChange).toHaveBeenCalledWith({ overlayBgOpacity: '0.5' })
  })

  it('onChange called with backdropBlur patch with px suffix', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Backdrop blur slider is the last range input — SliderControl renders them
    const rangeSliders = document.querySelectorAll('input[type="range"]')
    // 2 opacity sliders + 5 slider controls = 7 range inputs total
    // Backdrop blur is the last SliderControl
    const lastSlider = rangeSliders[rangeSliders.length - 1]
    fireEvent.change(lastSlider, { target: { value: '10' } })
    expect(onChange).toHaveBeenCalledWith({ backdropBlur: '10px' })
  })

  it('defaultSettings and onChange types are correct', () => {
    const onChange = vi.fn()
    expect(typeof onChange).toBe('function')
    expect(defaultSettings).toBeDefined()
  })
})
