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
  // The platform names this panel renders moved to common.platforms.* once the
  // overlay editor's bubble colour picker became their second reader.
  // __tests__/common.test.ts pins them now.
  it('keeps the action labels', () => {
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
    // The emphasised words are a placeholder so the sentence stays whole. The
    // rendered text must still read exactly as it did before the migration.
    expect(
      t('moderation.roster.explainer', { emphasis: t('moderation.roster.explainerEmphasis') })
    ).toBe(
      'Moderators act with their own platform accounts, so Twitch, YouTube and Kick check their moderator role on every action. Removing someone here takes effect immediately.'
    )
    expect(t('moderation.roster.explainerEmphasis')).toBe('their own')
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
    expect(
      t('moderation.invite.sendLink', { emphasis: t('moderation.invite.sendLinkEmphasis') })
    ).toBe(
      "Send this link to the person you want to moderate. It works once, expires in 7 days, and won't be shown again — if it gets lost, create a new invite."
    )
    expect(t('moderation.invite.sendLinkEmphasis')).toBe("won't be shown again")
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

describe('invite acceptance page copy', () => {
  it('keeps the page chrome', () => {
    expect(t('moderation.accept.heading')).toBe('Moderation invite')
    expect(t('moderation.accept.loadingInvite')).toBe('Loading invite')
    expect(t('moderation.accept.goToChannels')).toBe('Go to your channels')
    expect(t('moderation.accept.accept')).toBe('Accept and start moderating')
    expect(t('moderation.accept.accepting')).toBe('Accepting…')
    expect(t('moderation.accept.notNow')).toBe('Not now')
  })

  it('keeps the four action labels, which read differently here than in the roster', () => {
    // moderation.actions.* is the roster's terse wording ("Timeout"). This page
    // spells each action out for someone deciding whether to accept, so the
    // copy is genuinely different and gets its own keys.
    expect(t('moderation.accept.actionDelete')).toBe('Delete messages')
    expect(t('moderation.accept.actionTimeout')).toBe('Time viewers out')
    expect(t('moderation.accept.actionBan')).toBe('Ban viewers')
    expect(t('moderation.accept.actionUnban')).toBe('Lift bans and timeouts')
  })

  it('keeps the sign-in prompt', () => {
    expect(t('moderation.accept.signInHeading')).toBe('Sign in to accept this invite')
    expect(t('moderation.accept.signInBody')).toBe(
      'Moderating is tied to an All-Chat account, so we need to know which one to hand this to. Sign in, then open the invite link again — it stays valid.'
    )
    expect(t('moderation.accept.signIn')).toBe('Sign in')
  })

  it('keeps the invite summary', () => {
    // The owner name is emphasised, so the sentence stays whole with a
    // placeholder rather than being split around the <span>.
    expect(t('moderation.accept.askingToHelp', { owner: 'Sarah' })).toBe(
      'Sarah is asking you to help moderate'
    )
    // The render site spelled the quotes &ldquo;/&rdquo;.
    expect(t('moderation.accept.addressedTo', { label: 'my Twitch mod' })).toBe(
      'They addressed this invite to “my Twitch mod”.'
    )
    expect(t('moderation.accept.actionsHeading')).toBe('What you would be able to do')
    expect(t('moderation.accept.platformsHeading')).toBe('On these platforms')
    expect(t('moderation.accept.noPlatforms', { owner: 'Sarah' })).toBe(
      'None yet — Sarah still has to turn a platform on.'
    )
    expect(t('moderation.accept.ownAccountNote', { owner: 'Sarah' })).toBe(
      'You will act with your own platform account, so each platform still checks that Sarah made you a moderator there. Nothing is asked of you now — you connect a platform the first time you moderate on it.'
    )
    // expected_platform is optional server-side, so the two reachable renders
    // are two whole sentences. One key with an empty placeholder would leave a
    // translator with a sentence that has a hole in it in one of the two cases.
    expect(t('moderation.accept.expectedAccount', { account: 'sarah' })).toBe(
      'This invite is meant for sarah.'
    )
    expect(
      t('moderation.accept.expectedAccountOnPlatform', { platform: 'Twitch', account: 'sarah' })
    ).toBe('This invite is meant for Twitch sarah.')
  })

  it('keeps every invite failure message', () => {
    expect(t('moderation.accept.errorMissingToken')).toBe(
      'This link is missing its invite code. Ask the streamer to send it again.'
    )
    // "Not found" deliberately covers unknown, redeemed and revoked alike,
    // because the server keeps the three indistinguishable.
    expect(t('moderation.accept.errorNotFound')).toBe(
      'This invite is not valid any more — it may already have been used, or the streamer may have withdrawn it. Ask them for a new one.'
    )
    expect(t('moderation.accept.errorExpired')).toBe(
      'This invite has expired. Ask the streamer for a new one.'
    )
    expect(t('moderation.accept.errorAlreadyModerator')).toBe(
      'You already moderate this channel. It is on your channels page.'
    )
    expect(t('moderation.accept.errorOwnerCannotAccept')).toBe(
      'This is your own overlay — you already have full moderation on it.'
    )
    // The platform and the account are each optional server-side, so the four
    // reachable combinations are four whole sentences rather than one stem with
    // fragments appended.
    expect(t('moderation.accept.errorBoundToOther')).toBe(
      'This invite is for a specific account. Sign in as that account, or ask the streamer to send a new invite for this one.'
    )
    expect(t('moderation.accept.errorBoundToOtherAccount', { account: 'sarah' })).toBe(
      'This invite is for a specific account (sarah). Sign in as that account, or ask the streamer to send a new invite for this one.'
    )
    expect(t('moderation.accept.errorBoundToOtherPlatform', { platform: 'Twitch' })).toBe(
      'This invite is for a specific Twitch account. Sign in as that account, or ask the streamer to send a new invite for this one.'
    )
    expect(
      t('moderation.accept.errorBoundToOtherBoth', { platform: 'Twitch', account: 'sarah' })
    ).toBe(
      'This invite is for a specific Twitch account (sarah). Sign in as that account, or ask the streamer to send a new invite for this one.'
    )
    expect(t('moderation.accept.errorUnknown')).toBe(
      'Could not open this invite. Check the link and try again.'
    )
  })
})

describe('channels you moderate page copy', () => {
  it('keeps the page chrome', () => {
    expect(t('moderation.channels.heading')).toBe('Channels you moderate')
    expect(t('moderation.channels.subheading')).toBe(
      'Overlays other streamers have handed you. You act with your own platform account, so each platform still checks that you are one of their moderators.'
    )
    expect(t('moderation.channels.loading')).toBe('Loading channels')
    expect(t('moderation.channels.loadFailed')).toBe('Could not load your channels.')
    expect(t('moderation.channels.tryAgain')).toBe('Try again')
  })

  it('keeps the empty state', () => {
    expect(t('moderation.channels.emptyHeading')).toBe('No channels yet')
    expect(t('moderation.channels.emptyBody')).toBe(
      'When a streamer invites you to moderate their overlay, they send you a private link. Open it while signed in to this account and their channel appears here.'
    )
  })

  it('keeps the delegation card', () => {
    expect(t('moderation.channels.forOwner', { owner: 'Sarah' })).toBe('for Sarah')
    expect(t('moderation.channels.noPlatforms', { owner: 'Sarah' })).toBe(
      'No platforms turned on yet — ask Sarah to enable one.'
    )
    expect(t('moderation.channels.suspendedNote', { owner: 'Sarah' })).toBe(
      'Paused after 90 days without any actions. Ask Sarah to turn it back on.'
    )
    // The streamer's plan, never the moderator's: there is nothing here a
    // volunteer could buy, so the copy states the cause and stops.
    // The render site spelled the apostrophe &apos;, which is U+0027.
    expect(t('moderation.channels.unavailableNote', { owner: 'Sarah' })).toBe(
      "Sarah's plan does not include moderation right now, so actions are unavailable until they renew it."
    )
    expect(t('moderation.channels.openMonitor')).toBe('Open chat monitor')
  })

  it('keeps the Discord link prompt', () => {
    expect(t('moderation.channels.discordPromptBody')).toBe(
      'Link your Discord account to moderate Discord. All-Chat checks your own server permissions before it acts, so it needs to know which Discord account is yours.'
    )
    expect(t('moderation.channels.linkDiscord')).toBe('Link Discord')
  })

  it('keeps the four mod-consent redirect notices', () => {
    // already_linked is the one failure worth its own words: no amount of
    // retrying changes it.
    expect(t('moderation.channels.noticeDiscordAlreadyLinked')).toBe(
      'That Discord account is already linked to another All-Chat account. Link a different one, or unlink it from the other account first.'
    )
    expect(t('moderation.channels.noticeConnectFailed')).toBe(
      'That connection did not complete. Open a channel below and try again from there.'
    )
    expect(t('moderation.channels.noticeDiscordLinked')).toBe(
      'Discord account linked. All-Chat can now check your own server permissions when you moderate Discord.'
    )
    expect(t('moderation.channels.noticeConnected', { platform: 'Twitch' })).toBe(
      'Twitch connected. It now covers every channel that delegated Twitch to you.'
    )
  })
})
