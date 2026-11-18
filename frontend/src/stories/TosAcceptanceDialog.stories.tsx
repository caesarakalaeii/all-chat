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
import TosAcceptanceDialog from '@/components/TosAcceptanceDialog'
import { useAuthStore } from '@/lib/stores/auth-store'

const meta = {
  title: 'Legal/TosAcceptanceDialog',
  component: TosAcceptanceDialog,
  parameters: { layout: 'fullscreen' },
  decorators: [
    (Story) => {
      useAuthStore.setState({
        user: {
          id: 'u1',
          username: 'streamer',
          display_name: 'Streamer',
          is_admin: false,
          is_premium: false,
          is_beta_tester: false,
          is_ambassador: false,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          tos_accepted: false,
        },
        loading: false,
        isImpersonating: false,
      })
      return <Story />
    },
  ],
} satisfies Meta<typeof TosAcceptanceDialog>

export default meta
type Story = StoryObj<typeof meta>

export const Pending: Story = {}
