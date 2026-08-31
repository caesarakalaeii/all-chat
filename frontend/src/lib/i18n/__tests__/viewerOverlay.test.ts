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
 * Copy lock for the viewer-facing overlay surfaces and the overlay chrome. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 *
 * Overlay chat messages and event bodies are viewer-authored content and are
 * never translated, so nothing here pins them.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('engagement controls copy', () => {
  it('keeps the two column headings', () => {
    expect(t('viewerOverlay.engagement.pollHeading')).toBe('Poll')
    expect(t('viewerOverlay.engagement.predictionHeading')).toBe('Prediction')
  })

  it('keeps the origin badge for rounds mirrored from Twitch', () => {
    expect(t('viewerOverlay.engagement.twitchSourceBadge')).toBe('Twitch')
  })

  it('keeps the label list editor chrome', () => {
    // The numbered placeholder/aria-label pair the create forms share; the
    // per-list noun arrives as a parameter.
    expect(t('viewerOverlay.engagement.labelListEntry', { noun: 'Option', index: '1' })).toBe(
      'Option 1'
    )
    expect(t('viewerOverlay.engagement.labelListRemove')).toBe('Remove')
    expect(t('viewerOverlay.engagement.labelListAdd', { noun: 'outcome' })).toBe('Add outcome')
  })

  it('keeps the poll create form copy', () => {
    expect(t('viewerOverlay.engagement.pollQuestionPlaceholder')).toBe('Question')
    expect(t('viewerOverlay.engagement.pollQuestionLabel')).toBe('Poll question')
    expect(t('viewerOverlay.engagement.pollOptionNoun')).toBe('Option')
    expect(t('viewerOverlay.engagement.pollAllowChange')).toBe('Allow vote changes')
    expect(t('viewerOverlay.engagement.pollAutoCloseAfter')).toBe('Auto-close after')
    expect(t('viewerOverlay.engagement.secondsSuffix')).toBe('s')
    expect(t('viewerOverlay.engagement.pollStart')).toBe('Start poll')
  })

  it('keeps the poll live view copy', () => {
    expect(t('viewerOverlay.engagement.pollVotes', { total: '12' })).toBe('12 votes')
    expect(t('viewerOverlay.engagement.pollAutoCloses', { time: '19:04' })).toBe(
      ' · auto-closes 19:04'
    )
    expect(t('viewerOverlay.engagement.pollNew')).toBe('New poll')
    expect(t('viewerOverlay.engagement.pollClose')).toBe('Close poll')
    expect(t('viewerOverlay.engagement.pollMirroredNote')).toBe(
      'Mirrored from Twitch — viewers vote in the Twitch UI/chat'
    )
  })

  it('keeps the where-viewers-vote hint whole, with the link and the two commands', () => {
    // One sentence with three element holes rather than concatenated fragments:
    // word order is the first thing a second language changes.
    expect(t('viewerOverlay.engagement.pollParticipateHint')).toBe(
      'Viewers vote on the {link} or from chat ({voteCommand} or just {shortCommand})'
    )
    expect(t('viewerOverlay.engagement.participateLink')).toBe('participate page')
  })

  it('keeps the prediction create form copy', () => {
    expect(t('viewerOverlay.engagement.predictionTitlePlaceholder')).toBe(
      'Title (e.g. Will we win this round?)'
    )
    expect(t('viewerOverlay.engagement.predictionTitleLabel')).toBe('Prediction title')
    expect(t('viewerOverlay.engagement.predictionOutcomeNoun')).toBe('Outcome')
    expect(t('viewerOverlay.engagement.predictionAutoLockAfter')).toBe('Auto-lock wagers after')
    expect(t('viewerOverlay.engagement.predictionStart')).toBe('Start prediction')
    expect(t('viewerOverlay.engagement.predictionParticipateHint')).toBe(
      'Viewers wager on the {link} (they can see their balance) — or from chat: {predictCommand}'
    )
  })

  it('keeps the prediction live view copy', () => {
    expect(t('viewerOverlay.engagement.predictionPointsWagered', { total: '900' })).toBe(
      '900 points wagered'
    )
    expect(t('viewerOverlay.engagement.predictionAutoLocks', { time: '19:04' })).toBe(
      ' · auto-locks 19:04'
    )
    expect(
      t('viewerOverlay.engagement.predictionOutcomeTally', { points: '900', entrants: '3' })
    ).toBe('900 pts · 3 entrants')
    expect(t('viewerOverlay.engagement.winningOutcome')).toBe('Winning outcome')
    expect(t('viewerOverlay.engagement.winningOutcomeChoice', { label: 'Yes' })).toBe(
      'Winning outcome: Yes'
    )
    expect(t('viewerOverlay.engagement.predictionLock')).toBe('Lock wagers')
    expect(t('viewerOverlay.engagement.predictionNew')).toBe('New prediction')
    expect(t('viewerOverlay.engagement.predictionLockedNote')).toBe(
      'Pick the winning outcome, then pay out. Payouts are final.'
    )
    expect(t('viewerOverlay.engagement.predictionMirroredNote')).toBe(
      'Mirrored from Twitch — runs on Twitch channel points'
    )
  })

  it('keeps the payout button through its three states', () => {
    expect(t('viewerOverlay.engagement.predictionResolve')).toBe('Resolve')
    expect(t('viewerOverlay.engagement.predictionPayOut', { label: 'Yes' })).toBe('Pay out "Yes"')
    expect(t('viewerOverlay.engagement.predictionPayOutConfirm', { label: 'Yes' })).toBe(
      'Pay out "Yes" — final?'
    )
    expect(t('viewerOverlay.engagement.predictionResolveDisabledTitle')).toBe(
      'Select the winning outcome first'
    )
  })

  it('keeps both steps of the cancel-and-refund confirmation', () => {
    expect(t('viewerOverlay.engagement.predictionCancel')).toBe('Cancel & refund')
    expect(t('viewerOverlay.engagement.predictionCancelTitle')).toBe('Cancel and refund all wagers')
    expect(t('viewerOverlay.engagement.predictionCancelConfirm')).toBe('Really refund all wagers?')
  })

  it('keeps the Twitch mirroring opt-in note and its button', () => {
    expect(t('viewerOverlay.engagement.mirrorNote')).toBe(
      'Mirror native Twitch polls & predictions onto your overlays (read-only). Opt-in; takes effect after the next channel sync (a stream restart or re-adding the source).'
    )
    expect(t('viewerOverlay.engagement.mirrorEnable')).toBe('Enable Twitch mirroring')
  })
})
