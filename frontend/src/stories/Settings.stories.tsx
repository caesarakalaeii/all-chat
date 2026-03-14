import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'
import { Dialog } from '@/components/ui/dialog'

function SettingsPagePreview() {
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="max-w-2xl mx-auto space-y-6">
        <h1 className="text-2xl font-bold text-text">Settings</h1>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-4">Profile</h2>
          <div className="space-y-3">
            <div>
              <span className="text-sm text-text-sub">Username</span>
              <p className="text-text font-medium">streamer123</p>
            </div>
            <div>
              <span className="text-sm text-text-sub">Primary Platform</span>
              <p className="text-text font-medium">Twitch</p>
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-4">Connected Platforms</h2>
          <div className="flex gap-2">
            {(['twitch', 'youtube', 'kick'] as const).map(p => (
              <PlatformBadge key={p} platform={p} />
            ))}
          </div>
        </Card>

        <Card className="p-6 border-destructive/20">
          <h2 className="text-lg font-semibold text-destructive mb-2">Danger Zone</h2>
          <p className="text-text-sub text-sm mb-4">
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
              <div className="flex gap-3 justify-end mt-6">
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
