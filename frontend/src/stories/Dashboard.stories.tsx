import type { Meta, StoryObj } from '@storybook/react'

// Inline placeholder component — replace with real page import after migration
function DashboardPlaceholder() {
  return (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Dashboard — Pending Migration
    </div>
  )
}

const meta: Meta<typeof DashboardPlaceholder> = {
  title: 'Pages/Dashboard',
  component: DashboardPlaceholder,
}
export default meta
type Story = StoryObj<typeof DashboardPlaceholder>

export const Default: Story = {}

export const Loading: Story = {
  render: () => (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Dashboard loading state
    </div>
  ),
}

export const Empty: Story = {
  render: () => (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Dashboard empty state
    </div>
  ),
}
