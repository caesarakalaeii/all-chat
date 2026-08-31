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
 * Copy lock for sign-in, the OAuth callback catchers and the device-link
 * approve screen. See __tests__/dashboard.test.ts for why the copy is pinned
 * here rather than through a rendered-output diff.
 *
 * /link is the only human decision point in the ADR-0049 device pairing flow,
 * and its page docblock lists four things its copy must make unmistakable --
 * which overlay, which scopes, that the device name is unverified, and the
 * honest limit of the overlay binding. Those four are the reason this lock is
 * exact rather than approximate.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('device link page chrome', () => {
  it('keeps the page heading and its revoke pointer whole', () => {
    expect(t('auth.link.title')).toBe('Link a control surface')
    expect(t('auth.link.intro')).toBe(
      'Approve a Stream Deck, StreamController or other desktop control surface. Nothing is copied or pasted — the credential goes straight to the plugin, and you can revoke it any time from {devices}.'
    )
    expect(t('auth.link.introDevices')).toBe('your paired devices')
  })

  it('keeps the loading announcement', () => {
    expect(t('auth.link.loading')).toBe('Loading link request')
  })
})

describe('device link code entry', () => {
  it('keeps the typed-code form copy', () => {
    expect(t('auth.link.codeTitle')).toBe('Enter the pairing code')
    expect(t('auth.link.codeBody')).toBe(
      'Your plugin is showing an eight-character code. Type it here. This is the path for a control surface on a different machine from this browser.'
    )
    expect(t('auth.link.codeLabel')).toBe('Pairing code')
    expect(t('auth.link.codePlaceholder')).toBe('ABCD-EFGH')
    expect(t('auth.link.codeHint')).toBe(
      'Case does not matter, and the dash is optional. Codes expire after ten minutes.'
    )
    expect(t('auth.link.codeChecking')).toBe('Checking…')
    expect(t('auth.link.codeContinue')).toBe('Continue')
  })

  it('keeps the rejected-code error', () => {
    expect(t('auth.link.codeInvalid')).toBe(
      'That code is not valid. Check the code your plugin is showing, or start linking again from the plugin.'
    )
  })
})

describe('device link approval', () => {
  it('keeps the self-reported name warning whole', () => {
    // Two emphasised runs, and the device name is attacker-influenced text from
    // the plugin, so it is a parameter rather than part of the sentence.
    expect(t('auth.link.approveTitle')).toBe('A device wants to control your chat')
    expect(t('auth.link.approveBody', { name: 'Stream Deck' })).toBe(
      'It says it is “Stream Deck”. That name is {selfReported} — we have no way to verify it, so treat it as a hint, not proof. If you did not just start linking a control surface, deny this.'
    )
    expect(t('auth.link.approveSelfReported')).toBe('self-reported by the plugin')
  })

  it('keeps the overlay choice copy', () => {
    expect(t('auth.link.overlayLegend')).toBe('Which overlay may it control?')
    expect(t('auth.link.overlayBody')).toBe(
      'The device is locked to this overlay for as long as it stays paired. To move it, revoke it and link it again.'
    )
    expect(t('auth.link.overlayNone')).toBe(
      'You have no overlays yet. Create one first, then link your device.'
    )
  })

  it('keeps the scope choice copy', () => {
    expect(t('auth.link.scopeLegend')).toBe('What may it do?')
    expect(t('auth.link.scopeBody')).toBe(
      'The plugin asked for the items below. Turn off anything you do not want it to have — you can grant less than it asked for, never more.'
    )
    expect(t('auth.link.scopeNone')).toBe(
      'Pick at least one — a device with none can sign in but do nothing.'
    )
  })

  it('keeps the honest limit of the overlay binding', () => {
    // Point 4 of the page docblock. Saying this plainly is deliberate: chat
    // send has no overlay dimension, and implying otherwise would promise a
    // guarantee the token does not carry.
    expect(t('auth.link.chatWriteCaveat')).toBe(
      'Worth knowing: sending chat is not per-overlay. It goes to every platform your account has connected, so this permission is not narrowed by the overlay you picked above. Only the poll and prediction actions are.'
    )
  })

  it('names each requestable scope', () => {
    expect(t('auth.link.scopeChatWriteTitle')).toBe('Send chat messages')
    expect(t('auth.link.scopeChatWriteBody')).toBe(
      'Lets this device post messages to your connected chats.'
    )
    expect(t('auth.link.scopeEngagementWriteTitle')).toBe('Run polls and predictions')
    expect(t('auth.link.scopeEngagementWriteBody')).toBe(
      'Lets this device open, close, resolve and cancel polls and predictions.'
    )
    // An unrecognised scope string still has to render something, so the
    // fallback description is its own key. The title falls back to the raw
    // scope, which is not copy.
    expect(t('auth.link.scopeUnknownBody')).toBe('Requested by this device.')
  })

  it('keeps the device naming field copy', () => {
    expect(t('auth.link.nameLabel')).toBe('Name this device')
    expect(t('auth.link.namePlaceholder')).toBe('Stream Deck (studio PC)')
    expect(t('auth.link.nameHint')).toBe(
      "Shown in your paired-devices list, so you know what you are revoking later. Starts as the plugin's own suggestion; change it to anything you like."
    )
  })

  it('keeps the approve and deny actions and their outcomes', () => {
    expect(t('auth.link.approving')).toBe('Approving…')
    expect(t('auth.link.approve')).toBe('Approve this device')
    expect(t('auth.link.deny')).toBe('Deny')
    expect(t('auth.link.approvedTitle')).toBe('Device approved')
    expect(t('auth.link.approvedBody')).toBe(
      'Return to your plugin — it will finish linking on its own.'
    )
    expect(t('auth.link.deniedTitle')).toBe('Device denied')
  })

  it('keeps every failure the page can report', () => {
    expect(t('auth.link.approveFailed')).toBe(
      'Could not approve this device. The request may have expired — start linking again from the plugin.'
    )
    expect(t('auth.link.denyFailed')).toBe(
      'Could not deny this request. It will expire on its own within ten minutes.'
    )
    expect(t('auth.link.overlaysFailed')).toBe(
      'Could not load your overlays. Refresh the page to try again.'
    )
    expect(t('auth.link.requestExpired')).toBe(
      'This link request has expired or was already used. Start linking again from the plugin.'
    )
  })
})

describe('streamer OAuth callback', () => {
  it('keeps the progress and failure copy', () => {
    expect(t('auth.callback.authenticating')).toBe('Authenticating...')
    expect(t('auth.callback.noCode')).toBe('No authentication code received')
    expect(t('auth.callback.exchangeFailed')).toBe(
      'Authentication failed. The code may have expired — please try again.'
    )
    expect(t('auth.callback.failed')).toBe('Authentication failed. Please try again.')
    expect(t('auth.callback.returnHome')).toBe('Return to Home')
  })
})

describe('viewer OAuth success catcher', () => {
  it('keeps the progress copy', () => {
    expect(t('auth.viewerSuccess.completing')).toBe('Completing authentication...')
    expect(t('auth.viewerSuccess.pleaseWait')).toBe('Please wait')
  })

  it('keeps the extension-popup outcome', () => {
    expect(t('auth.viewerSuccess.succeeded')).toBe('Authentication successful!')
    expect(t('auth.viewerSuccess.closeWindow')).toBe('You can close this window')
  })

  it('keeps the two viewer exchange failures', () => {
    expect(t('auth.viewerSuccess.noCode')).toBe('No authentication code received')
    expect(t('auth.viewerSuccess.exchangeFailed')).toBe(
      'Authentication failed. The code may have expired — please try again.'
    )
    expect(t('auth.viewerSuccess.viewerInfoFailed')).toBe(
      'Failed to complete authentication. Please try again.'
    )
  })
})

describe('viewer OAuth error catcher', () => {
  it('keeps the failure page copy', () => {
    expect(t('auth.viewerError.title')).toBe('Authentication Failed')
    expect(t('auth.viewerError.defaultError')).toBe('Authentication failed')
    expect(t('auth.viewerError.returnHome')).toBe('Return to Home')
    expect(t('auth.viewerError.goBack')).toBe('Go Back')
  })

  it('names the failing account, or stays neutral when the platform is unknown', () => {
    // The page used to hardcode "Twitch" for every platform. The neutral
    // fallback is what stops an unrecognised ?platform= from misnaming the
    // provider that actually failed.
    expect(t('auth.viewerError.body', { account: 'Twitch account' })).toBe(
      'There was an error authenticating with your Twitch account. Please try again or contact support if the problem persists.'
    )
    expect(t('auth.viewerError.accountNamed', { platform: 'Kick' })).toBe('Kick account')
    expect(t('auth.viewerError.accountNeutral')).toBe('streaming account')
  })
})
