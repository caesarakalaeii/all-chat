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
import { Switch } from '@/components/ui/switch'
import { Field } from '@/components/ui/field'
import { ToggleSwitch } from '@/components/appearance/ToggleSwitch'

// The axe gate (a11y test: 'error') verifies every rendered switch has an
// accessible name — the exact defect the old hand-rolled ToggleSwitch had.
function SwitchDemo() {
  const [checked, setChecked] = React.useState(false)
  return (
    <Field.Root className="w-64 flex-row items-center justify-between">
      <Field.Label>Enable notifications</Field.Label>
      <Switch.Root checked={checked} onCheckedChange={setChecked}>
        <Switch.Thumb />
      </Switch.Root>
    </Field.Root>
  )
}

function ToggleSwitchDemo() {
  const [checked, setChecked] = React.useState(true)
  return (
    <div className="w-64">
      <ToggleSwitch label="Show timestamps" checked={checked} onChange={setChecked} />
    </div>
  )
}

const meta = {
  title: 'UI/Switch',
  component: SwitchDemo,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof SwitchDemo>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const AppearanceToggleRow: Story = { render: () => <ToggleSwitchDemo /> }
export const Disabled: Story = {
  render: () => (
    <Field.Root className="w-64 flex-row items-center justify-between">
      <Field.Label>Locked setting</Field.Label>
      <Switch.Root disabled checked={false}>
        <Switch.Thumb />
      </Switch.Root>
    </Field.Root>
  ),
}
