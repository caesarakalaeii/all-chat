import type { Meta, StoryObj } from '@storybook/react'

// Inline placeholder component — replace with real page import after migration
function LandingPagePlaceholder() {
  return (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Landing Page — Pending Migration
    </div>
  )
}

const meta: Meta<typeof LandingPagePlaceholder> = {
  title: 'Pages/Landing',
  component: LandingPagePlaceholder,
}
export default meta
type Story = StoryObj<typeof LandingPagePlaceholder>

export const Default: Story = {}
