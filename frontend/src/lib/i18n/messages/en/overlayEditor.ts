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
 * The overlay editor: canvas, inspector, appearance controls and the theme
 * marketplace.
 */

export const overlayEditor = {
  nav: {
    settingsLabel: 'Overlay settings',
    // The count is a placeholder rather than an appended `({n})`: the
    // parentheses are punctuation a language may render differently.
    advancedCount: 'Advanced ({count})',
  },
  previewBackdrop: {
    heading: 'Backdrop',
    appBackground: 'Preview on app background',
    lightBackground: 'Preview on light background',
    chromaGreen: 'Preview on chroma green',
    customColor: 'Custom preview background color',
  },
  settingsSearch: {
    label: 'Search settings',
    placeholder: 'Search settings… (e.g. badge, fade, banned words)',
    clearLabel: 'Clear search',
    resultsLabel: 'Matching settings',
    noResults: 'No settings match “{query}”',
  },
  themeMarketplace: {
    // Two title keys, not a qualifier concatenated onto a shared noun phrase:
    // see the comment in __tests__/overlayEditor.test.ts.
    title: 'Theme Marketplace',
    titleCreditRoll: 'Credit Roll Theme Marketplace',
    description: 'Browse and apply custom CSS themes for your overlay',
    descriptionCreditRoll: 'Browse and apply custom CSS themes for your credit roll',
    loading: 'Loading themes',
    loadingEllipsis: 'Loading themes...',
    errorTitle: 'Error Loading Themes',
    emptyTitle: 'No themes found',
    emptyBody: 'Try adjusting your filters',
    showingCount: 'Showing {shown} of {total} themes',
    applyTheme: 'Apply Theme',
    clearFilters: 'Clear Filters',
    searchLabel: 'Search themes',
    searchPlaceholder: 'Search themes...',
    sync: 'Sync',
    syncTitleInline: 'Force refresh themes from GitHub (Admin)',
    syncLabel: 'Force refresh themes from GitHub',
    syncTitle: 'Force refresh themes (Admin)',
    closeLabel: 'Close theme marketplace',
  },
  // Sample copy the credit-roll theme preview renders so a streamer can see a
  // theme applied to something. It mirrors the real credits overlay.
  creditRollPreview: {
    heading: '🎬 Stream Credits',
    subheading: 'Thank you for your support!',
    leaderboardHeading: 'Top Subscribers',
    footerHeading: 'Thank you! ❤️',
    footerBody: 'See you next stream!',
  },
} as const
