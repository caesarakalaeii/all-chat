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
 * Sign-in, the OAuth callback catchers and their error states.
 */

export const auth = {
  // ADR-0049 device pairing. See link/page.tsx's docblock: this is the only
  // human decision point in the flow, and four of these strings exist to make
  // the grant unmistakable rather than to read nicely.
  link: {
    title: 'Link a control surface',
    intro:
      'Approve a Stream Deck, StreamController or other desktop control surface. Nothing is copied or pasted — the credential goes straight to the plugin, and you can revoke it any time from {devices}.',
    introDevices: 'your paired devices',
    loading: 'Loading link request',
    codeTitle: 'Enter the pairing code',
    codeBody:
      'Your plugin is showing an eight-character code. Type it here. This is the path for a control surface on a different machine from this browser.',
    codeLabel: 'Pairing code',
    codePlaceholder: 'ABCD-EFGH',
    codeHint: 'Case does not matter, and the dash is optional. Codes expire after ten minutes.',
    codeChecking: 'Checking…',
    codeContinue: 'Continue',
    codeInvalid:
      'That code is not valid. Check the code your plugin is showing, or start linking again from the plugin.',
    approveTitle: 'A device wants to control your chat',
    // {name} is the plugin's self-reported name, so it is a parameter rather
    // than part of the sentence: it is not copy and we do not vouch for it.
    approveBody:
      'It says it is “{name}”. That name is {selfReported} — we have no way to verify it, so treat it as a hint, not proof. If you did not just start linking a control surface, deny this.',
    approveSelfReported: 'self-reported by the plugin',
    overlayLegend: 'Which overlay may it control?',
    overlayBody:
      'The device is locked to this overlay for as long as it stays paired. To move it, revoke it and link it again.',
    overlayNone: 'You have no overlays yet. Create one first, then link your device.',
    scopeLegend: 'What may it do?',
    scopeBody:
      'The plugin asked for the items below. Turn off anything you do not want it to have — you can grant less than it asked for, never more.',
    scopeNone: 'Pick at least one — a device with none can sign in but do nothing.',
    // The honest limit of the overlay binding. Chat send has no overlay
    // dimension, so the token is genuinely broader than the picker suggests.
    chatWriteCaveat:
      'Worth knowing: sending chat is not per-overlay. It goes to every platform your account has connected, so this permission is not narrowed by the overlay you picked above. Only the poll and prediction actions are.',
    scopeChatWriteTitle: 'Send chat messages',
    scopeChatWriteBody: 'Lets this device post messages to your connected chats.',
    scopeEngagementWriteTitle: 'Run polls and predictions',
    scopeEngagementWriteBody:
      'Lets this device open, close, resolve and cancel polls and predictions.',
    // An unrecognised scope still has to render. Its title falls back to the
    // raw scope string, which is a protocol value and not copy.
    scopeUnknownBody: 'Requested by this device.',
    nameLabel: 'Name this device',
    namePlaceholder: 'Stream Deck (studio PC)',
    nameHint:
      "Shown in your paired-devices list, so you know what you are revoking later. Starts as the plugin's own suggestion; change it to anything you like.",
    approving: 'Approving…',
    approve: 'Approve this device',
    deny: 'Deny',
    approvedTitle: 'Device approved',
    approvedBody: 'Return to your plugin — it will finish linking on its own.',
    deniedTitle: 'Device denied',
    approveFailed:
      'Could not approve this device. The request may have expired — start linking again from the plugin.',
    denyFailed: 'Could not deny this request. It will expire on its own within ten minutes.',
    overlaysFailed: 'Could not load your overlays. Refresh the page to try again.',
    requestExpired:
      'This link request has expired or was already used. Start linking again from the plugin.',
  },
  callback: {
    authenticating: 'Authenticating...',
    noCode: 'No authentication code received',
    exchangeFailed: 'Authentication failed. The code may have expired — please try again.',
    failed: 'Authentication failed. Please try again.',
    returnHome: 'Return to Home',
  },
  viewerSuccess: {
    completing: 'Completing authentication...',
    pleaseWait: 'Please wait',
    succeeded: 'Authentication successful!',
    closeWindow: 'You can close this window',
    noCode: 'No authentication code received',
    exchangeFailed: 'Authentication failed. The code may have expired — please try again.',
    viewerInfoFailed: 'Failed to complete authentication. Please try again.',
  },
  viewerError: {
    title: 'Authentication Failed',
    defaultError: 'Authentication failed',
    body: 'There was an error authenticating with your {account}. Please try again or contact support if the problem persists.',
    // Two forms, not one with an optional hole: this page once hardcoded
    // "Twitch" for every platform, so an unrecognised ?platform= must resolve
    // to the neutral wording rather than misname the provider that failed.
    accountNamed: '{platform} account',
    accountNeutral: 'streaming account',
    returnHome: 'Return to Home',
    goBack: 'Go Back',
  },
} as const
