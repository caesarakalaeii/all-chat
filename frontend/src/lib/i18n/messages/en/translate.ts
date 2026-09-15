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
 * The localization contributor surface: /translate (ADR-0063). Beta-tester
 * gated; see docs/frontend/I18N.md for the tool's place in the i18n design.
 */
export const translate = {
  heading: 'Help translate All-Chat',
  intro:
    'Pick a language and translate the interface, one string at a time. Everything you submit is reviewed before it goes live.',
  // The beta gate: non-beta-testers never fetch the tool, so this is what an
  // authenticated user sees when the page is reached without the role.
  gateHeading: 'Translation is a beta-tester feature',
  gateBody:
    'Contribute translations as a beta tester. Ask in the Discord if you want to help and are not in the beta yet.',
  // Locale picker.
  localeLabel: 'Language',
  localeNone: 'No languages yet',
  localeRequestButton: 'Request a language',
  localeRequestTitle: 'Request a language',
  localeRequestBody:
    'Requested languages appear for everyone once an admin approves them.',
  localeCodeLabel: 'Language code',
  localeCodePlaceholder: 'de, fr, pt-BR…',
  localeEnglishNameLabel: 'Name in English',
  localeNativeNameLabel: 'Name in the language itself',
  localeRequestSubmit: 'Send request',
  localeRequestedToast: 'Request sent — we will review it shortly',
  localeRequestFailedToast: 'Failed to send the request',
  // Namespace list with progress.
  namespaceIntro: 'Pick an area to translate. Your progress is saved per area.',
  namespaceProgress: '{done} of {total} strings',
  // The string editor.
  sourceLabel: 'English',
  yourTranslationLabel: 'Your translation',
  placeholderHint: 'Keep {placeholder} in your translation — it is replaced by a value.',
  placeholderMismatch: 'Your translation is missing {placeholder}.',
  statusPending: 'Waiting for review',
  statusApproved: 'Approved',
  statusRejected: 'Changes requested',
  rejectedNote: 'Reviewer note: {note}',
  saveButton: 'Submit {count} translation(s)',
  savedToast: '{count} translation(s) submitted for review',
  saveFailedToast: 'Submitting failed',
  rowError: 'Not submitted: {error}',
  backToNamespaces: 'All areas',
  // Accessibility: the editor region announces which string is being edited.
  stringLabel: 'Translate: {key}',
} as const
