'use client'

import Link from 'next/link'
import Image from 'next/image'
import { useRouter } from 'next/navigation'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/lib/stores/auth-store'
import { AppNav } from '@/components/AppNav'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { ProtectedRoute } from '@/components/ProtectedRoute'

function SettingsContent() {
  const router = useRouter()
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)

  async function handleDeleteAccount() {
    try {
      await authApi.deleteAccount()
      toastManager.add({ title: 'Account deleted', type: 'success' })
      logout()
      router.replace('/')
    } catch {
      toastManager.add({
        title: 'Failed to delete account',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }

  if (!user) return null

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <h1 className="text-2xl font-bold text-text">Settings</h1>

        {/* Profile section */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Profile</h2>
          <div className="space-y-3">
            {user.profile_image_url && (
              <div className="mb-4 flex items-center gap-3">
                <Image
                  src={user.profile_image_url}
                  alt={user.display_name}
                  width={48}
                  height={48}
                  className="rounded-full object-cover"
                />
                <span className="text-lg font-medium text-text">{user.display_name}</span>
              </div>
            )}
            <div>
              <span className="text-sm text-text-sub">Username</span>
              <p className="font-medium text-text">{user.username}</p>
            </div>
            <div>
              <span className="text-sm text-text-sub">Primary Platform</span>
              <p className="font-medium text-text capitalize">{user.auth_provider ?? 'Unknown'}</p>
            </div>
          </div>
        </Card>

        {/* Data & Privacy section */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Data &amp; Privacy</h2>
          <p className="mb-4 text-sm text-text-sub">
            We keep data collection minimal and transparent. Review the policies below for details
            about how tokens, overlays, and chat metadata are processed.
          </p>
          <div className="flex flex-col gap-3">
            <Link
              href="/legal/privacy"
              className="inline-flex items-center justify-between rounded-lg border border-border px-4 py-3 text-sm text-text transition-colors hover:bg-surface-2"
            >
              <span>Privacy Policy</span>
              <span aria-hidden="true">→</span>
            </Link>
            <Link
              href="/legal/terms"
              className="inline-flex items-center justify-between rounded-lg border border-border px-4 py-3 text-sm text-text transition-colors hover:bg-surface-2"
            >
              <span>Terms of Service</span>
              <span aria-hidden="true">→</span>
            </Link>
          </div>
        </Card>

        {/* Danger zone */}
        <Card className="border-destructive/20 p-6">
          <h2 className="text-destructive mb-2 text-lg font-semibold">Danger Zone</h2>
          <p className="mb-4 text-sm text-text-sub">
            Deleting your account removes all overlays, OAuth grants, and cached chat sources. This
            action is permanent and cannot be undone.
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
                <Button variant="destructive" onClick={handleDeleteAccount}>
                  Yes, delete my account
                </Button>
              </div>
            </Dialog.Content>
          </Dialog.Root>
        </Card>
      </main>
    </div>
  )
}

export default function SettingsPage() {
  return (
    <ProtectedRoute>
      <SettingsContent />
    </ProtectedRoute>
  )
}
