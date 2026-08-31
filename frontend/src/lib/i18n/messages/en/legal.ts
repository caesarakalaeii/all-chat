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
 * Privacy policy, terms and the Impressum's not-configured fallback.
 *
 * The Impressum body itself is mounted at runtime from IMPRESSUM_FILE_PATH and is
 * not in this repo, so only the fallback copy lives here.
 */

export const legal = {
  // The GDPR disclosures. Reword nothing here without legal review: what
  // All-Chat stores, what it does not track and which third parties see an IP
  // address are statements of fact about the service, not marketing copy.
  cookieBanner: {
    regionLabel: 'Cookie consent',
    iconLabel: 'Cookie',
    title: 'Privacy & Data Storage',
    storageBody:
      'All-Chat uses {storage} to save your authentication tokens and keep you logged in. This is essential for the service to function properly.',
    storageEmphasis: 'browser local storage',
    noTrackingBody:
      '{emphasis} We do not share your data with third parties for advertising purposes.',
    noTrackingEmphasis:
      'We use no tracking cookies, and our usage analytics are cookieless and store no personal data.',
    detailsSummary: 'What data do we store locally?',
    // Each row keeps its bolded label inside the sentence as {label}: the text
    // after the hyphen continues the sentence the label starts, so handing a
    // translator the two halves separately would leave them unable to reorder.
    tokensRow:
      '{label} - Required to keep you logged in and authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)',
    tokensLabel: 'Authentication tokens',
    preferencesRow: '{label} - Your overlay configurations and settings',
    preferencesLabel: 'User preferences',
    noTrackingRow: "{label} - We don't track your browsing behavior",
    noTrackingLabel: 'No tracking cookies',
    noAdsRow: "{label} - We don't serve ads or share data with advertisers",
    noAdsLabel: 'No advertising cookies',
    analyticsRow:
      '{label} - We measure aggregate usage with self-hosted Umami. It sets no cookies, stores no personal identifier, and does not track public overlays',
    analyticsLabel: 'Cookieless analytics',
    fontsNote:
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.',
    fontsLabel: 'Fonts:',
    thirdPartyNote:
      '{label} Dashboard pages may load fallback avatars from {avatars} and themes from the {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.',
    thirdPartyLabel: 'Third-party resources:',
    thirdPartyAvatars: 'UI Avatars',
    thirdPartyGithub: 'GitHub API',
    agreement:
      'By using All-Chat, you agree to this essential data storage. For more details, please read our {privacy} and {terms}.',
    privacyPolicy: 'Privacy Policy',
    termsOfService: 'Terms of Service',
    acknowledge: 'I Understand',
    learnMore: 'Learn More',
    footer: 'Your data is stored locally in your browser and transmitted securely via HTTPS',
  },
} as const
