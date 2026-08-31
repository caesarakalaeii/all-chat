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
 * Viewer-facing overlay chrome: labels, buttons, empty states, aria labels.
 *
 * Overlay chat messages and event bodies are viewer-authored content and are
 * never translated, so nothing here describes them.
 */

export const viewerOverlay = {
  engagement: {
    pollHeading: 'Poll',
    predictionHeading: 'Prediction',
    twitchSourceBadge: 'Twitch',

    labelListEntry: '{noun} {index}',
    labelListRemove: 'Remove',
    labelListAdd: 'Add {noun}',

    pollQuestionPlaceholder: 'Question',
    pollQuestionLabel: 'Poll question',
    pollOptionNoun: 'Option',
    pollAllowChange: 'Allow vote changes',
    pollAutoCloseAfter: 'Auto-close after',
    secondsSuffix: 's',
    pollStart: 'Start poll',

    pollVotes: '{total} votes',
    // Appended to pollVotes, so it opens with its own separator.
    pollAutoCloses: ' · auto-closes {time}',
    pollNew: 'New poll',
    pollClose: 'Close poll',
    pollMirroredNote: 'Mirrored from Twitch — viewers vote in the Twitch UI/chat',
    pollParticipateHint:
      'Viewers vote on the {link} or from chat ({voteCommand} or just {shortCommand})',
    participateLink: 'participate page',

    predictionTitlePlaceholder: 'Title (e.g. Will we win this round?)',
    predictionTitleLabel: 'Prediction title',
    predictionOutcomeNoun: 'Outcome',
    predictionAutoLockAfter: 'Auto-lock wagers after',
    predictionStart: 'Start prediction',
    predictionParticipateHint:
      'Viewers wager on the {link} (they can see their balance) — or from chat: {predictCommand}',

    predictionPointsWagered: '{total} points wagered',
    predictionAutoLocks: ' · auto-locks {time}',
    predictionOutcomeTally: '{points} pts · {entrants} entrants',
    winningOutcome: 'Winning outcome',
    winningOutcomeChoice: 'Winning outcome: {label}',
    predictionLock: 'Lock wagers',
    predictionNew: 'New prediction',
    predictionLockedNote: 'Pick the winning outcome, then pay out. Payouts are final.',
    predictionMirroredNote: 'Mirrored from Twitch — runs on Twitch channel points',

    predictionResolve: 'Resolve',
    predictionPayOut: 'Pay out "{label}"',
    predictionPayOutConfirm: 'Pay out "{label}" — final?',
    predictionResolveDisabledTitle: 'Select the winning outcome first',

    predictionCancel: 'Cancel & refund',
    predictionCancelTitle: 'Cancel and refund all wagers',
    predictionCancelConfirm: 'Really refund all wagers?',

    mirrorNote:
      'Mirror native Twitch polls & predictions onto your overlays (read-only). Opt-in; takes effect after the next channel sync (a stream restart or re-adding the source).',
    mirrorEnable: 'Enable Twitch mirroring',
  },

  participate: {
    loading: 'Loading…',
    loginHeading: 'Join the fun',
    loginBlurb: 'Log in with your platform account to vote and wager.',
    loginWith: 'Continue with {platform}',
    noWebLoginNote:
      "Watching on TikTok or Discord? Take part with the on-screen chat commands — web login isn't available for those platforms yet.",

    heading: 'Participate',
    balanceLabel: 'Balance: {balance} {pointsName}',
    balance: '{balance} {pointsName}',
    // Fallback for a streamer who has not renamed their points.
    defaultPointsName: 'Points',

    settledBanner:
      'Your prediction on “{outcome}” settled — you wagered {amount} {pointsName}. Check your balance above.',

    pollNativeNote: 'This poll runs on Twitch — vote in Twitch chat or the Twitch app.',
    pollVoteNativeTitle: 'Vote in Twitch chat',
    working: 'Working…',
    yourVote: '(your vote)',
    pollOptionTally: '{pct}% ({votes})',

    predictionLocked: 'locked',
    predictionNativeNote: 'This prediction runs on Twitch channel points.',
    youHave: 'You have {balance} {pointsName}',
    maxWager: 'Max',
    noPointsYet:
      'You have no {pointsName} yet. Earn them by keeping this page open and by supporting the stream (subs, bits, donations, gifts), then come back to wager.',
    wagerAmountLabel: 'Amount to wager in {pointsName}',
    wagerAmountPlaceholder: 'Amount to wager ({pointsName})',
    // Appended to the outcome label, so it opens with its own separator.
    yourWager: ' · your wager: {amount}',
    outcomeTally: '{points} · {pct}%',
    alreadyWagered: "You've locked in your wager for this round.",
    nothingActive: 'No active poll or prediction right now. Hang tight!',

    wagerNativeTitle: 'Runs on Twitch channel points',
    wagerAlreadyTitle: 'You already wagered this round',
    wagerClosedTitle: 'Betting is closed',

    lockedAnnouncement: 'Prediction locked — betting is closed.',

    // Server reasons for a rejected wager; see repository/predictions.go.
    rejectNotFound: 'This prediction is no longer available.',
    rejectNotActive: 'Betting is closed for this round.',
    rejectBadOutcome: 'That outcome is not valid.',
    rejectAlreadyWagered: 'You already placed a wager this round.',
    rejectInsufficient: 'Not enough {pointsName}. You have {balance}.',
    rejectNative: 'This prediction runs on Twitch channel points.',
    // Worded differently from rejectInsufficient on purpose: this one fires
    // locally, before the request, and both reach viewers today.
    insufficientLocal: 'Not enough {pointsName} — you have {balance}.',

    loginFailed: 'Could not start login. Please try again.',
    voteFailed: 'Vote failed',
    wagerFailed: 'Wager failed',
    wagerNeedsAmount: 'Enter a positive amount to wager.',
  },
} as const
