import type { Meta, StoryObj } from '@storybook/react'

// Inline placeholder component — replace with real page import after migration
function SettingsPlaceholder() {
  return (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Settings — Pending Migration
    </div>
  )
}

const meta: Meta<typeof SettingsPlaceholder> = {
  title: 'Pages/Settings',
  component: SettingsPlaceholder,
}
export default meta
type Story = StoryObj<typeof SettingsPlaceholder>

export const Default: Story = {}
