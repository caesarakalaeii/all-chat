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

/**
 * Copy lock for the strings shared across surfaces. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('shared platform names', () => {
  it('names every platform the UI labels', () => {
    // Two surfaces read these: the moderator roster, which delegates to Twitch,
    // YouTube, Kick and Discord, and the bubble colour picker, which colours
    // rows from all five. They live in common.* rather than in either namespace
    // because neither surface owns them.
    expect(t('common.platforms.twitch')).toBe('Twitch')
    expect(t('common.platforms.youtube')).toBe('YouTube')
    expect(t('common.platforms.kick')).toBe('Kick')
    expect(t('common.platforms.tiktok')).toBe('TikTok')
    expect(t('common.platforms.discord')).toBe('Discord')
  })
})

describe('Patreon subscription copy', () => {
  it('keeps the strings /settings/premium and /settings/viewer/premium both render', () => {
    // Both pages drive the same Patreon connect flow with byte-identical copy.
    // Only the strings that differ between them stay in settings.*.
    expect(t('common.patreon.heading')).toBe('Patreon')
    expect(t('common.patreon.connect')).toBe('Connect Patreon')
    expect(t('common.patreon.connecting')).toBe('Redirecting…')
    expect(t('common.patreon.notAPatron')).toBe('Not a patron yet?')
    expect(t('common.patreon.subscribe')).toBe('Subscribe on Patreon')
    expect(t('common.patreon.subscriptionRow')).toBe('Subscription')
    expect(t('common.patreon.renewsRow')).toBe('Renews')
    expect(t('common.patreon.active')).toBe('Active')
    expect(t('common.patreon.inactive')).toBe('Inactive')
    expect(t('common.patreon.disconnect')).toBe('Disconnect Patreon')
    expect(t('common.patreon.disconnectTitle')).toBe('Disconnect Patreon?')
    expect(t('common.patreon.disconnectCancel')).toBe('Cancel')
    expect(t('common.patreon.disconnectConfirm')).toBe('Yes, disconnect')
  })

  it('keeps the four shared pledge statuses', () => {
    // The fifth, "expired", differs per page ("Below premium tier" vs "Below
    // viewer tier"), so it stays in each page's own namespace.
    expect(t('common.patreon.statusActive')).toBe('Active')
    expect(t('common.patreon.statusDeclined')).toBe('Payment declined')
    expect(t('common.patreon.statusFormer')).toBe('Ended')
    expect(t('common.patreon.statusNotSubscribed')).toBe('Not subscribed')
  })
})

describe('shared sound preset names', () => {
  it('names the three soundPlayer presets', () => {
    // Two surfaces read these: the overlay editor's on-stream notification
    // sounds and the monitor view's private activity sound. Casing rules are
    // language-specific, so the display name cannot be derived from the stored
    // lowercase value by capitalising it.
    expect(t('common.soundPresets.chime')).toBe('Chime')
    expect(t('common.soundPresets.pop')).toBe('Pop')
    expect(t('common.soundPresets.ping')).toBe('Ping')
  })
})

describe('shared wordmark', () => {
  it('spells the product wordmark once for every nav that renders it', () => {
    // Four surfaces render it: the marketing header, the app nav, the admin rail
    // and the admin top nav. It is lowercase deliberately — the wordmark is set
    // in lowercase everywhere, unlike the 'All-Chat' of prose and aria labels.
    expect(t('common.brand.wordmark')).toBe('all-chat')
  })
})

describe('app nav copy', () => {
  it('keeps the logged-in nav link labels', () => {
    // AppNav is chrome on every logged-in surface (dashboard, settings,
    // overlays, docs), so its copy is shared rather than owned by one of them.
    expect(t('common.appNav.dashboard')).toBe('Dashboard')
    expect(t('common.appNav.flairs')).toBe('Flairs')
    expect(t('common.appNav.admin')).toBe('Admin')
    expect(t('common.appNav.settings')).toBe('Settings')
    expect(t('common.appNav.docs')).toBe('Docs')
    expect(t('common.appNav.logOut')).toBe('Log out')
  })
})

describe('compact duration copy', () => {
  it('keeps the four compact duration shapes', () => {
    // formatCompactDuration in lib/utils.ts assembled these from unit letters.
    // They live in common rather than admin.* because the helper is generic
    // infrastructure with no owning surface; the unit letters are copy in their
    // own right, since a language that does not abbreviate days as 'd' has
    // nowhere else to say so.
    expect(t('common.duration.justNow')).toBe('just now')
    expect(t('common.duration.minutes', { minutes: 8 })).toBe('8m')
    expect(t('common.duration.hours', { hours: 3 })).toBe('3h')
    expect(t('common.duration.hoursAndMinutes', { hours: 5, minutes: 12 })).toBe('5h 12m')
    expect(t('common.duration.days', { days: 3 })).toBe('3d')
    expect(t('common.duration.daysAndHours', { days: 3, hours: 4 })).toBe('3d 4h')
  })
})

describe('shared chrome copy', () => {
  it('keeps the two badge aria labels', () => {
    // AllChatBadge and PremiumBadge render on the overlay chat rows, the
    // appearance groups and the viewer settings tabs, so their labels belong to
    // no single surface.
    expect(t('common.badges.allChatLabel')).toBe('All-Chat badge')
    expect(t('common.badges.premiumLabel')).toBe('Premium badge')
  })

  it('keeps the dialog and toast dismiss labels', () => {
    // ui/dialog and ui/toast are primitives every surface mounts.
    expect(t('common.dialog.closeLabel')).toBe('Close dialog')
    expect(t('common.toast.closeLabel')).toBe('Close notification')
  })

  it('keeps the split pane controls', () => {
    // SplitView and ResizableSplit both label their divider 'Resize panels';
    // the step buttons and the preview iframe are SplitView's.
    expect(t('common.splitPane.resizeLabel')).toBe('Resize panels')
    expect(t('common.splitPane.shrinkConfigLabel')).toBe('Shrink config panel')
    expect(t('common.splitPane.growConfigLabel')).toBe('Grow config panel')
    expect(t('common.splitPane.previewTitle')).toBe('Overlay live preview')
  })

  it('keeps the CSS editor label, keyboard hint and loading state', () => {
    expect(t('common.cssEditor.regionLabel')).toBe('Custom CSS editor')
    expect(t('common.cssEditor.keyboardHint')).toBe(
      'Press Ctrl+M to toggle Tab capturing; Escape then Tab leaves the editor.'
    )
    expect(t('common.cssEditor.loading')).toBe('Loading editor...')
  })

  it('keeps the impersonation banner copy', () => {
    // 'Admin Mode:' and 'Viewing as' were two JSX runs either side of the
    // username. One sentence with the username as a param, so a language that
    // fronts the name can move it.
    expect(t('common.impersonation.bannerLabel')).toBe('Admin Mode:')
    expect(t('common.impersonation.viewingAs', { username: 'kate' })).toBe('Viewing as kate')
    expect(t('common.impersonation.exitButton')).toBe('Exit & Return to Admin')
  })

  it('keeps the moderating-elsewhere card copy', () => {
    // The sentence was three JSX runs around a bolded count, with the count's
    // noun switching on singular. Two whole sentences with the count as a param
    // and the bold applied through emphasise on the count phrase.
    expect(t('common.moderatingElsewhere.channelOne', { count: 1 })).toBe('1 channel')
    expect(t('common.moderatingElsewhere.channelMany', { count: 4 })).toBe('4 channels')
    expect(t('common.moderatingElsewhere.sentence', { channels: '4 channels' })).toBe(
      'You moderate 4 channels for other streamers.'
    )
    expect(t('common.moderatingElsewhere.openLink')).toBe('Open')
  })

  it('keeps the service announcements label', () => {
    // One key for both the aria-label and the title, which were byte-identical,
    // plus the popover heading that repeats it.
    expect(t('common.maintenanceInfo.buttonLabel')).toBe('Service announcements')
    expect(t('common.maintenanceInfo.popoverHeading')).toBe('Service announcements')
  })

  it('keeps the EventSub migration banner copy', () => {
    expect(t('common.eventSubMigration.body')).toBe(
      'The old IRC chat connection is being retired and can drop messages when many streams are live. Reconnect to move to the new connection and keep your chat reliable.'
    )
    expect(t('common.eventSubMigration.reconnectButton')).toBe('Reconnect now')
    expect(t('common.eventSubMigration.dismissLabel')).toBe('Dismiss')
  })

  it('keeps the 403 page copy', () => {
    expect(t('common.forbidden.heading')).toBe('403 Forbidden')
    expect(t('common.forbidden.body')).toBe(
      'You do not have permission to access this page. Admin privileges are required.'
    )
    expect(t('common.forbidden.dashboardButton')).toBe('Go to Dashboard')
  })

  it('keeps the beta warning dialog copy', () => {
    // Two platforms, two whole sets. The source built the titles and bodies by
    // splicing a capitalised platform name into a sentence; YouTube's copy
    // differs from TikTok's by more than that name, so a shared template with a
    // {platform} hole would have been a rewording.
    expect(t('common.betaWarning.youtubeTitle')).toBe(
      'YouTube \u2014 OAuth Verification in Progress'
    )
    expect(t('common.betaWarning.tiktokTitle')).toBe('TikTok \u2014 Closed Beta')
    expect(t('common.betaWarning.youtubeBody')).toBe(
      'YouTube integration is currently under Google OAuth verification review. We cannot add new test users during this period.'
    )
    expect(t('common.betaWarning.tiktokBody')).toBe(
      "TikTok integration is currently in closed beta. If you haven't been added to the beta program yet, authentication will fail."
    )
    expect(t('common.betaWarning.youtubeDiscordPrompt')).toBe(
      'Join our Discord community to stay updated on verification progress and get support:'
    )
    expect(t('common.betaWarning.tiktokDiscordPrompt')).toBe(
      'To join the beta, please join our Discord community:'
    )
    expect(t('common.betaWarning.youtubeExistingUser')).toBe(
      'If you were previously added as a test user, you can continue to use YouTube integration.'
    )
    expect(t('common.betaWarning.tiktokExistingUser')).toBe(
      "If you're already in the beta, you can proceed with authentication."
    )
    expect(t('common.betaWarning.discordLink')).toBe('Join Discord Server')
    expect(t('common.betaWarning.cancelButton')).toBe('Cancel')
    expect(t('common.betaWarning.continueButton')).toBe('I Understand, Continue')
  })
})

describe('shared toast copy', () => {
  it('keeps the try-again line used by three surfaces', () => {
    // /dashboard, /overlays/new and the onboarding create dialog all render
    // this as a toast description, so it is genuinely shared rather than
    // pre-populated. dashboard.toasts.tryAgain is folded into it.
    expect(t('common.toast.tryAgain')).toBe('Please try again.')
  })

  it('keeps the ElevenLabs fallback notice', () => {
    // Raised by both the viewer overlay and the editor's embedded preview, so
    // it belongs to neither surface's namespace.
    expect(t('common.toast.elevenLabsFallback')).toBe(
      'ElevenLabs unavailable \u2014 using browser voice.'
    )
  })

  it('keeps the EventSub migration toasts', () => {
    expect(t('common.eventSubMigration.connectedToast')).toBe('Twitch chat connected')
    expect(t('common.eventSubMigration.failedToast')).toBe('Could not start the upgrade')
  })
})

describe('Patreon connect toast copy', () => {
  it('keeps the five toasts both premium pages raise', () => {
    // /settings/premium and /settings/viewer/premium raise a byte-identical set
    // of five, so these are common.* by the two-callers-on-the-same-string rule
    // rather than duplicated into settings.premium.* and settings.viewerPremium.*.
    expect(t('common.patreon.connectedToast')).toBe('Patreon connected!')
    expect(t('common.patreon.connectFailedToast')).toBe('Could not connect Patreon')
    expect(t('common.patreon.connectStartFailedToast')).toBe('Could not start Patreon connect')
    expect(t('common.patreon.disconnectedToast')).toBe('Patreon disconnected')
    expect(t('common.patreon.disconnectFailedToast')).toBe('Failed to disconnect')
  })
})
