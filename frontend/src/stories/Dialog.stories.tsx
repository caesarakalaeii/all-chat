import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

// Storybook: need a wrapper with useState for controlled open state
function DialogDemo({ size }: { size?: 'sm' | 'default' | 'lg' }) {
  const [open, setOpen] = React.useState(false)
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger render={<Button variant="outline">Open Dialog</Button>} />
      <DialogContent size={size}>
        <DialogTitle>Dialog Title</DialogTitle>
        <DialogDescription>Dialog description and content goes here.</DialogDescription>
        <div className="mt-4 flex gap-2 justify-end">
          <Button variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
          <Button onClick={() => setOpen(false)}>Confirm</Button>
        </div>
      </DialogContent>
    </Dialog.Root>
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

export const Default: Story = {}
export const Small: Story = { args: { size: 'sm' } }
export const Large: Story = { args: { size: 'lg' } }
