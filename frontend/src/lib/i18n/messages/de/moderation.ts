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

export const moderation: LocaleNamespace<'moderation'> = {
  actions: {
    delete: 'Nachrichten löschen',
    timeout: 'Timeout',
    ban: 'Bannen',
    unban: 'Entbannen',
  },
  readiness: {
    notAModerator:
      'Noch kein Moderator auf {platform} – füge die Person in den Tools von {platform} selbst hinzu.',
    needsConsent: 'Wartet darauf, dass die Person ihr {platform}-Konto verbindet.',
    needsDiscordLink: 'Wartet darauf, dass die Person Discord verknüpft.',
    unavailable: '{platform} konnte gerade nicht geprüft werden.',
  },
  status: {
    pending: 'Einladung offen',
    suspended: 'Pausiert nach 90 Tagen Inaktivität',
    revoked: 'Entfernt',
    active: 'Aktiv',
  },
  unnamedInvite: 'Unbenannte Einladung',
  roster: {
    loadFailed: 'Die Moderatoren dieses Overlays konnten nicht geladen werden.',
    tryAgain: 'Erneut versuchen',
    loading: 'Lade Moderatoren…',
    explainer:
      'Moderatoren handeln mit {emphasis} Plattform-Konten, daher prüfen Twitch, YouTube und Kick bei jeder Aktion die Moderator-Rolle. Wer hier entfernt wird, verliert die Rechte sofort.',
    explainerEmphasis: 'ihren eigenen',
    inviteButton: 'Moderator einladen',
    usage: '{used} von {cap} belegt',
    removeAll: 'Alle entfernen',
    atCap:
      'Dieses Overlay hat sein Limit von {cap} Moderatoren erreicht. Entferne jemanden, um eine weitere Einladung zu senden.',
    empty:
      'Auf diesem Overlay moderiert noch niemand. Lade jemanden ein – die Person kann dann Nachrichten löschen und Zuschauer aus der Monitor-Ansicht heraus timeouts, und zwar mit ihrem eigenen Plattform-Konto.',
  },
  row: {
    legChangeFailed: '{platform} konnte für {name} nicht geändert werden.',
    needsOneAction: '{name} braucht mindestens eine Aktion. Entferne die Person stattdessen.',
    updateFailed: '{name} konnte nicht aktualisiert werden.',
    removed: '{name} entfernt.',
    removeFailed: '{name} konnte nicht entfernt werden.',
    removeAllFailed: 'Alle konnten nicht entfernt werden. Versuche es erneut.',
    removeButton: 'Entfernen',
    removeLabel: '{name} entfernen',
    forAccount: 'für {account}',
    platformToggleLabel: '{platform}-Moderation für {name}',
    removedCountOne: '{count} Moderator entfernt.',
    removedCountMany: '{count} Moderatoren entfernt.',
  },
  revoke: {
    title: '{name} entfernen?',
    description:
      'Die Person verliert beim nächsten Zugriff den Moderations-Zugang zu diesem Overlay. Frühere Aktionen bleiben im Protokoll.',
    cancel: 'Abbrechen',
    confirm: 'Entfernen',
  },
  revokeAll: {
    title: 'Alle Moderatoren entfernen?',
    description:
      'Damit werden alle auf diesem Overlay entfernt, auch Einladungen, die noch niemand angenommen hat. Neue Einladungen sind danach jederzeit möglich.',
    confirm: 'Alle entfernen',
  },
  invite: {
    title: 'Moderator einladen',
    sendLink:
      'Schicke diesen Link an die Person, die moderieren soll. Er funktioniert einmal, läuft nach 7 Tagen ab und {emphasis} – falls er verloren geht, erstelle eine neue Einladung.',
    sendLinkEmphasis: 'wird nicht erneut angezeigt',
    copied: 'Kopiert',
    copyLink: 'Link kopieren',
    done: 'Fertig',
    acceptExplainer:
      'Die Person nimmt mit ihrem eigenen All-Chat-Konto an und verbindet jede Plattform, sobald sie dort zum ersten Mal moderiert.',
    labelPrompt: 'Für wen ist die Einladung? (optional)',
    labelPlaceholder: 'Sarah, mein Twitch-Mod',
    actionsLegend: 'Was darf die Person tun?',
    platformsLegend: 'Auf welchen Plattformen? Deaktiviert heißt: keine Aktionen dort.',
    cancel: 'Abbrechen',
    creating: 'Wird erstellt…',
    create: 'Einladung erstellen',
    capReached: 'Dieses Overlay hat bereits die höchstzulässige Anzahl an Moderatoren.',
    createFailed: 'Die Einladung konnte nicht erstellt werden. Versuche es erneut.',
    boundToAccount: 'Diese Einladung gehört zu {account}.',
    copyFailed: 'Kopieren fehlgeschlagen. Markiere den Link und kopiere ihn manuell.',
    premiumGate:
      'Moderation delegieren ist Teil von All-Chat Premium. Deine Moderatoren zahlen nie etwas – es zählt nur dein eigener Tarif.',
    upgradeLink: 'Upgrade holen, um Moderatoren einzuladen',
  },
  accept: {
    heading: 'Moderations-Einladung',
    loadingInvite: 'Lade Einladung',
    goToChannels: 'Zu deinen Kanälen',
    accept: 'Annehmen und moderieren',
    accepting: 'Wird angenommen…',
    notNow: 'Nicht jetzt',
    actionDelete: 'Nachrichten löschen',
    actionTimeout: 'Zuschauer timeouten',
    actionBan: 'Zuschauer bannen',
    actionUnban: 'Bans und Timeouts aufheben',
    signInHeading: 'Melde dich an, um diese Einladung anzunehmen',
    signInBody:
      'Moderation ist an ein All-Chat-Konto gebunden, daher müssen wir wissen, welches gemeint ist. Melde dich an und öffne den Einladungslink erneut – er bleibt gültig.',
    signIn: 'Anmelden',
    askingToHelp: '{owner} bittet dich, beim Moderieren zu helfen',
    addressedTo: 'Die Einladung ist für „{label}“ ausgestellt.',
    actionsHeading: 'Was du tun können wirst',
    platformsHeading: 'Auf diesen Plattformen',
    noPlatforms: 'Noch keine – {owner} muss eine Plattform erst freischalten.',
    ownAccountNote:
      'Du handelst mit deinem eigenen Plattform-Konto, daher prüft jede Plattform weiterhin, ob {owner} dich dort als Moderator eingebunden hat. Jetzt wird nichts von dir verlangt – du verbindest eine Plattform, sobald du dort zum ersten Mal moderierst.',
    expectedAccount: 'Diese Einladung ist für {account} gedacht.',
    expectedAccountOnPlatform: 'Diese Einladung ist für das {platform}-Konto {account} gedacht.',
    errorMissingToken:
      'In diesem Link fehlt der Einladungscode. Bitte die Person, die den Stream betreibt, dir den Link erneut zu schicken.',
    errorNotFound:
      'Diese Einladung ist nicht mehr gültig – sie wurde eventuell bereits benutzt oder vom Streamer zurückgezogen. Bitte um eine neue.',
    errorExpired:
      'Diese Einladung ist abgelaufen. Bitte die Person, die den Stream betreibt, um eine neue.',
    errorAlreadyModerator: 'Du moderierst diesen Kanal bereits. Er steht auf deiner Kanäle-Seite.',
    errorOwnerCannotAccept:
      'Das ist dein eigenes Overlay – dort hast du ohnehin die volle Moderation.',
    errorBoundToOther:
      'Diese Einladung gilt für ein bestimmtes Konto. Melde dich mit diesem Konto an oder bitte die Person, die den Stream betreibt, dir eine neue Einladung dafür zu schicken.',
    errorBoundToOtherAccount:
      'Diese Einladung gilt für ein bestimmtes Konto ({account}). Melde dich mit diesem Konto an oder bitte die Person, die den Stream betreibt, dir eine neue Einladung dafür zu schicken.',
    errorBoundToOtherPlatform:
      'Diese Einladung gilt für ein bestimmtes {platform}-Konto. Melde dich mit diesem Konto an oder bitte die Person, die den Stream betreibt, dir eine neue Einladung dafür zu schicken.',
    errorBoundToOtherBoth:
      'Diese Einladung gilt für ein bestimmtes {platform}-Konto ({account}). Melde dich mit diesem Konto an oder bitte die Person, die den Stream betreibt, dir eine neue Einladung dafür zu schicken.',
    errorUnknown:
      'Diese Einladung konnte nicht geöffnet werden. Prüfe den Link und versuche es erneut.',
  },
  channels: {
    heading: 'Kanäle, in denen du moderierst',
    subheading:
      'Overlays, die andere Streamer an dich übertragen haben. Du handelst mit deinem eigenen Plattform-Konto, daher prüft jede Plattform, ob du zu ihren Moderatoren gehörst.',
    loading: 'Lade Kanäle',
    loadFailed: 'Deine Kanäle konnten nicht geladen werden.',
    tryAgain: 'Erneut versuchen',
    emptyHeading: 'Noch keine Kanäle',
    emptyBody:
      'Lädt dich eine Person zum Mitmoderieren ein, schickt sie dir einen privaten Link. Öffne ihn, während du mit diesem Konto angemeldet bist – der Kanal erscheint dann hier.',
    forOwner: 'für {owner}',
    noPlatforms: 'Noch keine Plattformen aktiv – bitte {owner}, eine freizuschalten.',
    suspendedNote: 'Pausiert nach 90 Tagen ohne Aktionen. Bitte {owner}, sie wieder einzuschalten.',
    unavailableNote:
      'Der Tarif von {owner} enthält zurzeit keine Moderation. Aktionen sind daher erst möglich, wenn die Person ihn verlängert.',
    openMonitor: 'Chat-Monitor öffnen',
    discordPromptBody:
      'Verknüpfe dein Discord-Konto, um auf Discord zu moderieren. All-Chat prüft zuerst deine eigenen Server-Berechtigungen, daher muss es wissen, welches Discord-Konto dir gehört.',
    linkDiscord: 'Discord verknüpfen',
    noticeDiscordAlreadyLinked:
      'Dieses Discord-Konto ist bereits mit einem anderen All-Chat-Konto verknüpft. Verknüpfe ein anderes oder löse es dort zuerst.',
    noticeConnectFailed:
      'Diese Verbindung wurde nicht abgeschlossen. Öffne unten einen Kanal und versuche es dort erneut.',
    noticeDiscordLinked:
      'Discord-Konto verknüpft. All-Chat kann jetzt beim Discord-Moderieren deine eigenen Server-Berechtigungen prüfen.',
    noticeConnected:
      '{platform} verbunden. Es deckt jetzt jeden Kanal ab, der dir {platform} delegiert hat.',
  },
}
