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
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

function FieldDemo() {
  return (
    <Field.Root className="w-72">
      <Field.Label>Channel name</Field.Label>
      <Field.Control render={<Input placeholder="e.g. caesarlp" />} />
      <Field.Description>The channel whose chat appears on the overlay.</Field.Description>
    </Field.Root>
  )
}

const meta = {
  title: 'UI/Field',
  component: FieldDemo,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof FieldDemo>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
