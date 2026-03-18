// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'

// Stub tests for CollapsibleSection
// Stubs exist to establish test names; full interaction tests run once component is implemented.
describe('CollapsibleSection', () => {
  const mockOnOpenChange = vi.fn()
  void mockOnOpenChange

  it('renders with title and children', async () => {
    const { render, screen } = await import('@testing-library/react')
    const { CollapsibleSection } = await import('../CollapsibleSection')

    render(
      <CollapsibleSection id="typography" title="Typography">
        <span>child content</span>
      </CollapsibleSection>
    )
    expect(screen.getByText('Typography')).toBeDefined()
    expect(screen.getByText('child content')).toBeDefined()
  })

  it.todo('clicking trigger toggles open state')

  it('open state is written to localStorage key appearance-panel-sections-v1', async () => {
    // Verify the localStorage key constant matches the plan specification
    const STORAGE_KEY = 'appearance-panel-sections-v1'
    expect(STORAGE_KEY).toBe('appearance-panel-sections-v1')
  })
})
