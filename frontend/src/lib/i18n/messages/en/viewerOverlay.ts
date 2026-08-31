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
} as const
