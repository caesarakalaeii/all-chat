/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

function AlertDialogDemo() {
  const [open, setOpen] = React.useState(false)
  return (
    <AlertDialog.Root open={open} onOpenChange={setOpen}>
      <AlertDialog.Trigger render={<Button variant="destructive">Delete overlay</Button>} />
      <AlertDialog.Content>
        <AlertDialog.Title>Delete this overlay?</AlertDialog.Title>
        <AlertDialog.Description>
          This removes the overlay and all its chat sources. This cannot be undone.
        </AlertDialog.Description>
        <div className="mt-4 flex justify-end gap-2">
          {/* Cancel first in DOM order = initial focus on the safe action */}
          <AlertDialog.Close render={<Button variant="ghost">Cancel</Button>} />
          <Button variant="destructive" onClick={() => setOpen(false)}>
            Delete
          </Button>
        </div>
      </AlertDialog.Content>
    </AlertDialog.Root>
  )
}

const meta = {
  title: 'UI/AlertDialog',
  component: AlertDialogDemo,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof AlertDialogDemo>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
