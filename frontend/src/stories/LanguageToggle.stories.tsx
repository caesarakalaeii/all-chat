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
import { LanguageToggle } from '@/components/home/LanguageToggle'

// Rendered inside the real nav chip so axe checks the ink against the chip,
// which is what it sits on over the homepage lanes.
function NavChip() {
  return (
    <nav className="lanes-chrome" style={{ position: 'static' }} aria-label="Main">
      <LanguageToggle />
    </nav>
  )
}

const meta = {
  title: 'Home/LanguageToggle',
  component: NavChip,
  parameters: { layout: 'centered' },
} satisfies Meta<typeof NavChip>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
