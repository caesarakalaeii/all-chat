import type { Meta, StoryObj } from '@storybook/react'

// Inline placeholder component — replace with real page import after migration
function AdminLayoutPlaceholder() {
  return (
    <div className="min-h-screen bg-bg flex items-center justify-center text-text">
      Admin Layout — Pending Migration
    </div>
  )
}

const meta: Meta<typeof AdminLayoutPlaceholder> = {
  title: 'Pages/Admin',
  component: AdminLayoutPlaceholder,
}
export default meta
type Story = StoryObj<typeof AdminLayoutPlaceholder>

export const Default: Story = {}
