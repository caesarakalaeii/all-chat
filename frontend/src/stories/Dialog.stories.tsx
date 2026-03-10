import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/dialog.tsx is created in Plan 03
function Dialog({ children, className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="dialog" className={className} {...props}>{children}</div>
}

function DialogDemo() {
  const [open, setOpen] = React.useState(false)
  return (
    <div>
      <button onClick={() => setOpen(true)} style={{ padding: '8px 16px', cursor: 'pointer' }}>
        Open Dialog
      </button>
      {open && (
        <Dialog style={{ position: 'fixed', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.5)' }}>
          <div style={{ background: 'var(--color-surface, #1a1a1a)', padding: '24px', borderRadius: '8px', minWidth: '300px' }}>
            <h2 style={{ margin: '0 0 16px' }}>Dialog Title</h2>
            <p style={{ margin: '0 0 16px' }}>Dialog content goes here.</p>
            <button onClick={() => setOpen(false)} style={{ padding: '8px 16px', cursor: 'pointer' }}>
              Close
            </button>
          </div>
        </Dialog>
      )}
    </div>
  )
}

const meta = {
  title: 'UI/Dialog',
  component: DialogDemo,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof DialogDemo>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: {} }
export const WithContent: Story = { args: {} }
