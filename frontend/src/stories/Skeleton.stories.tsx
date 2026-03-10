import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/skeleton.tsx is created in Plan 04
function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="skeleton"
      className={className}
      style={{ background: 'var(--color-surface, #2a2a2a)', borderRadius: '4px', animation: 'pulse 2s cubic-bezier(0.4,0,0.6,1) infinite' }}
      {...props}
    />
  )
}

function SkeletonTextLines() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
      <Skeleton style={{ height: '16px', width: '256px' }} />
      <Skeleton style={{ height: '16px', width: '200px' }} />
      <Skeleton style={{ height: '16px', width: '224px' }} />
    </div>
  )
}

const meta = {
  title: 'UI/Skeleton',
  component: Skeleton,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Skeleton>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  args: { style: { height: '16px', width: '128px' } },
}
export const Card: Story = {
  args: { style: { height: '128px', width: '256px' } },
}
export const Text: StoryObj = {
  render: () => <SkeletonTextLines />,
}
