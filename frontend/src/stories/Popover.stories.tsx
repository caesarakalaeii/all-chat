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
import { Popover } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { ColorPickerControl } from '@/components/appearance/ColorPickerControl'

function PopoverDemo() {
  return (
    <Popover.Root>
      <Popover.Trigger render={<Button variant="outline">Open popover</Button>} />
      <Popover.Content className="w-64">
        <Popover.Title>Quick settings</Popover.Title>
        <p className="mt-1 text-sm text-text-sub">
          Anchored panel with focus management and Escape-to-close.
        </p>
      </Popover.Content>
    </Popover.Root>
  )
}

function ColorPickerDemo() {
  const [color, setColor] = React.useState('#a37bff')
  return (
    <div className="w-80">
      <ColorPickerControl label="Username color" value={color} onChange={setColor} />
    </div>
  )
}

const meta = {
  title: 'UI/Popover',
  component: PopoverDemo,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof PopoverDemo>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const ColorPicker: Story = { render: () => <ColorPickerDemo /> }
