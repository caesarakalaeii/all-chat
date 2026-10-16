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

export const settings: LocaleNamespace<'settings'> = {
  ambassador: {
    heading: 'Botschafter',
    body: 'Du bist All-Chat-Botschafter. Entscheide, ob du auf der öffentlichen Startseite gezeigt wirst.',
    featureToggle: 'Zeig mich auf der Startseite',
    cardReads: 'Deine Karte lautet: „{tagline}“',
    toastFeatured: 'Du erscheinst jetzt auf der Startseite',
    toastUnfeatured: 'Aus der Startseiten-Showcase entfernt',
    toastFailed: 'Aktualisieren der Showcase-Einstellung fehlgeschlagen',
  },
  language: {
    heading: 'Sprache',
    body: 'Wähle die Sprache, die All-Chat in diesem Browser verwendet. Öffentliche Seiten und Overlays, die in OBS laufen, bleiben auf Englisch.',
    selectLabel: 'Sprache der Benutzeroberfläche',
    betaNote:
      'Weitere Sprachen sind im Betatest. Betatester und Botschafter können sie jetzt schon wählen.',
    checkFailed:
      'Konnte nicht prüfen, welche Sprachen du verwenden kannst. Englisch ist immer verfügbar.',
  },
  index: {
    heading: 'Einstellungen',
    profileHeading: 'Profil',
    usernameLabel: 'Benutzername',
    primaryPlatformLabel: 'Primäre Plattform',
    primaryPlatformUnknown: 'Unbekannt',
    setupGuideHeading: 'Einrichtungsanleitung',
    setupGuideBody:
      'Gehe die Overlay-Einrichtung noch einmal durch: Overlay erstellen, Chat verbinden, Theme wählen und den OBS-Link holen.',
    setupGuideRestart: 'Neu starten',
    setupGuideRestarting: 'Wird neu gestartet…',
    premiumHeading: 'Premium',
    premiumBody: 'Schalte Premium-Funktionen frei, indem du All-Chat auf Patreon unterstützt.',
    premiumManage: 'Premium verwalten',
    devicesHeading: 'Gekoppelte Geräte',
    devicesBody:
      'Sieh dir die mit deinem Konto verknüpften Stream Deck- und StreamController-Bedienflächen an und entkopple sie. Jede ist auf genau ein Overlay festgelegt.',
    devicesManage: 'Geräte verwalten',
    tokensHeading: 'API-Tokens',
    tokensBody:
      'Erstelle und widerrufe persönliche Access-Tokens. Verwende sie dort, wo Verknüpfen nicht möglich ist – einem headless Capture-Rechner, einem zweiten PC oder einem Skript.',
    tokensManage: 'Tokens verwalten',
    privacyHeading: 'Daten & Datenschutz',
    privacyBody:
      'Wir halten die Datenerfassung minimal und transparent. Wie Tokens, Overlays und Chat-Metadaten verarbeitet werden, erfährst du in den unten verlinkten Richtlinien.',
    privacyPolicyLink: 'Datenschutzerklärung',
    termsLink: 'Nutzungsbedingungen',
    discordHeading: 'Discord',
    discordServersLoading: 'Discord-Server werden geladen',
    discordNoServer: 'Kein Discord-Server verbunden.',
    discordConnectServer: 'Discord-Server verbinden',
    discordConnectAnother: 'Weiteren Server verbinden',
    discordDisconnect: 'Verbindung trennen',
    discordDisconnectTitle: 'Verbindung zu {guild} trennen?',
    discordDisconnectBody: 'Dadurch werden alle mit {guild} verbundenen Discord-Quellen entfernt.',
    discordDisconnectCancel: 'Abbrechen',
    discordDisconnectConfirm: 'Ja, trennen',
    discordAccountHeading: 'Dein Discord-Konto',
    discordAccountLoading: 'Deine Discord-Konto-Verknüpfung wird geladen',
    discordAccountLinked:
      'Verknüpft als {username}. Erforderlich, damit Moderator:innen auf deinen Discord-Servern Aktionen ausführen können.',
    discordAccountFallbackName: 'dein Discord-Konto',
    discordAccountUnlinked:
      'Nicht verknüpft. Verknüpfe es, damit Moderator:innen, die du einlädst, auf deinen Discord-Servern Aktionen ausführen können.',
    discordAccountUnlink: 'Verknüpfung aufheben',
    discordAccountLink: 'Discord-Konto verknüpfen',
    dangerHeading: 'Gefahrenzone',
    dangerBody:
      'Beim Löschen deines Kontos werden alle Overlays, OAuth-Berechtigungen und zwischengespeicherten Chatquellen entfernt. Diese Aktion ist endgültig und kann nicht rückgängig gemacht werden.',
    deleteAccount: 'Konto löschen',
    deleteConfirmTitle: 'Dein Konto löschen?',
    deleteConfirmBody:
      'Dadurch werden dein Konto und alle Overlays endgültig gelöscht. Diese Aktion kann nicht rückgängig gemacht werden.',
    deleteCancel: 'Abbrechen',
    deleteConfirm: 'Ja, mein Konto löschen',
    restartGuideFailedToast: 'Einrichtungsanleitung konnte nicht neu gestartet werden',
    accountDeletedToast: 'Konto gelöscht',
    accountDeleteFailedToast: 'Konto konnte nicht gelöscht werden',
    discordServerConnectedToast: 'Discord-Server verbunden!',
    discordLinkedToast: 'Discord-Konto verknüpft!',
    discordUnlinkedToast: 'Discord-Konto-Verknüpfung aufgehoben',
    discordUnlinkFailedToast: 'Verknüpfung deines Discord-Kontos konnte nicht aufgehoben werden',
    discordDisconnectFailedToast: 'Serververbindung konnte nicht getrennt werden',
  },
  viewer: {
    heading: 'Zuschauer-Identität',
    subheading: 'Passe an, wie dein Name in allen Overlays erscheint',
    signInHeading: 'Melde dich an, um deine Zuschauer-Identität zu verwalten',
    signInBody:
      'Verbinde dein Streaming-Plattform-Konto, um eine eigene Namensfarbe festzulegen und deine Zuschauer-Identität zu verwalten.',
    signInTwitch: 'Mit Twitch anmelden',
    signInYoutube: 'Mit YouTube anmelden',
    signInKick: 'Mit Kick anmelden',
    profileHeading: 'Profil',
    viewerFallbackName: 'Zuschauer',
    nameColorHeading: 'Namensfarbe',
    nameColorBody:
      'Lege eine eigene Farbe oder einen Farbverlauf für deinen Namen auf Overlays fest',
    solidTab: 'Einfarbig',
    gradientTab: 'Verlauf',
    premiumPill: 'Premium',
    gradientUpsell: 'Namen mit Farbverlauf sind eine Zuschauer-Premium-Kosmetik.',
    unlockPremium: 'Zuschauer Premium freischalten',
    colorPickerLabel: 'Namensfarben-Farbpipette',
    colorHexLabel: 'Hex-Wert der Namensfarbe',
    savedFeedback: 'Gespeichert ✓',
    autoSaveNote: 'Änderungen werden automatisch gespeichert',
    previewLabel: 'Vorschau',
    previewMessage: 'Hallo Welt!',
    removeStopLabel: 'Stop {index} entfernen',
    addStop: '+ Stop hinzufügen',
    angleLabel: 'Winkel',
    angleDegreesLabel: 'Winkel in Grad',
    saveGradient: 'Verlauf speichern',
    savingGradient: 'Wird gespeichert…',
    cosmeticsHeading: 'Avatar-Kosmetik',
    cosmeticsBody: 'Wähle einen Rahmen und ein Flair für deinen Avatar',
    cosmeticsUpsell: 'Einige Rahmen und Flairs sind Zuschauer Premium.',
    frameHeading: 'Avatarrahmen',
    flairHeading: 'Avatar-Flair',
    noneItem: 'Keine',
    save: 'Speichern',
    saving: 'Wird gespeichert…',
    premiumRequired: 'Premium erforderlich',
    saveFailed: 'Speichern fehlgeschlagen',
    linkedHeading: 'Verknüpfte Plattformen',
    linkedBody: 'Verbinde weitere Plattformen, um deine Kosmetik in allen deinen Chats zu teilen',
    connected: 'Verbunden',
    connect: 'Verbinden',
    connecting: 'Wird verbunden…',
    disconnect: 'Verbindung trennen',
    disconnecting: 'Verbindung wird getrennt…',
    loadLinkedFailed: 'Verknüpfte Plattformen konnten nicht geladen werden',
    disconnectFailed: 'Plattformverbindung konnte nicht getrennt werden',
  },
  apiTokens: {
    heading: 'API-Tokens',
    subheading: 'Persönliche Access-Tokens für die Stream Deck- und StreamController-Plugins.',
    listHeading: 'Deine Tokens',
    listBody:
      'Gespeichert werden nur die unten aufgeführten Daten – das Token selbst liegt als Hash vor und kann nie wieder angezeigt werden.',
    loadFailed:
      'Deine Tokens konnten nicht geladen werden. Lade die Seite neu und versuche es erneut.',
    revealRegionLabel: 'Neues Token {name}',
    revealHeading: 'Kopiere jetzt dein neues Token',
    revealWarning:
      'Das ist das einzige Mal, dass {name} angezeigt wird. Gespeichert wird nur ein Hash, deshalb kann es nicht erneut angezeigt werden – wenn du es verlierst, widerrufe dieses Token und erstelle ein neues.',
    copyToken: 'Token kopieren',
    copied: 'Kopiert ✓',
    copyFailed: 'Automatisches Kopieren schlug fehl – markiere das Token und kopiere es manuell.',
    dismissReveal: 'Ich habe es gespeichert',
    createHeading: 'Token erstellen',
    createBody:
      'Gib dem Token einen Namen, den du später wiedererkennst, und genehmige ihm nur, was das Gerät braucht.',
    nameLabel: 'Token-Name',
    namePlaceholder: 'Stream Deck (Studio-PC)',
    nameDescription: 'Erscheint in der Liste unten, damit du weißt, was du widerrufen kannst.',
    scopesLegend: 'Scopes',
    noScopesWarning:
      'Wähle mindestens einen Scope – ein Token ohne Scope kann sich zwar authentifizieren, aber nichts tun.',
    create: 'Token erstellen',
    creating: 'Wird erstellt…',
    createFailed: 'Token konnte nicht erstellt werden. Versuche es erneut.',
    scopeChatWriteTitle: 'Chat-Nachrichten senden',
    scopeChatWriteDescription:
      'Erlaubt dem Plugin, Nachrichten in deine verbundenen Chats zu posten.',
    scopeEngagementWriteTitle: 'Umfragen und Vorhersagen ausführen',
    scopeEngagementWriteDescription:
      'Erlaubt dem Plugin, Umfragen und Vorhersagen zu eröffnen, abzuschließen und abzubrechen.',
    emptyHeading: 'Du hast noch keine API-Tokens',
    emptyBody:
      'Ein persönliches Access-Token lässt ein Gerät sich als dich ohne dein Passwort anmelden – so senden die Stream Deck- und StreamController-Plugins Chat-Nachrichten und führen Umfragen und Vorhersagen in deinem Namen aus. Erstelle eines pro Gerät, damit du es einzeln widerrufen kannst.',
    setupGuides: 'Einrichtungsanleitungen:',
    streamDeckReadme: 'Stream Deck-Plugin-README',
    streamControllerReadme: 'StreamController-Plugin-README',
    tokenDates: 'Erstellt {created} · Zuletzt verwendet {lastUsed}',
    neverUsed: 'nie',
    unknownDate: '—',
    revokeLabel: '{name} widerrufen',
    revoke: 'Widerrufen',
    revokeConfirmTitle: 'Dieses Token widerrufen?',
    revokeConfirmBody:
      '„{name}“ funktioniert ab sofort nicht mehr. Jedes Gerät, das es nutzt, braucht ein neues Token.',
    revokeCancel: 'Abbrechen',
    revokeConfirm: 'Token widerrufen',
    revoking: 'Wird widerrufen…',
    revokedToast: '{name} widerrufen',
    revokeFailedToast: 'Dieses Token konnte nicht widerrufen werden',
  },
  devices: {
    heading: 'Gekoppelte Geräte',
    subheading:
      'Stream Deck- und StreamController-Bedienflächen, die mit deinem Konto verknüpft sind. Jede ist auf genau ein Overlay festgelegt und erlischt von selbst, wenn sie nicht mehr benutzt wird.',
    listHeading: 'Deine Geräte',
    listBody:
      'Gespeichert werden nur die unten aufgeführten Daten. Die Zugangsdaten selbst gingen direkt an das Plugin und liegen hier als Hash vor – sie erscheinen nie auf diesem Dashboard, deshalb gibt es auf dieser Seite nichts zu kopieren.',
    loadingLabel: 'Gekoppelte Geräte werden geladen',
    loadFailed:
      'Deine gekoppelten Geräte konnten nicht geladen werden. Lade die Seite neu und versuche es erneut.',
    emptyHeading: 'Noch keine gekoppelten Geräte',
    emptyBody:
      'Verknüpfen beginnt im Plugin, nicht hier: öffne die Einstellungen deines Stream Deck oder StreamController und drücke {linkAction}. Dein Browser öffnet eine Freigabeseite, du wählst ein Overlay, und das Plugin erhält seine Zugangsdaten direkt – nichts wird kopiert oder eingefügt.',
    emptyLinkAction: 'Mit All-Chat verknüpfen',
    setupGuide: 'Einrichtungsanleitung',
    havePairingCode: 'Ich habe einen Kopplungscode',
    controlsOverlay: 'Steuert {overlay} · {status}',
    rowDates: 'Zuletzt verwendet {lastUsed} · Gekoppelt {paired}',
    neverUsed: 'nie',
    unknownDate: '—',
    revokeLabel: '{name} widerrufen',
    revoke: 'Widerrufen',
    statusRevoked: 'Widerrufen {date}',
    statusExpired: 'Abgelaufen {date}',
    statusActive: 'Aktiv bis {date}',
    headlessHeading: 'Auf einem zweiten Rechner oder ohne Desktop?',
    headlessBody:
      'Verknüpfen braucht Plugin und diesen Browser auf demselben Computer. Wenn nicht – ein Stream Deck, das einen Capture-PC steuert, ein Server ohne Desktop – verwende stattdessen ein {tokenLink} oder beginne mit {pairingLink}, falls dein Plugin eines anzeigt.',
    headlessTokenLink: 'persönliches Access-Token',
    headlessPairingLink: 'einen Kopplungscode',
    revokeConfirmTitle: 'Dieses Gerät widerrufen?',
    revokeConfirmBody:
      '„{name}“ funktioniert ab sofort nicht mehr. Verknüpfe es über das Plugin erneut, wenn du es weiter nutzen willst.',
    revokeCancel: 'Abbrechen',
    revokeConfirm: 'Gerät widerrufen',
    revoking: 'Wird widerrufen…',
    revokedToast: '{name} widerrufen',
    revokeFailedToast: 'Dieses Gerät konnte nicht widerrufen werden',
  },
  premium: {
    back: '← Zurück zu den Einstellungen',
    heading: 'Premium',
    connectPitch:
      'Unterstütze All-Chat auf Patreon, um Premium-Funktionen automatisch freizuschalten.',
    notAPatronSuffix: 'Noch kein Patron? {link}, dann verbinde.',
    premiumRow: 'Premium',
    statusExpired: 'Unter der Premium-Stufe',
    notGranting:
      'Dein Patreon ist verknüpft, gewährt aber kein Premium. Stelle sicher, dass deine Patreon-Unterstützung aktiv ist und mindestens der Premium-Stufe entspricht.',
    disconnectBody:
      'Dadurch wird die Verknüpfung mit deinem Patreon-Konto aufgehoben. Das durch dein Abonnement gewährte Premium wird entfernt.',
  },
  viewerPremium: {
    back: '← Zurück zur Zuschauer-Identität',
    heading: 'Zuschauer Premium',
    subheading:
      'Ein günstigeres Abonnement, das Zuschauer-Kosmetik freischaltet – dein Premium-Chat-Badge und dein Namensverlauf – in jedem Overlay, in dem du erscheinst.',
    connectPitch:
      'Unterstütze All-Chat auf Patreon, um deine Zuschauer-Premium-Kosmetik automatisch freizuschalten.',
    notAPatronSuffix: 'Noch kein Patron? {link} (Zuschauer-Stufe ab €2), dann verbinde.',
    premiumRow: 'Zuschauer Premium',
    statusExpired: 'Unter der Zuschauer-Stufe',
    notGranting:
      'Dein Patreon ist verknüpft, gewährt aber kein Zuschauer Premium. Stelle sicher, dass deine Patreon-Unterstützung aktiv ist und mindestens der Zuschauer-Stufe entspricht.',
    disconnectBody:
      'Dadurch wird die Verknüpfung mit deinem Patreon-Konto aufgehoben. Das durch dein Abonnement gewährte Zuschauer Premium wird entfernt.',
    signInHeading: 'Melde dich an, um Zuschauer Premium zu verwalten',
    signInBody:
      'Melde dich mit deinem Streaming-Plattform-Konto an und unterstütze dann All-Chat auf Patreon, um deinen Premium-Chat-Badge und Kosmetik freizuschalten.',
    signInLink: 'Zur Zuschauer-Anmeldung →',
  },
}
