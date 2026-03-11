import type { Meta, StoryObj } from '@storybook/react'

// Inline placeholder component — replace with real page import after migration
function OverlayEditorPlaceholder() {
  return (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Overlay Editor — Pending Migration
    </div>
  )
}

const meta: Meta<typeof OverlayEditorPlaceholder> = {
  title: 'Pages/OverlayEditor',
  component: OverlayEditorPlaceholder,
}
export default meta
type Story = StoryObj<typeof OverlayEditorPlaceholder>

export const Default: Story = {}

export const SplitView: Story = {
  render: () => (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Overlay Editor — Split-View Preview (FEAT-01 stub)
    </div>
  ),
}
