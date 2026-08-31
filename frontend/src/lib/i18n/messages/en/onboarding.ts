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
 * The first-run tour and its steps.
 */

export const onboarding = {
  steps: {
    createOverlay: 'Create your overlay',
    connectSource: 'Connect a chat source',
    chooseTheme: 'Pick a theme',
    copyObs: 'Add it to OBS',
  },
  checklist: {
    title: 'Setup guide',
    allDone: 'All steps done!',
    progress: '{done} of {total} steps done',
    expand: 'Expand setup guide',
    minimize: 'Minimize setup guide',
    dismiss: 'Dismiss setup guide',
    // Renders straight after the step label inside VisuallyHidden, so the
    // leading space is what separates the two for a screen reader.
    stepDone: ' (done)',
    create: 'Create',
    showMe: 'Show me',
    openEditor: 'Open editor',
    copyLink: 'Copy link',
    copied: 'Copied!',
  },
  obs: {
    heading: 'In OBS:',
    // One whole sentence with two placeholders rather than three fragments:
    // the emphasised runs are OBS UI affordances, and the word order around
    // them is the first thing a second language rearranges.
    step1: 'In OBS, click {plus} under Sources and choose {browser}.',
    step1Plus: '+',
    step1Browser: 'Browser',
    step2: 'Paste the copied overlay link into the URL field.',
    step3:
      'Size the source to the area chat should fill (a tall, narrow box like 450×800 works well, not your full canvas), then drag it into place. Chat appears as soon as the overlay connects.',
  },
  extras: {
    heading: 'Optional: go further',
    ttsTitle: 'Text-to-speech',
    ttsBody: 'Read chat aloud. Browser voices are free; ElevenLabs voices are Premium.',
    moderationTitle: 'Moderate from your overlay',
    moderationBody:
      'Delete, timeout, ban and unban from the Monitor View button at the top of the editor. Full controls on Twitch, Kick and Discord; timeout and ban on YouTube. (Premium)',
    moderatorsTitle: 'Let your mods help',
    moderatorsBody:
      'Hand the Monitor View to your existing moderators under Moderators. They act with their own platform accounts, and they never need Premium themselves. (Premium)',
    sharedChatTitle: 'Shared chat',
    sharedChatBody:
      'Combine several channels into one conversation via the Share Overlay button. (Premium)',
    streamSelectionTitle: 'YouTube stream selection',
    streamSelectionBody:
      'Pick exactly which broadcast an overlay listens to, per YouTube source in Sources. (Premium)',
    bubbleColorsTitle: 'Differently-coloured bubbles',
    bubbleColorsBody:
      'Give each platform its own bubble colour, or cycle a palette down the feed, under Bubble colors in Appearance. Free.',
    flairsTitle: 'Viewer flairs',
    flairsBody:
      'Premium cosmetics for chatters, like animated name gradients, under Flairs in the navigation.',
    streamDeckTitle: 'Stream Deck buttons',
    streamDeckBody:
      'Drive polls, predictions and canned messages from a physical button. Link a Stream Deck or StreamController under Paired devices — nothing to copy or paste. Starting a poll or prediction is still Premium.',
    seePremium: 'See everything Premium includes',
    done: 'Done',
    skip: 'Skip',
  },
  finale: {
    title: "You're live! 🎉",
    body: 'Questions, feedback, or theme requests? Our community is happy to help.',
    joinDiscord: 'Join the Discord',
    finish: 'Finish',
  },
  dismissDialog: {
    title: 'Hide the setup guide?',
    body: 'You can restart it anytime from Settings → Setup guide.',
    keep: 'Keep it',
    hide: 'Hide guide',
  },
  createDialog: {
    title: 'Create your overlay',
    body: "Give it a name — you'll connect your chat right after.",
    nameLabel: 'Overlay name',
    cancel: 'Cancel',
    submit: 'Create overlay',
    submitting: 'Creating…',
    failedToast: 'Could not create the overlay',
  },
} as const
