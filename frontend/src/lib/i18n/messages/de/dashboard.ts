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

export const dashboard: LocaleNamespace<'dashboard'> = {
  overlays: {
    heading: 'Overlays',
    newOverlay: 'Neues Overlay',
    loading: 'Overlays werden geladen',
    extensionBadge: 'Extension',
    deactivateExtension: 'Extension deaktivieren',
    setAsExtension: 'Als Extension-Overlay festlegen',
    deleteLabel: '{name} löschen',
    openLabel: '{name} öffnen',
    sourceCountOne: '{count} Quelle',
    sourceCountOther: '{count} Quellen',
  },
  discoveryPaused: {
    title: 'YouTube-Erkennung pausiert',
    body: 'Eine Stunde lang wurde kein live Stream gefunden, daher hat All-Chat die Suche gestoppt. Starte die Erkennung über den Chat-Monitor neu, sobald du live gehst.',
    action: 'Chat-Monitor öffnen',
  },
  empty: {
    heading: 'Noch keine Overlays',
    body: 'Erstelle dein erstes Overlay, um den Chat all deiner Plattformen an einem Ort zu sehen.',
    createFirst: 'Erstes Overlay erstellen',
  },
  deleteOverlay: {
    title: '„{name}“ löschen?',
    description: 'Diese Aktion kann nicht rückgängig gemacht werden. Alle Quellen werden entfernt.',
    cancel: 'Abbrechen',
    confirm: 'Löschen',
  },
  toasts: {
    overlayDeleted: 'Overlay gelöscht',
    overlayDeleteFailed: 'Overlay konnte nicht gelöscht werden',
    extensionOverlayUpdated: 'Extension-Overlay aktualisiert',
    extensionOverlayDeactivated: 'Extension-Overlay deaktiviert',
    overlayUpdateFailed: 'Overlay konnte nicht aktualisiert werden',
  },
  shares: {
    heading: 'Freigabe-Anfragen',
    tabPending: 'Offen ({count})',
    tabHistory: 'Verlauf ({count})',
    loading: 'Anfragen werden geladen...',
    emptyPending: 'Keine offenen Freigabe-Anfragen',
    emptyHistory: 'Kein Anfragen-Verlauf',
    unknownUser: 'Unbekannter Nutzer',
    statusPending: 'Offen',
    statusAccepted: 'Aktiv',
    statusExpired: 'Abgelaufen',
    statusRevoked: 'Widerrufen',
    statusRejected: 'Abgelehnt',
    loadingUser: 'Nutzerinfo wird geladen...',
    revoke: 'Widerrufen',
    accept: 'Annehmen',
    reject: 'Ablehnen',
    userFallbackName: 'Nutzer',
    acceptTitle: '{sender} möchte mit dir teilen',
    cannotAcceptTitle: 'Freigabe kann nicht angenommen werden',
    noOverlaysError: 'Erstelle zuerst ein Overlay, um Freigaben anzunehmen',
    loadOverlaysFailed: 'Overlays konnten nicht geladen werden',
    close: 'Schließen',
    loadingOverlays: 'Overlays werden geladen...',
    shareBackLabel: 'Welches Overlay teilst du zurück? {required}',
    requiredMarker: '*',
    cancel: 'Abbrechen',
    acceptButton: 'Annehmen',
    accepting: 'Wird angenommen...',
    expiryLegend: 'Wann soll die Freigabe ablaufen?',
    expiryThisStream: 'Dieser Stream',
    expiryThisStreamHint: 'Läuft ab, wenn dein Stream endet',
    expiryKickUnavailable: '(für Kick nicht verfügbar – Stream-Erkennung noch nicht unterstützt)',
    expiryCustom: 'Eigene Dauer',
    expiryCustomPlaceholder: 'Stunden',
    expiryCustomLabel: 'Eigene Dauer in Stunden',
    expiryCustomHint: 'Stunden (1–168)',
    expiryCustomError: 'Muss zwischen 1 und 168 Stunden liegen',
    expiryUnlimited: 'Unbegrenzt',
    expiryUnlimitedHint: 'Läuft nie ab',
    addSourceTitle: 'Das Overlay von {sender} zu einem deiner Overlays hinzufügen?',
    addSourcePreview: 'Overlay von {sender} (geteilter Chat)',
    addSourceSelectLabel: 'Zu welchem Overlay hinzufügen?',
    addSourceSkip: 'Überspringen',
    addSourceAdd: 'Hinzufügen',
    addSourceAdding: 'Wird hinzugefügt...',
    revokeTitle: 'Freigabe an {partner} widerrufen?',
    revokeBody: 'Die Zustellung der Nachrichten wird sofort gestoppt.',
    revokeCancel: 'Abbrechen',
    revokeConfirm: 'Widerrufen',
    revoking: 'Wird widerrufen...',
    loadRequestsFailed: 'Freigabe-Anfragen konnten nicht geladen werden',
    retry: 'Erneut versuchen',
    notificationUpdateFailed: 'Benachrichtigungsstatus konnte nicht aktualisiert werden',
    acceptedToast: 'Freigabe von {sender} angenommen!',
    acceptedToastUnknownSender: 'Nutzer',
    circularTitle: 'Kann nicht angenommen werden',
    circularBody: 'Dies würde eine zirkuläre Freigabe-Abhängigkeit erzeugen',
    addSourceToast: 'Overlay von {sender} hinzugefügt!',
    revokedToast: 'Freigabe widerrufen',
    revokeFailedToast: 'Freigabe konnte nicht widerrufen werden',
  },
}
