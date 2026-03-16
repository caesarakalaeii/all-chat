// @vitest-environment jsdom

/**
 * UserAvatar component tests (TDD)
 *
 * Tests:
 * - Renders frame img when frameUrl provided
 * - No frame img when frameUrl empty/undefined
 * - Renders flair img when flairUrl provided
 * - No flair img when flairUrl empty/undefined
 * - Renders avatar img when avatarUrl provided
 * - Renders fallback initials when avatarUrl absent
 */

import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { UserAvatar } from '../UserAvatar'

describe('UserAvatar', () => {
  it('renders frame img when frameUrl is provided', () => {
    const { container } = render(
      <UserAvatar size={40} frameUrl="https://example.com/frame.png" />
    )
    const imgs = container.querySelectorAll('img')
    const frameImg = Array.from(imgs).find((img) => img.src.includes('frame.png'))
    expect(frameImg).toBeDefined()
  })

  it('does not render frame img when frameUrl is empty', () => {
    const { container } = render(
      <UserAvatar size={40} frameUrl="" />
    )
    const imgs = container.querySelectorAll('img')
    const frameImg = Array.from(imgs).find((img) => img.src.includes('frame'))
    expect(frameImg).toBeUndefined()
  })

  it('does not render frame img when frameUrl is undefined', () => {
    const { container } = render(
      <UserAvatar size={40} />
    )
    // Only avatar img should be absent (no avatarUrl), no frame imgs
    const imgs = container.querySelectorAll('img')
    expect(imgs.length).toBe(0)
  })

  it('renders flair img when flairUrl is provided', () => {
    const { container } = render(
      <UserAvatar size={40} flairUrl="https://example.com/flair.png" />
    )
    const imgs = container.querySelectorAll('img')
    const flairImg = Array.from(imgs).find((img) => img.src.includes('flair.png'))
    expect(flairImg).toBeDefined()
  })

  it('does not render flair img when flairUrl is undefined', () => {
    const { container } = render(
      <UserAvatar size={40} />
    )
    const imgs = container.querySelectorAll('img')
    expect(imgs.length).toBe(0)
  })

  it('renders avatar img when avatarUrl is provided', () => {
    const { container } = render(
      <UserAvatar size={40} avatarUrl="https://example.com/avatar.png" />
    )
    const imgs = container.querySelectorAll('img')
    const avatarImg = Array.from(imgs).find((img) => img.src.includes('avatar.png'))
    expect(avatarImg).toBeDefined()
  })

  it('renders fallback initials when avatarUrl is absent', () => {
    const { getByText } = render(
      <UserAvatar size={40} displayName="Alice" />
    )
    expect(getByText('A')).toBeDefined()
  })

  it('renders ? fallback when avatarUrl and displayName both absent', () => {
    const { getAllByText } = render(
      <UserAvatar size={40} />
    )
    const matches = getAllByText('?')
    expect(matches.length).toBeGreaterThan(0)
  })

  it('frame img has 1.4x size', () => {
    const { container } = render(
      <UserAvatar size={40} frameUrl="https://example.com/frame.png" />
    )
    const imgs = container.querySelectorAll('img')
    const frameImg = Array.from(imgs).find((img) => img.src.includes('frame.png'))
    expect(frameImg).toBeDefined()
    expect(frameImg!.style.width).toBe('56px')
    expect(frameImg!.style.height).toBe('56px')
  })

  it('flair img has 0.4x size at bottom-right', () => {
    const { container } = render(
      <UserAvatar size={40} flairUrl="https://example.com/flair.png" />
    )
    const imgs = container.querySelectorAll('img')
    const flairImg = Array.from(imgs).find((img) => img.src.includes('flair.png'))
    expect(flairImg).toBeDefined()
    expect(flairImg!.style.width).toBe('16px')
    expect(flairImg!.style.height).toBe('16px')
  })
})
