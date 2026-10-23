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
    'Answer chat automatically, clip on cue and put images on stream. A command fires when a message starts with its trigger, a keyword when the trigger appears anywhere in a message. Replies go to the chat the message came from, sent by the All-Chat Bot or, if you choose, your own account.',
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
    dog: 'Random dog',
  },
  list: {
    kindCommand: 'Command',
    kindKeyword: 'Keyword',
    clips: 'Clips',
    image: 'Image',
    cooldown: '{seconds}s cooldown',
    uses: '{count} uses',
    noReply: 'No reply in chat',
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
    responseHint: 'Leave empty to only clip or show an image. Click a variable to insert it.',
    insertVariable: 'Insert {token}',
    imageLabel: 'Image on stream',
    imageHint:
      'An https link to a picture or GIF, shown on your alerts overlays whenever the command fires, from any chat. Templates work here, but what viewers type never reaches the image.',
    imagePreviewAlt: 'Image preview',
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
      'Wrap code in a js block to compute a reply. The value of the last expression is inserted. The variables user, touser, query, args, count, platform, channel, role and clip are available. The code itself has no network or file access, and each reply gets 100 ms and 16 MB.',
    jsExample: 'Example',
    urlfetchHeading: 'Fetching a web address',
    urlfetchBody:
      '{token} inserts the text a web address answers with. Inside a js block it arrives as a string, ready for JSON.parse. Only public https addresses work, at most 3 per reply and 3 seconds each. Variables in the address are escaped, so viewers cannot change where it points.',
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
    urlfetch: 'The text a web address answers with',
  },
  delete: {
    title: 'Delete {trigger}?',
    body: 'The command stops answering right away. Its use count is lost.',
    cancel: 'Keep it',
    confirm: 'Delete',
    failed: 'Could not delete the command.',
  },

  // The bot sender (ADR-0081): All-Chat Bot by default, the streamer's own
  // account as the alternative; premium custom bots per platform live here too.
  bot: {
    heading: 'Bot and sender',
    senderBot: 'Replies come from the All-Chat Bot',
    senderBotHint: 'Works on Twitch, Kick and YouTube without a separate account.',
    senderSelf: 'Replies come from your own account',
    senderSelfHint: 'The message appears as if you typed it. Requires the chat-send permission.',
    senderToggleLabel: 'Toggle who sends replies: All-Chat Bot or your account',
    senderSaveFailed: 'Could not change who sends replies.',
    twitchGranted: 'The All-Chat Bot is allowed in your Twitch chat.',
    twitchNotGranted: 'The All-Chat Bot needs your permission to post in your Twitch chat.',
    grant: 'Allow in Twitch chat',
    granting: 'Opening Twitch',
    grantFailed: 'Could not start the Twitch permission flow.',
    connectedToast: 'Bot connected.',
    connectFailedToast: 'Connecting the bot failed.',
    customHeading: 'Custom bot accounts',
    customIntro: 'Connect your own bot account per platform. Replies turn to that name.',
    customPremium:
      'Custom bot accounts are a premium feature. See premium to connect your own bot on Twitch, Kick or YouTube.',
    noneConnected: 'No custom bot connected',
    connectedAs: '{name} ({login})',
    connect: 'Connect {platform} bot',
    connecting: 'Connecting',
    connectFailed: 'Could not start the bot connection.',
    disconnect: 'Disconnect',
    disconnecting: 'Disconnecting',
    disconnectLabel: 'Disconnect the {platform} bot',
    disconnected: 'Bot disconnected.',
    disconnectFailed: 'Disconnecting the bot failed.',
    removeTitle: 'Disconnect the {platform} bot?',
    removeBody: 'Replies go back to the All-Chat Bot or your own account on that platform.',
    removeCancel: 'Keep it',
    removeConfirm: 'Disconnect',
  },

  // Repeating chat messages (ADR-0081): interval minutes, minimum chat
  // messages, or both.
  timers: {
    heading: 'Timers',
    intro:
      'Timers post a message in your Twitch and Kick chat, and in YouTube chat with premium. Give one an interval in minutes, a minimum count of chat messages, or both: with both set it fires every X minutes, but only if at least Y messages arrived since the last post. A timer with only an interval also posts while you are offline.',
    new: 'New timer',
    empty: 'No timers yet.',
    edit: 'Edit',
    delete: 'Delete',
    deleteLabel: 'Delete {name}',
    enabledLabel: 'Enable {name}',
    toggleFailed: 'Could not update the timer.',
    deleteFailed: 'Could not delete the timer.',
    saved: 'Timer saved',
    createTitle: 'New timer',
    editTitle: 'Edit {name}',
    dialogIntro: 'A timer posts its message in your chat on a schedule or on chat activity.',
    nameLabel: 'Name',
    nameRequired: 'Give the timer a name.',
    messageLabel: 'Message',
    messageHint:
      'What gets posted. Template variables that need no chatter work, such as count, random, pick, time and js.',
    messageRequired: 'Write the message to post.',
    intervalLabel: 'Interval (minutes, optional)',
    intervalHint: 'Between {min} and {max} minutes. Leave empty for no interval.',
    messagesLabel: 'Minimum messages (optional)',
    messagesHint: 'Between {min} and {max} messages. Leave empty for no minimum.',
    combinedRule:
      'Set at least one of the two. With both set, the timer waits for the interval and for the minimum number of messages since its last post.',
    triggerRequired: 'Set an interval, a minimum message count, or both.',
    intervalRange: 'The interval must be a whole number between {min} and {max} minutes.',
    messagesRange: 'The minimum must be a whole number between {min} and {max} messages.',
    triggerBoth: 'every {minutes} minutes, at least {messages} messages since the last one',
    triggerEvery: 'every {minutes} minutes',
    triggerMessages: 'after {messages} new messages',
    cancel: 'Cancel',
    save: 'Save',
    saving: 'Saving',
    saveFailed: 'Could not save the timer.',
    deleteTitle: 'Delete {name}?',
    deleteBody: 'The timer stops posting right away.',
    deleteCancel: 'Keep it',
    deleteConfirm: 'Delete',
  },
} as const
