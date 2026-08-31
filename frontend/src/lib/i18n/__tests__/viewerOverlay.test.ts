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

describe('participate page copy', () => {
  it('keeps the loading and login gate', () => {
    expect(t('viewerOverlay.participate.loading')).toBe('Loading…')
    expect(t('viewerOverlay.participate.loginHeading')).toBe('Join the fun')
    expect(t('viewerOverlay.participate.loginBlurb')).toBe(
      'Log in with your platform account to vote and wager.'
    )
    expect(t('viewerOverlay.participate.loginWith', { platform: 'Twitch' })).toBe(
      'Continue with Twitch'
    )
    // N2: TikTok and Discord have no web login, so those viewers are pointed at
    // the chat commands instead.
    expect(t('viewerOverlay.participate.noWebLoginNote')).toBe(
      "Watching on TikTok or Discord? Take part with the on-screen chat commands — web login isn't available for those platforms yet."
    )
  })

  it('keeps the header and its balance readout', () => {
    expect(t('viewerOverlay.participate.heading')).toBe('Participate')
    expect(
      t('viewerOverlay.participate.balanceLabel', { balance: '1,200', pointsName: 'Points' })
    ).toBe('Balance: 1,200 Points')
    expect(t('viewerOverlay.participate.balance', { balance: '1,200', pointsName: 'Points' })).toBe(
      '1,200 Points'
    )
    // The name a streamer gives their points is configurable; this is the
    // fallback when they have not set one.
    expect(t('viewerOverlay.participate.defaultPointsName')).toBe('Points')
  })

  it('keeps the settled-prediction banner as one sentence', () => {
    expect(
      t('viewerOverlay.participate.settledBanner', {
        outcome: 'Yes',
        amount: '500',
        pointsName: 'Points',
      })
    ).toBe('Your prediction on “Yes” settled — you wagered 500 Points. Check your balance above.')
  })

  it('keeps the poll section copy', () => {
    expect(t('viewerOverlay.participate.pollNativeNote')).toBe(
      'This poll runs on Twitch — vote in Twitch chat or the Twitch app.'
    )
    expect(t('viewerOverlay.participate.pollVoteNativeTitle')).toBe('Vote in Twitch chat')
    expect(t('viewerOverlay.participate.working')).toBe('Working…')
    expect(t('viewerOverlay.participate.yourVote')).toBe('(your vote)')
    expect(t('viewerOverlay.participate.pollOptionTally', { pct: '40', votes: '12' })).toBe(
      '40% (12)'
    )
  })

  it('keeps the prediction section copy', () => {
    expect(t('viewerOverlay.participate.predictionLocked')).toBe('locked')
    expect(t('viewerOverlay.participate.predictionNativeNote')).toBe(
      'This prediction runs on Twitch channel points.'
    )
    expect(t('viewerOverlay.participate.youHave', { balance: '1,200', pointsName: 'Points' })).toBe(
      'You have 1,200 Points'
    )
    expect(t('viewerOverlay.participate.maxWager')).toBe('Max')
    expect(t('viewerOverlay.participate.noPointsYet', { pointsName: 'Points' })).toBe(
      'You have no Points yet. Earn them by keeping this page open and by supporting the stream (subs, bits, donations, gifts), then come back to wager.'
    )
    expect(t('viewerOverlay.participate.wagerAmountLabel', { pointsName: 'Points' })).toBe(
      'Amount to wager in Points'
    )
    expect(t('viewerOverlay.participate.wagerAmountPlaceholder', { pointsName: 'Points' })).toBe(
      'Amount to wager (Points)'
    )
    expect(t('viewerOverlay.participate.yourWager', { amount: '500' })).toBe(' · your wager: 500')
    expect(t('viewerOverlay.participate.outcomeTally', { points: '900', pct: '60' })).toBe(
      '900 · 60%'
    )
    expect(t('viewerOverlay.participate.alreadyWagered')).toBe(
      "You've locked in your wager for this round."
    )
    expect(t('viewerOverlay.participate.nothingActive')).toBe(
      'No active poll or prediction right now. Hang tight!'
    )
  })

  it('keeps the four disabled-outcome button titles distinct', () => {
    expect(t('viewerOverlay.participate.wagerNativeTitle')).toBe('Runs on Twitch channel points')
    expect(t('viewerOverlay.participate.wagerAlreadyTitle')).toBe('You already wagered this round')
    expect(t('viewerOverlay.participate.wagerClosedTitle')).toBe('Betting is closed')
  })

  it('announces the prediction locking', () => {
    expect(t('viewerOverlay.participate.lockedAnnouncement')).toBe(
      'Prediction locked — betting is closed.'
    )
  })

  it('keeps every wager rejection reason, including both insufficient-balance wordings', () => {
    // The server reason and the local pre-empt are worded differently and both
    // reach viewers; collapsing them would change rendered output.
    expect(t('viewerOverlay.participate.rejectNotFound')).toBe(
      'This prediction is no longer available.'
    )
    expect(t('viewerOverlay.participate.rejectNotActive')).toBe('Betting is closed for this round.')
    expect(t('viewerOverlay.participate.rejectBadOutcome')).toBe('That outcome is not valid.')
    expect(t('viewerOverlay.participate.rejectAlreadyWagered')).toBe(
      'You already placed a wager this round.'
    )
    expect(
      t('viewerOverlay.participate.rejectInsufficient', {
        pointsName: 'Points',
        balance: '10',
      })
    ).toBe('Not enough Points. You have 10.')
    expect(
      t('viewerOverlay.participate.insufficientLocal', { pointsName: 'Points', balance: '10' })
    ).toBe('Not enough Points — you have 10.')
    expect(t('viewerOverlay.participate.rejectNative')).toBe(
      'This prediction runs on Twitch channel points.'
    )
  })

  it('keeps the remaining failure notices', () => {
    expect(t('viewerOverlay.participate.loginFailed')).toBe(
      'Could not start login. Please try again.'
    )
    expect(t('viewerOverlay.participate.voteFailed')).toBe('Vote failed')
    expect(t('viewerOverlay.participate.wagerFailed')).toBe('Wager failed')
    expect(t('viewerOverlay.participate.wagerNeedsAmount')).toBe(
      'Enter a positive amount to wager.'
    )
  })
})

describe('poll widget copy', () => {
  it('keeps the closed-round badge and footer', () => {
    expect(t('viewerOverlay.pollWidget.finalBadge')).toBe('Final')
    expect(t('viewerOverlay.pollWidget.finalResults')).toBe('Final results')
  })

  it('keeps the winner and tie pills distinct', () => {
    // P3-12: a non-unique top count labels every tied option, rather than
    // arbitrarily crowning the first.
    expect(t('viewerOverlay.pollWidget.winnerPill')).toBe('Winner')
    expect(t('viewerOverlay.pollWidget.tiePill')).toBe('Tie')
  })

  it('keeps the tally and the countdown', () => {
    expect(t('viewerOverlay.pollWidget.optionTally', { pct: '40', votes: '12' })).toBe('40% (12)')
    expect(t('viewerOverlay.pollWidget.remaining', { clock: '1:05' })).toBe('1:05 remaining')
  })
})

describe('prediction widget copy', () => {
  it('keeps the three state badges', () => {
    // The padlock and trophy are inseparable from these badges as rendered —
    // there is no sibling word for either, so they are part of the string.
    expect(t('viewerOverlay.predictionWidget.stateLocked')).toBe('🔒 LOCKED')
    expect(t('viewerOverlay.predictionWidget.stateResolved')).toBe('🏆 RESOLVED')
    expect(t('viewerOverlay.predictionWidget.stateOpen')).toBe('OPEN')
  })

  it('keeps the winner pill, the tally and the pool footer', () => {
    expect(t('viewerOverlay.predictionWidget.winnerPill')).toBe('Winner')
    expect(t('viewerOverlay.predictionWidget.outcomeTally', { points: '900', pct: '60' })).toBe(
      '900 pts · 60%'
    )
    expect(t('viewerOverlay.predictionWidget.pool', { points: '1,500', players: '4' })).toBe(
      '1,500 pts wagered · 4 players'
    )
  })
})

describe('credit roll copy', () => {
  it('keeps the loading, error and empty states', () => {
    expect(t('viewerOverlay.credits.loading')).toBe('Loading Credits...')
    expect(t('viewerOverlay.credits.errorHeading')).toBe('Unable to Load Credit Roll')
    expect(t('viewerOverlay.credits.errorHint')).toBe(
      'Make sure you have an active streaming session'
    )
    expect(t('viewerOverlay.credits.empty')).toBe('No credit roll data available')
  })

  it('keeps the load failure message', () => {
    expect(t('viewerOverlay.credits.loadFailed')).toBe('Failed to load credit roll')
  })

  it('keeps the roll header and its session facts', () => {
    expect(t('viewerOverlay.credits.heading')).toBe('🎬 Stream Credits')
    expect(t('viewerOverlay.credits.subheading')).toBe(
      'Thank you to everyone who supported the stream!'
    )
    expect(t('viewerOverlay.credits.session', { date: '31/08/2026' })).toBe('Session: 31/08/2026')
    expect(t('viewerOverlay.credits.duration', { duration: '2 hours' })).toBe('Duration: 2 hours')
  })

  it('keeps every branch of the session duration, including both plural forms', () => {
    // English-only pluralisation, so each form is its own key rather than a
    // suffix stitched on at the render site.
    expect(t('viewerOverlay.credits.durationHourOne', { hours: '1' })).toBe('1 hour')
    expect(t('viewerOverlay.credits.durationHourMany', { hours: '2' })).toBe('2 hours')
    expect(t('viewerOverlay.credits.durationMinuteOne', { minutes: '1' })).toBe('1 minute')
    expect(t('viewerOverlay.credits.durationMinuteMany', { minutes: '5' })).toBe('5 minutes')
    expect(
      t('viewerOverlay.credits.durationHoursAndMinutes', { hours: '2 hours', minutes: '5 minutes' })
    ).toBe('2 hours 5 minutes')
    expect(t('viewerOverlay.credits.durationJustStarted')).toBe('just started')
  })

  it('keeps all seven leaderboard titles', () => {
    expect(t('viewerOverlay.credits.topSubscribers')).toBe('Top Subscribers')
    expect(t('viewerOverlay.credits.topGifters')).toBe('Top Gifters')
    expect(t('viewerOverlay.credits.topCheerers')).toBe('Top Cheerers')
    expect(t('viewerOverlay.credits.topChannelPoints')).toBe('Top Channel Points')
    expect(t('viewerOverlay.credits.topRaiders')).toBe('Top Raiders')
    expect(t('viewerOverlay.credits.topSuperChats')).toBe('Top Super Chats')
    expect(t('viewerOverlay.credits.newFollowers')).toBe('New Followers')
  })

  it('keeps the now-playing clip block', () => {
    expect(t('viewerOverlay.credits.nowPlaying')).toBe('Now Playing')
    expect(t('viewerOverlay.credits.clipViews', { views: '120' })).toBe('120 views')
    expect(t('viewerOverlay.credits.clipCounter', { index: '2', total: '5' })).toBe('Clip 2/5')
  })

  it('keeps the sign-off', () => {
    expect(t('viewerOverlay.credits.thanks')).toBe('Thank you for watching! ❤️')
    expect(t('viewerOverlay.credits.seeYou')).toBe('See you next stream!')
  })
})

describe('OBS chat overlay copy', () => {
  it('keeps the shared-chat indicator', () => {
    expect(t('viewerOverlay.chatOverlay.sharedChat')).toBe('Shared Chat')
  })
})
