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
  // /moderate/accept: the page a delegation invite link lands on (ADR-0048).
  accept: {
    heading: 'Moderation invite',
    loadingInvite: 'Loading invite',
    goToChannels: 'Go to your channels',
    accept: 'Accept and start moderating',
    accepting: 'Accepting…',
    notNow: 'Not now',
    // Not moderation.actions.*: those are the roster's terse labels. This page
    // spells each action out for someone deciding whether to accept, keyed by
    // the ModerationAction the row grants.
    actionDelete: 'Delete messages',
    actionTimeout: 'Time viewers out',
    actionBan: 'Ban viewers',
    actionUnban: 'Lift bans and timeouts',
    signInHeading: 'Sign in to accept this invite',
    signInBody:
      'Moderating is tied to an All-Chat account, so we need to know which one to hand this to. Sign in, then open the invite link again — it stays valid.',
    signIn: 'Sign in',
    // {owner} is emphasised at the render site.
    askingToHelp: '{owner} is asking you to help moderate',
    // The render site spelled the quotes &ldquo;/&rdquo;.
    addressedTo: 'They addressed this invite to “{label}”.',
    actionsHeading: 'What you would be able to do',
    platformsHeading: 'On these platforms',
    noPlatforms: 'None yet — {owner} still has to turn a platform on.',
    ownAccountNote:
      'You will act with your own platform account, so each platform still checks that {owner} made you a moderator there. Nothing is asked of you now — you connect a platform the first time you moderate on it.',
    // expected_platform is optional, so two whole sentences rather than one key
    // with an empty placeholder in it.
    expectedAccount: 'This invite is meant for {account}.',
    expectedAccountOnPlatform: 'This invite is meant for {platform} {account}.',
    errorMissingToken: 'This link is missing its invite code. Ask the streamer to send it again.',
    // Deliberately covers unknown, already redeemed and revoked alike: the
    // server keeps the three indistinguishable, so the copy names all three
    // rather than guessing one.
    errorNotFound:
      'This invite is not valid any more — it may already have been used, or the streamer may have withdrawn it. Ask them for a new one.',
    errorExpired: 'This invite has expired. Ask the streamer for a new one.',
    errorAlreadyModerator: 'You already moderate this channel. It is on your channels page.',
    errorOwnerCannotAccept:
      'This is your own overlay — you already have full moderation on it.',
    // The platform and the account are each optional, so all four reachable
    // combinations are whole sentences.
    errorBoundToOther:
      'This invite is for a specific account. Sign in as that account, or ask the streamer to send a new invite for this one.',
    errorBoundToOtherAccount:
      'This invite is for a specific account ({account}). Sign in as that account, or ask the streamer to send a new invite for this one.',
    errorBoundToOtherPlatform:
      'This invite is for a specific {platform} account. Sign in as that account, or ask the streamer to send a new invite for this one.',
    errorBoundToOtherBoth:
      'This invite is for a specific {platform} account ({account}). Sign in as that account, or ask the streamer to send a new invite for this one.',
    errorUnknown: 'Could not open this invite. Check the link and try again.',
  },
  // /moderate: "Channels you moderate", the only listing an accepted delegation
  // is reachable from. Never links a moderator to /upgrade — entitlement is the
  // streamer's, so there is nothing here a volunteer could buy.
  channels: {
    heading: 'Channels you moderate',
    subheading:
      'Overlays other streamers have handed you. You act with your own platform account, so each platform still checks that you are one of their moderators.',
    loading: 'Loading channels',
    loadFailed: 'Could not load your channels.',
    tryAgain: 'Try again',
    emptyHeading: 'No channels yet',
    emptyBody:
      'When a streamer invites you to moderate their overlay, they send you a private link. Open it while signed in to this account and their channel appears here.',
    forOwner: 'for {owner}',
    noPlatforms: 'No platforms turned on yet — ask {owner} to enable one.',
    suspendedNote: 'Paused after 90 days without any actions. Ask {owner} to turn it back on.',
    // The render site spelled the apostrophe &apos;, which is U+0027.
    unavailableNote:
      "{owner}'s plan does not include moderation right now, so actions are unavailable until they renew it.",
    openMonitor: 'Open chat monitor',
    discordPromptBody:
      'Link your Discord account to moderate Discord. All-Chat checks your own server permissions before it acts, so it needs to know which Discord account is yours.',
    linkDiscord: 'Link Discord',
    // The mod-consent redirect's query string. already_linked is the one failure
    // worth its own words: that Discord account backs a different All-Chat
    // account, and retrying cannot change it.
    noticeDiscordAlreadyLinked:
      'That Discord account is already linked to another All-Chat account. Link a different one, or unlink it from the other account first.',
    noticeConnectFailed:
      'That connection did not complete. Open a channel below and try again from there.',
    noticeDiscordLinked:
      'Discord account linked. All-Chat can now check your own server permissions when you moderate Discord.',
    noticeConnected:
      '{platform} connected. It now covers every channel that delegated {platform} to you.',
  },
} as const
