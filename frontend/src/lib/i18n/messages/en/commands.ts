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
 * The chat commands editor (/commands, ADR-0079). A chat command template token
 * such as {user} never appears literally in this catalog: the catalog's own
 * placeholders use the same brace syntax, so the editor passes tokens in as the
 * {token} parameter.
 */

export const commands = {
  heading: 'Chat commands',
  intro:
    'Answer chat automatically and clip on cue. A command fires when a message starts with its trigger, a keyword when the trigger appears anywhere in a message. Replies are posted from your own account, in the chat the message came from.',
  newCommand: 'New command',
  loading: 'Loading chat commands',
  loadFailed: 'Could not load your chat commands.',
  retry: 'Retry',
  empty: 'No commands yet. Start from an example or create your own.',
  examplesHeading: 'Start from an example',
  examples: {
    hello: 'Greeting',
    clip: 'Clip on cue',
    dice: 'Dice roll',
    shoutout: 'Shoutout (mods)',
  },
  list: {
    kindCommand: 'Command',
    kindKeyword: 'Keyword',
    clips: 'Clips',
    cooldown: '{seconds}s cooldown',
    uses: '{count} uses',
    noReply: 'No reply, clip only',
    edit: 'Edit',
    editLabel: 'Edit {trigger}',
    delete: 'Delete',
    deleteLabel: 'Delete {trigger}',
    enabledLabel: 'Enable {trigger}',
    toggleFailed: 'Could not update the command.',
  },
  roles: {
    everyone: 'Everyone',
    subscriber: 'Subscribers and up',
    vip: 'VIPs and up',
    moderator: 'Moderators and you',
    broadcaster: 'Only you',
  },
  status: {
    heading: 'Where commands can answer',
    twitchNoLogin: 'Twitch replies and clips need you to sign in with Twitch.',
    twitchReplies: 'Replies in Twitch chat',
    twitchClips: 'Twitch clips',
    allowed: 'Allowed',
    missing: 'Needs permission',
    grant: 'Allow on Twitch',
    granting: 'Opening Twitch',
    grantFailed: 'Could not start the Twitch permission flow.',
    grantNeedsOverlay: 'Add your Twitch channel to one of your overlays first, then allow it here.',
    youtubePremium: 'Replies in YouTube chat are on: you have premium.',
    youtubeLocked:
      'Replies in YouTube chat are premium, because each one spends YouTube API quota. Commands still clip from YouTube chat.',
    upgrade: 'See premium',
    kick: 'Replies in Kick chat use the chat permission from an overlay’s advanced controls.',
    others:
      'TikTok, Discord and the beta platforms can trigger clips, but commands cannot reply there.',
  },
  editor: {
    createTitle: 'New chat command',
    editTitle: 'Edit {trigger}',
    kindLabel: 'Fires when',
    kindCommand: 'A message starts with it',
    kindKeyword: 'A message contains it',
    triggerLabel: 'Trigger',
    triggerHintCommand: 'One word, for example !hello. Upper and lower case do not matter.',
    triggerHintKeyword: 'A word or phrase, for example clip that. Matched anywhere in a message.',
    responseLabel: 'Reply',
    responseHint: 'Leave empty to only clip. Click a variable to insert it.',
    insertVariable: 'Insert {token}',
    clipLabel: 'Create a Twitch clip',
    clipHint: 'Clips your live Twitch stream. Put {token} in the reply to post the link.',
    roleLabel: 'Who can use it',
    cooldownLabel: 'Cooldown (seconds)',
    cooldownHint: 'The command ignores chat for this long after it fires.',
    enabledLabel: 'Enabled',
    previewHeading: 'Preview',
    previewInputLabel: 'A viewer types after the trigger',
    previewInputPlaceholder: '@friend some words',
    previewEmpty: 'The reply appears here.',
    previewFailed: 'Preview unavailable.',
    cancel: 'Cancel',
    save: 'Save',
    saving: 'Saving',
    saveFailed: 'Could not save the command.',
    saved: 'Command saved',
    referenceHeading: 'Template reference',
    jsHeading: 'JavaScript',
    jsBody:
      'Wrap code in a js block to compute a reply. The value of the last expression is inserted. The variables user, touser, query, args, count, platform, channel, role and clip are available. There is no network or file access, and each reply gets 100 ms and 16 MB.',
    jsExample: 'Example',
  },
  reference: {
    user: 'Name of the chatter who used the command',
    touser: 'First word after the trigger without @, or the chatter when there is none',
    query: 'Everything after the trigger; for a keyword, the whole message',
    args: 'Single words after the trigger',
    count: 'How often the command has run',
    random: 'A random number from 1 to 100, or from your range',
    pick: 'One of the options, at random',
    time: 'The time (24h) in a time zone; UTC when left out',
    where: 'The platform and channel the message came from',
    clip: 'Link to the clip this command made',
  },
  delete: {
    title: 'Delete {trigger}?',
    body: 'The command stops answering right away. Its use count is lost.',
    cancel: 'Keep it',
    confirm: 'Delete',
    failed: 'Could not delete the command.',
  },
} as const
