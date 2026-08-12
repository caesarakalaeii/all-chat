'use client'

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

import Link from 'next/link'
import Image from 'next/image'
import { useRouter, useSearchParams } from 'next/navigation'
import { useEffect, useState } from 'react'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useOnboardingStore } from '@/lib/stores/onboarding-store'
import { AppNav } from '@/components/AppNav'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { toastManager } from '@/lib/toast'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { AmbassadorSettingsCard } from '@/components/settings/AmbassadorSettingsCard'
import {
  getGuilds,
  disconnectGuild,
  startDiscordOAuth,
  getDiscordIdentity,
  startDiscordAccountLink,
  unlinkDiscordAccount,
} from '@/lib/api/discord'
import type { DiscordGuild, DiscordIdentity } from '@/lib/api/discord'

function SettingsContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)

  const [guilds, setGuilds] = useState<DiscordGuild[]>([])
  const [guildsLoading, setGuildsLoading] = useState(true)
  const [disconnectTarget, setDisconnectTarget] = useState<DiscordGuild | null>(null)
  const [identity, setIdentity] = useState<DiscordIdentity | null>(null)
  const [identityLoading, setIdentityLoading] = useState(true)
  const [restartingGuide, setRestartingGuide] = useState(false)
  const initAuth = useAuthStore((state) => state.init)
  const startOnboarding = useOnboardingStore((state) => state.start)

  // Re-arm the first-run setup guide: clear the server flag, refresh
  // /auth/me, then start explicitly (bypasses the zero-overlay auto-start
  // guard so users with existing overlays can re-walk the steps too).
  async function handleRestartGuide() {
    setRestartingGuide(true)
    try {
      await authApi.updateOnboarding(false)
      await initAuth()
      startOnboarding('settings')
      router.push('/dashboard')
    } catch {
      toastManager.add({
        title: 'Could not restart the setup guide',
        description: 'Please try again.',
        type: 'error',
      })
      setRestartingGuide(false)
    }
  }

  async function fetchGuilds() {
    try {
      const data = await getGuilds()
      setGuilds(data)
    } catch {
      // silently ignore — user may not have Discord connected
    } finally {
      setGuildsLoading(false)
    }
  }

  useEffect(() => {
    fetchGuilds()
  }, [])

  useEffect(() => {
    if (searchParams.get('discord') === 'connected') {
      toastManager.add({ title: 'Discord server connected!', type: 'success' })
      router.replace('/settings')
      fetchGuilds()
    }
  }, [searchParams])

  // The account link is fetched separately from the guilds: a user can have servers connected and
  // no link, or the reverse, and conflating them would hide whichever is missing.
  //
  // Keyed on the just-linked marker rather than refetched from the callback effect below, so every
  // setState here happens in a promise callback: `react-hooks/set-state-in-effect` is a build
  // error and it follows the call, so an effect must not invoke anything that setStates. When the
  // marker is cleared from the URL the key flips and this re-reads the freshly linked identity.
  const justLinked = searchParams.get('discord_account') === 'linked'
  useEffect(() => {
    let cancelled = false
    getDiscordIdentity()
      .then((data) => {
        if (!cancelled) setIdentity(data)
      })
      .catch(() => {
        // A failed read is reported as "not linked": the link is an affordance, and offering it
        // again is harmless, whereas claiming a link we could not confirm is not.
        if (!cancelled) setIdentity({ linked: false })
      })
      .finally(() => {
        if (!cancelled) setIdentityLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [justLinked])

  // The account-link callback lands here with one of these markers. `already_linked` is the one
  // needing real copy: it means that Discord account backs a DIFFERENT All-Chat account, which no
  // amount of retrying fixes — one Discord identity may back at most one account, or a second
  // account could inherit the first's server permissions.
  useEffect(() => {
    if (justLinked) {
      toastManager.add({ title: 'Discord account linked!', type: 'success' })
      router.replace('/settings')
      return
    }
    const error = searchParams.get('error')
    if (!error) return
    toastManager.add({
      title:
        error === 'already_linked'
          ? 'That Discord account is already linked to another All-Chat account'
          : 'Could not link your Discord account',
      description: error === 'already_linked' ? undefined : 'Please try again.',
      type: 'error',
    })
    router.replace('/settings')
  }, [justLinked, searchParams, router])

  async function handleUnlinkDiscordAccount() {
    try {
      await unlinkDiscordAccount()
      setIdentity({ linked: false })
      toastManager.add({ title: 'Discord account unlinked', type: 'success' })
    } catch {
      toastManager.add({ title: 'Could not unlink your Discord account', type: 'error' })
    }
  }

  async function handleDisconnectGuild() {
    if (!disconnectTarget) return
    const targetId = disconnectTarget.guild_id
    setDisconnectTarget(null)
    try {
      await disconnectGuild(targetId)
      setGuilds((prev) => prev.filter((g) => g.guild_id !== targetId))
    } catch {
      toastManager.add({
        title: 'Failed to disconnect server',
        description: 'Please try again.',
        type: 'error',
      })
    }
  }

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
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
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

        {/* Setup guide section */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Setup guide</h2>
          <div className="flex items-center justify-between gap-4">
            <p className="text-sm text-text-sub">
              Walk through overlay setup again: create an overlay, connect chat, pick a theme, and
              get the OBS link.
            </p>
            <Button
              variant="outline"
              disabled={restartingGuide}
              onClick={() => void handleRestartGuide()}
            >
              {restartingGuide ? 'Restarting…' : 'Restart'}
            </Button>
          </div>
        </Card>

        {/* Premium section */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Premium</h2>
          <div className="flex items-center justify-between gap-4">
            <p className="text-sm text-text-sub">
              Unlock premium features by backing All-Chat on Patreon.
            </p>
            <Link
              href="/settings/premium"
              className="inline-flex items-center justify-center rounded-lg border border-border px-4 py-2 text-sm text-text transition-colors hover:bg-surface-2"
            >
              Manage Premium
            </Link>
          </div>
        </Card>

        {/* Ambassador section (ADR-0041) — only for ambassadors; opt in/out of the
            public homepage showcase. */}
        {user.is_ambassador && <AmbassadorSettingsCard />}

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

        {/* Discord section */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">Discord</h2>

          {guildsLoading ? (
            <div role="status">
              <VisuallyHidden>Loading Discord servers</VisuallyHidden>
              <Skeleton className="h-10 w-full" />
            </div>
          ) : guilds.length === 0 ? (
            <div className="flex items-center justify-between">
              <p className="text-sm text-text-sub">No Discord server connected.</p>
              <Button onClick={startDiscordOAuth}>Connect Discord Server</Button>
            </div>
          ) : (
            <div className="space-y-3">
              {guilds.map((guild) => (
                <div key={guild.guild_id} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    {guild.guild_icon ? (
                      <Image
                        src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.guild_icon}.png?size=64`}
                        alt={guild.guild_name}
                        width={32}
                        height={32}
                        className="rounded-full object-cover"
                      />
                    ) : (
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-surface text-sm font-medium text-text-sub">
                        {guild.guild_name.charAt(0).toUpperCase()}
                      </div>
                    )}
                    <span className="font-medium text-text">{guild.guild_name}</span>
                  </div>
                  <Dialog.Root
                    open={disconnectTarget?.guild_id === guild.guild_id}
                    onOpenChange={(open) => {
                      if (!open) setDisconnectTarget(null)
                    }}
                  >
                    <Dialog.Trigger
                      render={
                        <Button variant="destructive" onClick={() => setDisconnectTarget(guild)}>
                          Disconnect
                        </Button>
                      }
                    />
                    <Dialog.Content showCloseButton={false}>
                      <Dialog.Title>Disconnect {guild.guild_name}?</Dialog.Title>
                      <Dialog.Description>
                        This will remove all Discord sources connected to {guild.guild_name}.
                      </Dialog.Description>
                      <div className="mt-6 flex justify-end gap-3">
                        <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                        <Button variant="destructive" onClick={handleDisconnectGuild}>
                          Yes, disconnect
                        </Button>
                      </div>
                    </Dialog.Content>
                  </Dialog.Root>
                </div>
              ))}
              <div className="pt-2">
                <Button variant="ghost" className="text-sm" onClick={startDiscordOAuth}>
                  Connect another server
                </Button>
              </div>
            </div>
          )}

          {/* The Discord ACCOUNT link, kept visually apart from the servers above because it is a
              different kind of thing: those record which servers this account controls, this
              records who you are on Discord. Moderation needs both — Discord has no per-user
              moderation API, so the shared bot writes and All-Chat checks real people's server
              permissions itself (ADR-0048). No token is kept. */}
          <div className="mt-6 border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-text">Your Discord account</h3>
            {identityLoading ? (
              <div role="status" className="mt-3">
                <VisuallyHidden>Loading your Discord account link</VisuallyHidden>
                <Skeleton className="h-9 w-full" />
              </div>
            ) : (
              <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-text-sub">
                  {identity?.linked
                    ? `Linked as ${identity.discord_username ?? 'your Discord account'}. Needed so moderators can act on your Discord servers.`
                    : 'Not linked. Link it to let moderators you invite act on your Discord servers.'}
                </p>
                {identity?.linked ? (
                  <Button variant="outline" onClick={handleUnlinkDiscordAccount}>
                    Unlink
                  </Button>
                ) : (
                  <Button onClick={() => void startDiscordAccountLink('settings')}>
                    Link Discord account
                  </Button>
                )}
              </div>
            )}
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
