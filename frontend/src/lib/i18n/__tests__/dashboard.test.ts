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
 * Copy lock for the dashboard surface.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. A rendered-output
 * diff across 229 files is not reviewable, so the strings that were at the
 * render sites are pinned here instead, transcribed from the pre-migration
 * source. If a key's text drifts, this fails and names the key.
 *
 * Values built from a placeholder are asserted through `t()` so the
 * interpolation is covered too, not just the template.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('dashboard copy', () => {
  it('keeps the overlay list chrome', () => {
    expect(t('dashboard.overlays.heading')).toBe('Overlays')
    expect(t('dashboard.overlays.newOverlay')).toBe('New Overlay')
    expect(t('dashboard.overlays.loading')).toBe('Loading overlays')
    expect(t('dashboard.overlays.extensionBadge')).toBe('Extension')
    expect(t('dashboard.overlays.deactivateExtension')).toBe('Deactivate Extension')
    expect(t('dashboard.overlays.setAsExtension')).toBe('Set as Extension Overlay')
  })

  it('keeps the empty state', () => {
    expect(t('dashboard.empty.heading')).toBe('No overlays yet')
    expect(t('dashboard.empty.body')).toBe(
      'Create your first overlay to see chat from all your platforms in one place.'
    )
    expect(t('dashboard.empty.createFirst')).toBe('Create your first overlay')
  })

  it('keeps the source count in one key per grammatical number', () => {
    // Was `{n} source{n !== 1 ? 's' : ''}` at the render site. Two keys rather
    // than a concatenated suffix: a language that inflects the noun cannot
    // rebuild it from an English 's'. There are no ICU plurals here, so the
    // caller picks the key.
    expect(t('dashboard.overlays.sourceCountOne', { count: 1 })).toBe('1 source')
    expect(t('dashboard.overlays.sourceCountOther', { count: 0 })).toBe('0 sources')
    expect(t('dashboard.overlays.sourceCountOther', { count: 3 })).toBe('3 sources')
  })

  it('keeps the delete dialog, typographic quotes included', () => {
    // The render site spelled these &ldquo;/&rdquo;; a catalog string is not
    // HTML, so they are the characters themselves and must render the same.
    expect(t('dashboard.deleteOverlay.title', { name: 'Main chat' })).toBe(
      'Delete \u201cMain chat\u201d?'
    )
    expect(t('dashboard.deleteOverlay.description')).toBe(
      'This action cannot be undone. All sources will be removed.'
    )
    expect(t('dashboard.deleteOverlay.cancel')).toBe('Cancel')
    expect(t('dashboard.deleteOverlay.confirm')).toBe('Delete')
    expect(t('dashboard.overlays.deleteLabel', { name: 'Main chat' })).toBe('Delete Main chat')
  })

  it('keeps the parked-discovery warning reading as action needed', () => {
    // Nothing has failed here, so no word in this copy may suggest a fault. It
    // also has to say what clears the state: only Rediscover from the monitor
    // does, and a browser-source refresh does not — which is the mistake the
    // whole warning exists to prevent.
    expect(t('dashboard.discoveryPaused.title')).toBe('YouTube discovery paused')
    expect(t('dashboard.discoveryPaused.body')).toBe(
      'No live stream was found for an hour, so All-Chat stopped looking. Re-discover from the chat monitor when you go live.'
    )
    expect(t('dashboard.discoveryPaused.action')).toBe('Open chat monitor')
  })

  it('keeps the overlay mutation toasts', () => {
    expect(t('dashboard.toasts.overlayDeleted')).toBe('Overlay deleted')
    expect(t('dashboard.toasts.overlayDeleteFailed')).toBe('Failed to delete overlay')
    // 'Please try again.' moved to common.toast.tryAgain when a third caller
    // appeared; common.test.ts asserts it now. Not dropped, relocated.
    expect(t('dashboard.toasts.extensionOverlayUpdated')).toBe('Extension overlay updated')
    expect(t('dashboard.toasts.extensionOverlayDeactivated')).toBe('Extension overlay deactivated')
    expect(t('dashboard.toasts.overlayUpdateFailed')).toBe('Failed to update overlay')
  })
})

describe('share requests page copy', () => {
  it('keeps the page chrome and its two tabs', () => {
    expect(t('dashboard.shares.heading')).toBe('Share Requests')
    // The count is part of the tab label, so it is a placeholder rather than a
    // fragment appended after the word.
    expect(t('dashboard.shares.tabPending', { count: '3' })).toBe('Pending (3)')
    expect(t('dashboard.shares.tabHistory', { count: '7' })).toBe('History (7)')
    expect(t('dashboard.shares.loading')).toBe('Loading requests...')
    expect(t('dashboard.shares.emptyPending')).toBe('No pending share requests')
    expect(t('dashboard.shares.emptyHistory')).toBe('No request history')
    // Stands in for a sender whose profile did not load.
    expect(t('dashboard.shares.unknownUser')).toBe('Unknown User')
  })

  it('keeps the five status badges', () => {
    expect(t('dashboard.shares.statusPending')).toBe('Pending')
    // 'accepted' reads "Active": the badge names the state of the share, not
    // the state of the request.
    expect(t('dashboard.shares.statusAccepted')).toBe('Active')
    expect(t('dashboard.shares.statusExpired')).toBe('Expired')
    expect(t('dashboard.shares.statusRevoked')).toBe('Revoked')
    expect(t('dashboard.shares.statusRejected')).toBe('Rejected')
  })

  it('keeps the request card', () => {
    expect(t('dashboard.shares.loadingUser')).toBe('Loading user info...')
    expect(t('dashboard.shares.revoke')).toBe('Revoke')
    expect(t('dashboard.shares.accept')).toBe('Accept')
    expect(t('dashboard.shares.reject')).toBe('Reject')
    // Stands in for a sender with no display name.
    expect(t('dashboard.shares.userFallbackName')).toBe('User')
  })

  it('keeps the accept modal', () => {
    expect(t('dashboard.shares.acceptTitle', { sender: 'Sarah' })).toBe(
      'Sarah wants to share with you'
    )
    expect(t('dashboard.shares.cannotAcceptTitle')).toBe('Cannot Accept Share')
    expect(t('dashboard.shares.noOverlaysError')).toBe('Create an overlay first to accept shares')
    expect(t('dashboard.shares.loadOverlaysFailed')).toBe('Failed to load overlays')
    expect(t('dashboard.shares.close')).toBe('Close')
    expect(t('dashboard.shares.loadingOverlays')).toBe('Loading overlays...')
    // The required marker is rendered as its own element, so the label keeps a
    // placeholder for it rather than being split around the asterisk.
    expect(t('dashboard.shares.shareBackLabel', { required: '*' })).toBe(
      'Share back which overlay? *'
    )
    expect(t('dashboard.shares.requiredMarker')).toBe('*')
    expect(t('dashboard.shares.cancel')).toBe('Cancel')
    expect(t('dashboard.shares.acceptButton')).toBe('Accept')
    expect(t('dashboard.shares.accepting')).toBe('Accepting...')
  })

  it('keeps the three expiry options', () => {
    expect(t('dashboard.shares.expiryLegend')).toBe('When should the share expire?')
    expect(t('dashboard.shares.expiryThisStream')).toBe('This stream')
    expect(t('dashboard.shares.expiryThisStreamHint')).toBe('Expires when your stream ends')
    // Kick has no stream-lifecycle detection, so 'this stream' would never
    // expire there.
    expect(t('dashboard.shares.expiryKickUnavailable')).toBe(
      '(not available for Kick — stream detection not yet supported)'
    )
    expect(t('dashboard.shares.expiryCustom')).toBe('Custom duration')
    expect(t('dashboard.shares.expiryCustomPlaceholder')).toBe('hours')
    expect(t('dashboard.shares.expiryCustomLabel')).toBe('Custom duration in hours')
    expect(t('dashboard.shares.expiryCustomHint')).toBe('hours (1-168)')
    expect(t('dashboard.shares.expiryCustomError')).toBe('Must be between 1 and 168 hours')
    expect(t('dashboard.shares.expiryUnlimited')).toBe('Unlimited')
    expect(t('dashboard.shares.expiryUnlimitedHint')).toBe('Never expires')
  })

  it('keeps the add-source modal', () => {
    // The render site spelled the apostrophes &apos;, which is U+0027.
    expect(t('dashboard.shares.addSourceTitle', { sender: 'Sarah' })).toBe(
      "Add Sarah's overlay to one of yours?"
    )
    expect(t('dashboard.shares.addSourcePreview', { sender: 'Sarah' })).toBe(
      "Sarah's overlay (shared chat)"
    )
    expect(t('dashboard.shares.addSourceSelectLabel')).toBe('Add to which overlay?')
    expect(t('dashboard.shares.addSourceSkip')).toBe('Skip')
    expect(t('dashboard.shares.addSourceAdd')).toBe('Add')
    expect(t('dashboard.shares.addSourceAdding')).toBe('Adding...')
  })

  it('keeps the revocation confirmation', () => {
    expect(t('dashboard.shares.revokeTitle', { partner: 'Sarah' })).toBe('Revoke share with Sarah?')
    expect(t('dashboard.shares.revokeBody')).toBe('This will stop message delivery immediately.')
    expect(t('dashboard.shares.revokeCancel')).toBe('Cancel')
    expect(t('dashboard.shares.revokeConfirm')).toBe('Revoke')
    expect(t('dashboard.shares.revoking')).toBe('Revoking...')
  })
})

describe('share request toast copy', () => {
  it('keeps the load and notification failures', () => {
    expect(t('dashboard.shares.loadRequestsFailed')).toBe('Failed to load share requests')
    // One key for both notification toggles: the two call sites were
    // byte-identical, and a language cannot need two spellings of one sentence.
    expect(t('dashboard.shares.notificationUpdateFailed')).toBe(
      'Failed to update notification status'
    )
  })

  it('keeps the accept toasts', () => {
    expect(t('dashboard.shares.acceptedToast', { sender: 'Alice' })).toBe(
      'Share accepted from Alice!'
    )
    // The fallback when the sender has no display name. Lowercase in the
    // original because it sits mid-sentence, unlike shares.userFallbackName.
    expect(t('dashboard.shares.acceptedToastUnknownSender')).toBe('user')
    expect(t('dashboard.shares.circularTitle')).toBe('Cannot accept')
    expect(t('dashboard.shares.circularBody')).toBe('This would create a circular share dependency')
  })

  it('keeps the add-source and revoke toasts', () => {
    expect(t('dashboard.shares.addSourceToast', { sender: 'Alice' })).toBe("Added Alice's overlay!")
    expect(t('dashboard.shares.revokedToast')).toBe('Share revoked')
    expect(t('dashboard.shares.revokeFailedToast')).toBe('Failed to revoke share')
  })
})
