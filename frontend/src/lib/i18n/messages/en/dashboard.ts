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
 * The streamer dashboard and the widgets it owns.
 */

export const dashboard = {
  overlays: {
    heading: 'Overlays',
    newOverlay: 'New Overlay',
    loading: 'Loading overlays',
    extensionBadge: 'Extension',
    deactivateExtension: 'Deactivate Extension',
    setAsExtension: 'Set as Extension Overlay',
    deleteLabel: 'Delete {name}',
    // Two keys rather than a concatenated 's': see the comment in
    // __tests__/dashboard.test.ts. The caller picks by count.
    sourceCountOne: '{count} source',
    sourceCountOther: '{count} sources',
  },
  // The overlay-card warning for a YouTube channel whose stream discovery parked
  // itself after an hour of finding nothing live. Reads as "action needed", not
  // "broken", matching viewerOverlay.statusIndicator.discoveryPaused — nothing has
  // failed, but only Rediscover clears it, and a browser-source refresh will not.
  discoveryPaused: {
    title: 'YouTube discovery paused',
    body: 'No live stream was found for an hour, so All-Chat stopped looking. Re-discover from the chat monitor when you go live.',
    action: 'Open chat monitor',
  },
  empty: {
    heading: 'No overlays yet',
    body: 'Create your first overlay to see chat from all your platforms in one place.',
    createFirst: 'Create your first overlay',
  },
  deleteOverlay: {
    // The render site spelled the quotes &ldquo;/&rdquo;. A catalog string is
    // not HTML, so they are the characters themselves.
    title: 'Delete “{name}”?',
    description: 'This action cannot be undone. All sources will be removed.',
    cancel: 'Cancel',
    confirm: 'Delete',
  },
  toasts: {
    overlayDeleted: 'Overlay deleted',
    overlayDeleteFailed: 'Failed to delete overlay',
    extensionOverlayUpdated: 'Extension overlay updated',
    extensionOverlayDeactivated: 'Extension overlay deactivated',
    overlayUpdateFailed: 'Failed to update overlay',
  },
  // /dashboard/shares: incoming overlay-share requests and the four modals that
  // accept, add, and revoke them.
  shares: {
    heading: 'Share Requests',
    // The count is inside the tab label, so it is a placeholder.
    tabPending: 'Pending ({count})',
    tabHistory: 'History ({count})',
    loading: 'Loading requests...',
    emptyPending: 'No pending share requests',
    emptyHistory: 'No request history',
    // Stands in for a sender whose profile did not load.
    unknownUser: 'Unknown User',
    // Status badges, keyed by the ShareRequest status. 'accepted' reads "Active"
    // because the badge names the state of the share, not of the request.
    statusPending: 'Pending',
    statusAccepted: 'Active',
    statusExpired: 'Expired',
    statusRevoked: 'Revoked',
    statusRejected: 'Rejected',
    loadingUser: 'Loading user info...',
    revoke: 'Revoke',
    accept: 'Accept',
    reject: 'Reject',
    // Stands in for a sender with no display name.
    userFallbackName: 'User',
    acceptTitle: '{sender} wants to share with you',
    cannotAcceptTitle: 'Cannot Accept Share',
    noOverlaysError: 'Create an overlay first to accept shares',
    loadOverlaysFailed: 'Failed to load overlays',
    close: 'Close',
    loadingOverlays: 'Loading overlays...',
    // {required} is the red asterisk, rendered as its own element.
    shareBackLabel: 'Share back which overlay? {required}',
    requiredMarker: '*',
    cancel: 'Cancel',
    acceptButton: 'Accept',
    accepting: 'Accepting...',
    expiryLegend: 'When should the share expire?',
    expiryThisStream: 'This stream',
    expiryThisStreamHint: 'Expires when your stream ends',
    // Kick has no stream-lifecycle detection, so 'this stream' would never
    // expire there and the radio is disabled.
    expiryKickUnavailable: '(not available for Kick — stream detection not yet supported)',
    expiryCustom: 'Custom duration',
    expiryCustomPlaceholder: 'hours',
    expiryCustomLabel: 'Custom duration in hours',
    expiryCustomHint: 'hours (1-168)',
    expiryCustomError: 'Must be between 1 and 168 hours',
    expiryUnlimited: 'Unlimited',
    expiryUnlimitedHint: 'Never expires',
    // The render site spelled the apostrophes &apos;, which is U+0027.
    addSourceTitle: "Add {sender}'s overlay to one of yours?",
    addSourcePreview: "{sender}'s overlay (shared chat)",
    addSourceSelectLabel: 'Add to which overlay?',
    addSourceSkip: 'Skip',
    addSourceAdd: 'Add',
    addSourceAdding: 'Adding...',
    revokeTitle: 'Revoke share with {partner}?',
    revokeBody: 'This will stop message delivery immediately.',
    revokeCancel: 'Cancel',
    revokeConfirm: 'Revoke',
    revoking: 'Revoking...',
    // Toasts. loadOverlaysFailed above is the modal's inline error, not a toast.
    loadRequestsFailed: 'Failed to load share requests',
    // One key for both notification toggles: the call sites were identical.
    notificationUpdateFailed: 'Failed to update notification status',
    acceptedToast: 'Share accepted from {sender}!',
    // Mid-sentence, so lowercase; userFallbackName stands alone and is 'User'.
    acceptedToastUnknownSender: 'user',
    circularTitle: 'Cannot accept',
    circularBody: 'This would create a circular share dependency',
    addSourceToast: "Added {sender}'s overlay!",
    revokedToast: 'Share revoked',
    revokeFailedToast: 'Failed to revoke share',
  },
} as const
