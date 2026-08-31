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
 * The moderation queue and its actions.
 */

export const moderation = {
  platforms: {
    twitch: 'Twitch',
    youtube: 'YouTube',
    kick: 'Kick',
    discord: 'Discord',
  },
  actions: {
    delete: 'Delete messages',
    timeout: 'Timeout',
    ban: 'Ban',
    unban: 'Unban',
  },
  // Advisory, never a verdict about the person: `verification` is the last
  // platform answer we happened to observe, not an authorization decision, so
  // every string is something to check or do.
  readiness: {
    notAModerator: "Not a moderator on {platform} yet — add them in {platform}'s own tools.",
    needsConsent: 'Waiting for them to connect their {platform} account.',
    needsDiscordLink: 'Waiting for them to link Discord.',
    unavailable: 'Could not check {platform} just now.',
  },
  status: {
    pending: 'Invite pending',
    suspended: 'Paused after 90 days idle',
    revoked: 'Removed',
    active: 'Active',
  },
  unnamedInvite: 'Unnamed invite',
  roster: {
    loadFailed: "Could not load this overlay's moderators.",
    tryAgain: 'Try again',
    loading: 'Loading moderators…',
    // The render site emphasised "their own" with <strong>. It stays one whole
    // sentence with the emphasised words as a {placeholder}, rather than three
    // fragments the render site reassembles: word order is the first thing a
    // second language changes, and a translator needs the surrounding sentence
    // to place the emphasis at all. `emphasis` is the only key here whose value
    // is rendered as an element rather than text.
    explainer:
      'Moderators act with {emphasis} platform accounts, so Twitch, YouTube and Kick check their moderator role on every action. Removing someone here takes effect immediately.',
    explainerEmphasis: 'their own',
    inviteButton: 'Invite a moderator',
    usage: '{used} of {cap} used',
    removeAll: 'Remove all',
    atCap: 'This overlay is at its limit of {cap} moderators. Remove someone to invite another.',
    empty:
      'No one moderates this overlay yet. Invite someone and they will be able to delete messages and time viewers out from the monitor view, using their own platform accounts.',
  },
  row: {
    legChangeFailed: 'Could not change {platform} for {name}.',
    needsOneAction: '{name} needs at least one action. Remove them instead.',
    updateFailed: 'Could not update {name}.',
    removed: 'Removed {name}.',
    removeFailed: 'Could not remove {name}.',
    removeAllFailed: 'Could not remove everyone. Try again.',
    removeButton: 'Remove',
    removeLabel: 'Remove {name}',
    forAccount: 'for {account}',
    platformToggleLabel: '{platform} moderation for {name}',
    // Two keys rather than one with a branch: pluralisation rules are
    // per-language, so a second language cannot reuse an English two-form
    // choice made at the render site.
    removedCountOne: 'Removed {count} moderator.',
    removedCountMany: 'Removed {count} moderators.',
  },
  revoke: {
    title: 'Remove {name}?',
    description:
      "They lose access to this overlay's moderation on their next action. Their past actions stay in the log.",
    cancel: 'Cancel',
    confirm: 'Remove',
  },
  revokeAll: {
    title: 'Remove every moderator?',
    description:
      'This removes everyone on this overlay, including invites nobody has accepted yet. You can invite people again afterwards.',
    confirm: 'Remove everyone',
  },
  invite: {
    title: 'Invite a moderator',
    // As with roster.explainer, the emphasised clause is a {placeholder} so the
    // sentence stays whole and translatable.
    sendLink:
      'Send this link to the person you want to moderate. It works once, expires in 7 days, and {emphasis} — if it gets lost, create a new invite.',
    sendLinkEmphasis: "won't be shown again",
    copied: 'Copied',
    copyLink: 'Copy link',
    done: 'Done',
    acceptExplainer:
      'They accept with their own All-Chat account, then connect each platform the first time they moderate on it.',
    labelPrompt: 'Who is this for? (optional)',
    labelPlaceholder: 'Sarah, my Twitch mod',
    actionsLegend: 'What may they do?',
    platformsLegend: 'On which platforms? Off means they cannot act there.',
    cancel: 'Cancel',
    creating: 'Creating…',
    create: 'Create invite',
    capReached: 'This overlay already has the maximum number of moderators.',
    createFailed: 'Could not create the invite. Try again.',
    boundToAccount: 'That invite belongs to {account}.',
    copyFailed: 'Could not copy. Select the link and copy it manually.',
    premiumGate:
      'Delegating moderation is part of All-Chat premium. Your moderators never pay — only your own plan matters.',
    upgradeLink: 'Upgrade to invite moderators',
  },
} as const
