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

export const auth: LocaleNamespace<'auth'> = {
  link: {
    title: 'Steuerungsgerät koppeln',
    intro:
      'Genehmige ein Stream Deck, StreamController oder ein anderes Desktop-Steuerungsgerät. Nichts wird kopiert oder eingefügt — die Anmeldeinformation geht direkt an das Plugin, und du kannst sie jederzeit über {devices} widerrufen.',
    introDevices: 'deine gekoppelten Geräte',
    loading: 'Kopplungsanfrage wird geladen',
    codeTitle: 'Gib den Kopplungscode ein',
    codeBody:
      'Dein Plugin zeigt einen achtstelligen Code an. Tippe ihn hier ein. Dieser Weg ist für ein Steuerungsgerät auf einem anderen Rechner als diesem Browser.',
    codeLabel: 'Kopplungscode',
    codePlaceholder: 'ABCD-EFGH',
    codeHint:
      'Groß- und Kleinschreibung ist egal, der Strich ist optional. Codes laufen nach zehn Minuten ab.',
    codeChecking: 'Wird geprüft…',
    codeContinue: 'Weiter',
    codeInvalid:
      'Dieser Code ist ungültig. Prüfe den Code, den dein Plugin anzeigt, oder starte die Kopplung im Plugin neu.',
    approveTitle: 'Ein Gerät möchte deinen Chat steuern',
    approveBody:
      'Es nennt sich „{name}“. Dieser Name wurde {selfReported} — wir können ihn nicht überprüfen, also betrachte ihn als Hinweis, nicht als Beweis. Falls du gerade keine Kopplung eines Steuerungsgeräts gestartet hast, lehne dies ab.',
    approveSelfReported: 'vom Plugin selbst angegeben',
    overlayLegend: 'Welches Overlay darf es steuern?',
    overlayBody:
      'Das Gerät ist an dieses Overlay gebunden, solange es gekoppelt ist. Um es zu verschieben, widerrufe es und koppel es erneut.',
    overlayNone: 'Du hast noch keine Overlays. Erstelle zuerst eines und koppel dann dein Gerät.',
    scopeLegend: 'Was darf es tun?',
    scopeBody:
      'Das Plugin hat die folgenden Berechtigungen angefragt. Deaktiviere alles, was du es nicht haben lassen willst — du kannst weniger gewähren als angefragt, aber nie mehr.',
    scopeNone:
      'Wähle mindestens eine — ein Gerät ohne Berechtigung kann sich zwar anmelden, aber nichts tun.',
    chatWriteCaveat:
      'Gut zu wissen: Das Senden von Chat-Nachrichten ist nicht pro Overlay möglich. Sie gehen an jede Plattform, mit der dein Konto verbunden ist, daher wird diese Berechtigung durch das oben gewählte Overlay nicht eingeschränkt. Nur die Umfrage- und Vorhersage-Aktionen sind es.',
    scopeChatWriteTitle: 'Chat-Nachrichten senden',
    scopeChatWriteBody:
      'Erlaubt diesem Gerät, Nachrichten in deine verbundenen Chats zu schreiben.',
    scopeEngagementWriteTitle: 'Umfragen und Vorhersagen starten',
    scopeEngagementWriteBody:
      'Erlaubt diesem Gerät, Umfragen und Vorhersagen zu öffnen, zu schließen, auszuwerten und abzubrechen.',
    scopeUnknownBody: 'Von diesem Gerät angefragt.',
    nameLabel: 'Benenne dieses Gerät',
    namePlaceholder: 'Stream Deck (Studio-PC)',
    nameHint:
      'Wird in deiner Liste der gekoppelten Geräte angezeigt, damit du später weißt, was du widerrufst. Startet mit dem Vorschlag des Plugins; ändere ihn beliebig.',
    approving: 'Wird genehmigt…',
    approve: 'Dieses Gerät genehmigen',
    deny: 'Ablehnen',
    approvedTitle: 'Gerät genehmigt',
    approvedBody: 'Kehre zu deinem Plugin zurück — es schließt die Kopplung selbstständig ab.',
    deniedTitle: 'Gerät abgelehnt',
    approveFailed:
      'Dieses Gerät konnte nicht genehmigt werden. Die Anfrage ist möglicherweise abgelaufen — starte die Kopplung im Plugin neu.',
    denyFailed:
      'Diese Anfrage konnte nicht abgelehnt werden. Sie läuft innerhalb von zehn Minuten von selbst ab.',
    overlaysFailed:
      'Deine Overlays konnten nicht geladen werden. Lade die Seite neu und versuche es erneut.',
    requestExpired:
      'Diese Kopplungsanfrage ist abgelaufen oder wurde bereits verwendet. Starte die Kopplung im Plugin neu.',
  },
  callback: {
    authenticating: 'Authentifizierung läuft...',
    noCode: 'Kein Authentifizierungscode erhalten',
    exchangeFailed:
      'Authentifizierung fehlgeschlagen. Der Code ist möglicherweise abgelaufen — bitte versuche es erneut.',
    failed: 'Authentifizierung fehlgeschlagen. Bitte versuche es erneut.',
    returnHome: 'Zur Startseite',
  },
  viewerSuccess: {
    completing: 'Authentifizierung wird abgeschlossen...',
    pleaseWait: 'Bitte warten',
    succeeded: 'Authentifizierung erfolgreich!',
    closeWindow: 'Du kannst dieses Fenster schließen',
    noCode: 'Kein Authentifizierungscode erhalten',
    exchangeFailed:
      'Authentifizierung fehlgeschlagen. Der Code ist möglicherweise abgelaufen — bitte versuche es erneut.',
    viewerInfoFailed:
      'Authentifizierung konnte nicht abgeschlossen werden. Bitte versuche es erneut.',
  },
  viewerError: {
    title: 'Authentifizierung fehlgeschlagen',
    defaultError: 'Authentifizierung fehlgeschlagen',
    body: 'Beim Anmelden mit deinem {account} ist ein Fehler aufgetreten. Bitte versuche es erneut oder kontaktiere den Support, falls das Problem bestehen bleibt.',
    accountNamed: '{platform}-Konto',
    accountNeutral: 'Streaming-Konto',
    returnHome: 'Zur Startseite',
    goBack: 'Zurück',
  },
}
