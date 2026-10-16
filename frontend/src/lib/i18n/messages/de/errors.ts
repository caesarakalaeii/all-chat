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

export const errors: LocaleNamespace<'errors'> = {
  unauthorized: {
    title: 'Anmeldung erforderlich',
    message: 'Du musst dich anmelden, um Nachrichten zu senden.',
    step1: 'Klicke auf die Schaltfläche „Mit {platform} anmelden“',
    step2: 'Autorisiere die Anwendung, Nachrichten in deinem Namen zu senden',
    step3: 'Versuche, deine Nachricht erneut zu senden',
  },
  tokenExpired: {
    title: 'Sitzung abgelaufen',
    message: 'Deine Authentifizierungssitzung ist abgelaufen.',
    step1: 'Melde dich erneut an, um deine Sitzung zu aktualisieren',
    step2: 'Stelle sicher, dass du die Anwendung autorisierst',
    step3: 'Versuche, deine Nachricht erneut zu senden',
  },
  rateLimited: {
    title: 'Sendelimit erreicht',
    message: 'Du sendest Nachrichten zu schnell. Bitte langsamer.',
    step1: 'Warte kurz, bevor du eine weitere Nachricht sendest',
    step2: 'Vermeide es, Nachrichten in schneller Abfolge zu senden',
    step3: 'Das Sendelimit wird automatisch zurückgesetzt',
  },
  banned: {
    title: 'Nachrichten senden nicht möglich',
    message: 'Du bist derzeit gebannt und kannst keine Nachrichten senden.',
    step1: 'Prüfe unten den Grund für den Bann',
    step2:
      'Melde dich beim Streamer oder bei den Moderatoren, wenn du glaubst, dass dies ein Fehler ist',
    step3: 'Warte bei einem temporären Ban, bis er abläuft',
  },
  streamerOffline: {
    title: 'Stream nicht live',
    message: 'Dieser Streamer streamt derzeit nicht.',
    step1: 'Prüfe, ob der Stream schon beendet oder noch nicht gestartet ist',
    step2: 'Lade die Seite neu, um den Stream-Status zu aktualisieren',
    step3: 'Nachrichten kannst du nur senden, wenn der Stream live ist',
  },
  platformApiError: {
    title: 'Plattform-Fehler',
    message: 'Auf der Streaming-Plattform ist ein Fehler aufgetreten.',
    step1: 'Dies ist höchstwahrscheinlich ein vorübergehendes Problem der Plattform',
    step2: 'Warte kurz und versuche es erneut',
    step3: 'Prüfe, ob die Plattform gerade Störungen hat',
    step4: 'Versuche in wenigen Augenblicken, deine Nachricht erneut zu senden',
  },
  networkError: {
    title: 'Verbindungsfehler',
    message: 'Die Verbindung zum Server konnte nicht hergestellt werden.',
    step1: 'Prüfe deine Internetverbindung',
    step2: 'Lade die Seite neu',
    step3: 'Besteht das Problem weiter, hat der Server möglicherweise Störungen',
  },
  validationError: {
    title: 'Ungültige Nachricht',
    message: 'Deine Nachricht erfüllt die Anforderungen nicht.',
    step1: 'Stelle sicher, dass deine Nachricht nicht leer ist',
    step2: 'Stelle sicher, dass deine Nachricht nicht zu lang ist',
    step3: 'Vermeide verbotene Zeichen oder Inhalte',
  },
  unknownError: {
    title: 'Unerwarteter Fehler',
    message: 'Beim Senden deiner Nachricht ist ein unerwarteter Fehler aufgetreten.',
    step1: 'Versuche, deine Nachricht erneut zu senden',
    step2: 'Lade die Seite neu, wenn das Problem weiterhin besteht',
    step3: 'Kontaktiere den Support, wenn dieser Fehler weiterhin auftritt',
  },
  display: {
    iconLabel: 'Fehlersymbol',
    dismissLabel: 'Fehler ausblenden',

    rateLimitCountdown: 'Du kannst in {countdown} wieder eine Nachricht senden',
    reasonLabel: 'Grund:',
    expiresLabel: 'Läuft ab:',
    platformMessage: 'Plattform-Nachricht: {message}',

    whatYouCanDo: 'Das kannst du tun:',
    tryAgain: 'Erneut versuchen',
    signInWith: 'Mit {platform} anmelden',
    showDetails: 'Details anzeigen',
    hideDetails: 'Details ausblenden',
  },
}
