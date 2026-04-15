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
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'
import { Dialog } from '@/components/ui/dialog'

function SettingsPagePreview() {
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="mx-auto max-w-2xl space-y-6">
        <h1 className="text-2xl font-bold text-text">Settings</h1>

        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Profile</h2>
          <div className="space-y-3">
            <div>
              <span className="text-sm text-text-sub">Username</span>
              <p className="font-medium text-text">streamer123</p>
            </div>
            <div>
              <span className="text-sm text-text-sub">Primary Platform</span>
              <p className="font-medium text-text">Twitch</p>
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Connected Platforms</h2>
          <div className="flex gap-2">
            {(['twitch', 'youtube', 'kick'] as const).map((p) => (
              <PlatformBadge key={p} platform={p} />
            ))}
          </div>
        </Card>

        <Card className="border-destructive/20 p-6">
          <h2 className="text-destructive mb-2 text-lg font-semibold">Danger Zone</h2>
          <p className="mb-4 text-sm text-text-sub">
            Deleting your account is permanent and cannot be undone.
          </p>
          <Dialog.Root>
            <Dialog.Trigger render={<Button variant="destructive">Delete Account</Button>} />
            <Dialog.Content showCloseButton={false}>
              <Dialog.Title>Delete your account?</Dialog.Title>
              <Dialog.Description>
                This permanently deletes your account and all overlays. This action cannot be
                undone.
              </Dialog.Description>
              <div className="mt-6 flex justify-end gap-3">
                <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                <Button variant="destructive">Yes, delete my account</Button>
              </div>
            </Dialog.Content>
          </Dialog.Root>
        </Card>
      </div>
    </div>
  )
}

const meta: Meta<typeof SettingsPagePreview> = {
  title: 'Pages/Settings',
  component: SettingsPagePreview,
}
export default meta
type Story = StoryObj<typeof SettingsPagePreview>

export const Default: Story = {}
