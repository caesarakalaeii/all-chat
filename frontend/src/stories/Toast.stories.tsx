import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/toast.tsx is created in Plan 03
function Toast({ children, className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="toast"
      className={className}
      style={{ padding: '12px 16px', borderRadius: '6px', minWidth: '280px', display: 'flex', alignItems: 'center', gap: '8px' }}
      {...props}
    >
      {children}
    </div>
  )
}

const meta = {
  title: 'UI/Toast',
  component: Toast,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Toast>

export default meta
type Story = StoryObj<typeof meta>

export const Success: Story = {
  args: {
    children: 'Operation successful!',
    style: { background: '#166534', color: '#dcfce7' },
  },
}
export const Error: Story = {
  args: {
    children: 'Something went wrong.',
    style: { background: '#991b1b', color: '#fee2e2' },
  },
}
export const Info: Story = {
  args: {
    children: 'Here is some information.',
    style: { background: '#1e3a5f', color: '#dbeafe' },
  },
}
