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

import type { LocaleNamespace } from '../../translate'

export const translate: LocaleNamespace<'translate'> = {
  heading: 'Hilf bei der Übersetzung von All-Chat',
  intro:
    'Wähle eine Sprache und übersetze die Oberfläche, immer einen Text nach dem anderen. Alles, was du einreichst, wird vor der Veröffentlichung geprüft.',
  gateHeading: 'Übersetzen ist eine Betatester-Funktion',
  gateBody:
    'Reiche Übersetzungen als Betatester ein. Frag im Discord, wenn du helfen willst und noch nicht im Beta-Programm bist.',
  localeLabel: 'Sprache',
  localeNone: 'Noch keine Sprachen',
  localeRequestButton: 'Sprache anfragen',
  localeRequestTitle: 'Sprache anfragen',
  localeRequestBody:
    'Reiche Übersetzungen als Betatester ein. Frag im Discord, wenn du helfen willst und noch nicht im Beta-Programm bist.',
  localeCodeLabel: 'Sprachcode',
  localeCodePlaceholder: 'de, fr, pt-BR…',
  localeEnglishNameLabel: 'Name auf Englisch',
  localeNativeNameLabel: 'Name in der Sprache selbst',
  localeRequestSubmit: 'Anfrage senden',
  localeRequestedToast: 'Anfrage gesendet – wir prüfen sie in Kürze',
  localeRequestFailedToast: 'Anfrage konnte nicht gesendet werden',
  namespaceIntro:
    'Wähle einen Bereich zum Übersetzen. Dein Fortschritt wird pro Bereich gespeichert.',
  namespaceProgress: '{done} von {total} Texten',
  sourceLabel: 'Englisch',
  yourTranslationLabel: 'Deine Übersetzung',
  placeholderHint:
    'Behalte {placeholder} in deiner Übersetzung – es wird durch einen Wert ersetzt.',
  placeholderMismatch: 'Deiner Übersetzung fehlt {placeholder}.',
  statusPending: 'Warte auf Prüfung',
  statusApproved: 'Freigegeben',
  statusRejected: 'Änderungen angefordert',
  rejectedNote: 'Anmerkung der Prüfung: {note}',
  saveButton: '{count} Übersetzung(en) einreichen',
  savedToast: '{count} Übersetzung(en) zur Prüfung eingereicht',
  saveFailedToast: 'Senden fehlgeschlagen',
  rowError: 'Nicht eingereicht: {error}',
  backToNamespaces: 'Alle Bereiche',
  stringLabel: 'Übersetzen: {key}',
}
