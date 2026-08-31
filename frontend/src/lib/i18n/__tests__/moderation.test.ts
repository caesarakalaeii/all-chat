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
 * Copy lock for the moderation surface. See __tests__/dashboard.test.ts for why
 * the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('moderators panel copy', () => {
  it('keeps the platform and action labels', () => {
    expect(t('moderation.platforms.twitch')).toBe('Twitch')
    expect(t('moderation.platforms.youtube')).toBe('YouTube')
    expect(t('moderation.platforms.kick')).toBe('Kick')
    expect(t('moderation.platforms.discord')).toBe('Discord')

    expect(t('moderation.actions.delete')).toBe('Delete messages')
    expect(t('moderation.actions.timeout')).toBe('Timeout')
    expect(t('moderation.actions.ban')).toBe('Ban')
    expect(t('moderation.actions.unban')).toBe('Unban')
  })

  it('keeps the readiness notes advisory, naming the platform', () => {
    // These are phrased as something to check, never as a verdict about the
    // person — `verification` is telemetry, not authorization.
    expect(t('moderation.readiness.notAModerator', { platform: 'Twitch' })).toBe(
      "Not a moderator on Twitch yet — add them in Twitch's own tools."
    )
    expect(t('moderation.readiness.needsConsent', { platform: 'Kick' })).toBe(
      'Waiting for them to connect their Kick account.'
    )
    expect(t('moderation.readiness.needsDiscordLink')).toBe('Waiting for them to link Discord.')
    expect(t('moderation.readiness.unavailable', { platform: 'YouTube' })).toBe(
      'Could not check YouTube just now.'
    )
  })

  it('keeps the grant status labels and the unnamed-invite fallback', () => {
    expect(t('moderation.status.pending')).toBe('Invite pending')
    expect(t('moderation.status.suspended')).toBe('Paused after 90 days idle')
    expect(t('moderation.status.revoked')).toBe('Removed')
    expect(t('moderation.status.active')).toBe('Active')
    expect(t('moderation.unnamedInvite')).toBe('Unnamed invite')
  })

  it('keeps the roster chrome and its cap copy', () => {
    expect(t('moderation.roster.loadFailed')).toBe("Could not load this overlay's moderators.")
    expect(t('moderation.roster.tryAgain')).toBe('Try again')
    expect(t('moderation.roster.loading')).toBe('Loading moderators…')
    expect(t('moderation.roster.explainer')).toBe(
      'Moderators act with their own platform accounts, so Twitch, YouTube and Kick check their moderator role on every action. Removing someone here takes effect immediately.'
    )
    expect(t('moderation.roster.inviteButton')).toBe('Invite a moderator')
    expect(t('moderation.roster.usage', { used: '2', cap: '5' })).toBe('2 of 5 used')
    expect(t('moderation.roster.removeAll')).toBe('Remove all')
    expect(t('moderation.roster.atCap', { cap: '5' })).toBe(
      'This overlay is at its limit of 5 moderators. Remove someone to invite another.'
    )
    expect(t('moderation.roster.empty')).toBe(
      'No one moderates this overlay yet. Invite someone and they will be able to delete messages and time viewers out from the monitor view, using their own platform accounts.'
    )
  })

  it('keeps the row action errors and notices', () => {
    expect(t('moderation.row.legChangeFailed', { platform: 'Twitch', name: 'Sarah' })).toBe(
      'Could not change Twitch for Sarah.'
    )
    expect(t('moderation.row.needsOneAction', { name: 'Sarah' })).toBe(
      'Sarah needs at least one action. Remove them instead.'
    )
    expect(t('moderation.row.updateFailed', { name: 'Sarah' })).toBe('Could not update Sarah.')
    expect(t('moderation.row.removed', { name: 'Sarah' })).toBe('Removed Sarah.')
    expect(t('moderation.row.removeFailed', { name: 'Sarah' })).toBe('Could not remove Sarah.')
    expect(t('moderation.row.removeAllFailed')).toBe('Could not remove everyone. Try again.')
    expect(t('moderation.row.removeButton')).toBe('Remove')
    expect(t('moderation.row.removeLabel', { name: 'Sarah' })).toBe('Remove Sarah')
    expect(t('moderation.row.forAccount', { account: 'sarah' })).toBe('for sarah')
    expect(t('moderation.row.platformToggleLabel', { platform: 'Kick', name: 'Sarah' })).toBe(
      'Kick moderation for Sarah'
    )
  })

  it('keeps both plural forms of the remove-all notice', () => {
    // The render site branched on `revoked === 1`; the two forms stay separate
    // keys because a second language pluralises on its own rules.
    expect(t('moderation.row.removedCountOne', { count: '1' })).toBe('Removed 1 moderator.')
    expect(t('moderation.row.removedCountMany', { count: '3' })).toBe('Removed 3 moderators.')
  })

  it('keeps the revoke confirmation dialogs', () => {
    expect(t('moderation.revoke.title', { name: 'Sarah' })).toBe('Remove Sarah?')
    expect(t('moderation.revoke.description')).toBe(
      "They lose access to this overlay's moderation on their next action. Their past actions stay in the log."
    )
    expect(t('moderation.revoke.cancel')).toBe('Cancel')
    expect(t('moderation.revoke.confirm')).toBe('Remove')
    expect(t('moderation.revokeAll.title')).toBe('Remove every moderator?')
    expect(t('moderation.revokeAll.description')).toBe(
      'This removes everyone on this overlay, including invites nobody has accepted yet. You can invite people again afterwards.'
    )
    expect(t('moderation.revokeAll.confirm')).toBe('Remove everyone')
  })

  it('keeps the invite dialog copy, including the shown-once warning', () => {
    expect(t('moderation.invite.title')).toBe('Invite a moderator')
    expect(t('moderation.invite.sendLink')).toBe(
      "Send this link to the person you want to moderate. It works once, expires in 7 days, and won't be shown again — if it gets lost, create a new invite."
    )
    expect(t('moderation.invite.copied')).toBe('Copied')
    expect(t('moderation.invite.copyLink')).toBe('Copy link')
    expect(t('moderation.invite.done')).toBe('Done')
    expect(t('moderation.invite.acceptExplainer')).toBe(
      'They accept with their own All-Chat account, then connect each platform the first time they moderate on it.'
    )
    expect(t('moderation.invite.labelPrompt')).toBe('Who is this for? (optional)')
    expect(t('moderation.invite.labelPlaceholder')).toBe('Sarah, my Twitch mod')
    expect(t('moderation.invite.actionsLegend')).toBe('What may they do?')
    expect(t('moderation.invite.platformsLegend')).toBe(
      'On which platforms? Off means they cannot act there.'
    )
    expect(t('moderation.invite.cancel')).toBe('Cancel')
    expect(t('moderation.invite.creating')).toBe('Creating…')
    expect(t('moderation.invite.create')).toBe('Create invite')
  })

  it('keeps the invite failure copy, premium gate included', () => {
    expect(t('moderation.invite.capReached')).toBe(
      'This overlay already has the maximum number of moderators.'
    )
    expect(t('moderation.invite.createFailed')).toBe('Could not create the invite. Try again.')
    expect(t('moderation.invite.boundToAccount', { account: 'sarah' })).toBe(
      'That invite belongs to sarah.'
    )
    expect(t('moderation.invite.copyFailed')).toBe(
      'Could not copy. Select the link and copy it manually.'
    )
    expect(t('moderation.invite.premiumGate')).toBe(
      'Delegating moderation is part of All-Chat premium. Your moderators never pay — only your own plan matters.'
    )
    expect(t('moderation.invite.upgradeLink')).toBe('Upgrade to invite moderators')
  })
})
