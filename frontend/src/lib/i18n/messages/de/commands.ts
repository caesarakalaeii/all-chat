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

export const commands: LocaleNamespace<'commands'> = {
  heading: 'Chat-Befehle',
  intro:
    'Beantworte den Chat automatisch, clippe auf Zuruf und zeige Bilder im Stream. Ein Befehl reagiert, wenn eine Nachricht mit seinem Auslöser beginnt, ein Stichwort, wenn der Auslöser irgendwo in der Nachricht steht. Antworten landen in dem Chat, aus dem die Nachricht kam, gesendet vom All-Chat-Bot oder, wenn du willst, von deinem eigenen Konto.',
  newCommand: 'Neuer Befehl',
  loading: 'Chat-Befehle werden geladen',
  loadFailed: 'Deine Chat-Befehle konnten nicht geladen werden.',
  retry: 'Erneut versuchen',
  empty: 'Noch keine Befehle. Starte mit einem Beispiel oder lege einen eigenen an.',
  examplesHeading: 'Mit einem Beispiel starten',
  examples: {
    hello: 'Begrüßung',
    clip: 'Clip auf Zuruf',
    dice: 'Würfelwurf',
    shoutout: 'Shoutout (Mods)',
    dog: 'Zufälliger Hund',
  },
  list: {
    kindCommand: 'Befehl',
    kindKeyword: 'Stichwort',
    clips: 'Clippt',
    image: 'Bild',
    cooldown: '{seconds} s Abklingzeit',
    uses: '{count}× benutzt',
    noReply: 'Keine Antwort im Chat',
    edit: 'Bearbeiten',
    editLabel: '{trigger} bearbeiten',
    delete: 'Löschen',
    deleteLabel: '{trigger} löschen',
    enabledLabel: '{trigger} aktivieren',
    toggleFailed: 'Der Befehl konnte nicht geändert werden.',
  },
  roles: {
    everyone: 'Alle',
    subscriber: 'Abonnenten und höher',
    vip: 'VIPs und höher',
    moderator: 'Moderatoren und du',
    broadcaster: 'Nur du',
  },
  status: {
    heading: 'Wo Befehle antworten können',
    twitchNoLogin: 'Für Antworten und Clips auf Twitch musst du dich mit Twitch anmelden.',
    twitchReplies: 'Antworten im Twitch-Chat',
    twitchClips: 'Twitch-Clips',
    allowed: 'Erlaubt',
    missing: 'Berechtigung fehlt',
    grant: 'Auf Twitch erlauben',
    granting: 'Twitch wird geöffnet',
    grantFailed: 'Die Twitch-Berechtigung konnte nicht gestartet werden.',
    grantNeedsOverlay:
      'Füge deinen Twitch-Kanal zuerst einem deiner Overlays hinzu und erlaube es dann hier.',
    youtubePremium: 'Antworten im YouTube-Chat sind aktiv: Du hast Premium.',
    youtubeLocked:
      'Antworten im YouTube-Chat sind Premium, weil jede davon YouTube-API-Kontingent verbraucht. Clips aus dem YouTube-Chat funktionieren trotzdem.',
    upgrade: 'Premium ansehen',
    kick: 'Antworten im Kick-Chat nutzen die Chat-Berechtigung aus den erweiterten Steuerelementen eines Overlays.',
    others:
      'TikTok, Discord und die Beta-Plattformen können Clips auslösen, aber Befehle können dort nicht antworten.',
  },
  editor: {
    createTitle: 'Neuer Chat-Befehl',
    editTitle: '{trigger} bearbeiten',
    kindLabel: 'Reagiert, wenn',
    kindCommand: 'eine Nachricht damit beginnt',
    kindKeyword: 'eine Nachricht es enthält',
    triggerLabel: 'Auslöser',
    triggerHintCommand:
      'Ein Wort, zum Beispiel !hallo. Groß- und Kleinschreibung spielt keine Rolle.',
    triggerHintKeyword:
      'Ein Wort oder eine Phrase, zum Beispiel clip das. Wird überall in einer Nachricht erkannt.',
    responseLabel: 'Antwort',
    responseHint:
      'Leer lassen, um nur zu clippen oder ein Bild zu zeigen. Klicke auf eine Variable, um sie einzufügen.',
    insertVariable: '{token} einfügen',
    imageLabel: 'Bild im Stream',
    imageHint:
      'Ein https-Link zu einem Bild oder GIF. Es erscheint auf deinen Alert-Overlays, sobald der Befehl auslöst, egal aus welchem Chat. Vorlagen funktionieren hier, aber was Zuschauer tippen, erreicht das Bild nie.',
    imagePreviewAlt: 'Bildvorschau',
    clipLabel: 'Twitch-Clip erstellen',
    clipHint:
      'Clippt deinen Twitch-Livestream. Setze {token} in die Antwort, um den Link zu posten.',
    roleLabel: 'Wer ihn benutzen darf',
    cooldownLabel: 'Abklingzeit (Sekunden)',
    cooldownHint: 'So lange ignoriert der Befehl den Chat, nachdem er ausgelöst hat.',
    enabledLabel: 'Aktiv',
    previewHeading: 'Vorschau',
    previewInputLabel: 'Ein Zuschauer tippt nach dem Auslöser',
    previewInputPlaceholder: '@freund ein paar Wörter',
    previewEmpty: 'Hier erscheint die Antwort.',
    previewFailed: 'Vorschau nicht verfügbar.',
    cancel: 'Abbrechen',
    save: 'Speichern',
    saving: 'Wird gespeichert',
    saveFailed: 'Der Befehl konnte nicht gespeichert werden.',
    saved: 'Befehl gespeichert',
    referenceHeading: 'Vorlagen-Referenz',
    jsHeading: 'JavaScript',
    jsBody:
      'Schreibe Code in einen js-Block, um eine Antwort zu berechnen. Der Wert des letzten Ausdrucks wird eingefügt. Die Variablen user, touser, query, args, count, platform, channel, role und clip stehen bereit. Der Code selbst hat keinen Netzwerk- oder Dateizugriff, und jede Antwort bekommt 100 ms und 16 MB.',
    jsExample: 'Beispiel',
    urlfetchHeading: 'Eine Webadresse abrufen',
    urlfetchBody:
      '{token} fügt den Text ein, mit dem eine Webadresse antwortet. In einem js-Block kommt er als String an, bereit für JSON.parse. Nur öffentliche https-Adressen funktionieren, höchstens 3 pro Antwort und je 3 Sekunden. Variablen in der Adresse werden maskiert, damit Zuschauer nicht ändern können, wohin sie zeigt.',
  },
  reference: {
    user: 'Name der Person, die den Befehl benutzt hat',
    touser: 'Erstes Wort nach dem Auslöser ohne @, sonst die Person selbst',
    query: 'Alles nach dem Auslöser; bei einem Stichwort die ganze Nachricht',
    args: 'Einzelne Wörter nach dem Auslöser',
    count: 'Wie oft der Befehl schon ausgelöst hat',
    random: 'Eine Zufallszahl von 1 bis 100 oder aus deinem Bereich',
    pick: 'Eine der Optionen, zufällig',
    time: 'Die Uhrzeit (24 h) in einer Zeitzone; ohne Angabe UTC',
    where: 'Plattform und Kanal, aus denen die Nachricht kam',
    clip: 'Link zum Clip, den dieser Befehl erstellt hat',
    urlfetch: 'Der Text, mit dem eine Webadresse antwortet',
  },
  delete: {
    title: '{trigger} löschen?',
    body: 'Der Befehl antwortet sofort nicht mehr. Sein Zähler geht verloren.',
    cancel: 'Behalten',
    confirm: 'Löschen',
    failed: 'Der Befehl konnte nicht gelöscht werden.',
  },

  bot: {
    heading: 'Bot und Absender',
    senderBot: 'Antworten kommen vom All-Chat-Bot',
    senderBotHint: 'Funktioniert auf Twitch, Kick und YouTube ohne separate Anmeldung.',
    senderSelf: 'Antworten kommen von deinem eigenen Konto',
    senderSelfHint:
      'Die Nachricht erscheint, als hättest du sie selbst getippt. Benötigt die Chat-Sendeberechtigung.',
    senderToggleLabel: 'Umschalten, wer antwortet: All-Chat-Bot oder dein Konto',
    senderSaveFailed: 'Der Absender konnte nicht geändert werden.',
    twitchGranted: 'Der All-Chat-Bot ist für deinen Twitch-Chat zugelassen.',
    twitchNotGranted:
      'Der All-Chat-Bot braucht deine Erlaubnis, um in deinen Twitch-Chat zu schreiben.',
    grant: 'Im Twitch-Chat erlauben',
    granting: 'Twitch wird geöffnet',
    grantFailed: 'Die Twitch-Berechtigung konnte nicht gestartet werden.',
    connectedToast: 'Bot verbunden.',
    connectFailedToast: 'Das Verbinden des Bots ist fehlgeschlagen.',
    customHeading: 'Eigene Bot-Konten',
    customIntro:
      'Verbinde pro Plattform dein eigenes Bot-Konto. Antworten erscheinen unter diesem Namen.',
    customPremium:
      'Eigene Bot-Konten sind eine Premium-Funktion. Sieh dir Premium an, um auf Twitch, Kick oder YouTube deinen eigenen Bot zu verbinden.',
    noneConnected: 'Kein eigener Bot verbunden',
    connectedAs: '{name} ({login})',
    connect: '{platform}-Bot verbinden',
    connecting: 'Wird verbunden',
    connectFailed: 'Die Bot-Verbindung konnte nicht gestartet werden.',
    disconnect: 'Trennen',
    disconnecting: 'Wird getrennt',
    disconnectLabel: 'Den {platform}-Bot trennen',
    disconnected: 'Bot getrennt.',
    disconnectFailed: 'Das Trennen des Bots ist fehlgeschlagen.',
    removeTitle: 'Den {platform}-Bot trennen?',
    removeBody:
      'Antworten auf dieser Plattform gehen zurück an den All-Chat-Bot oder dein eigenes Konto.',
    removeCancel: 'Behalten',
    removeConfirm: 'Trennen',
  },

  timers: {
    heading: 'Timer',
    intro:
      'Timer posten eine Nachricht in deinen Twitch- und Kick-Chat, mit Premium auch in den YouTube-Chat. Lege ein Intervall in Minuten, eine Mindestanzahl an Chatnachrichten oder beides fest: Bei beidem feuert er alle X Minuten, aber nur, wenn seit dem letzten Post mindestens Y Nachrichten geschrieben wurden. Ein Timer nur mit Intervall postet auch, während du offline bist.',
    new: 'Neuer Timer',
    empty: 'Noch keine Timer.',
    edit: 'Bearbeiten',
    delete: 'Löschen',
    deleteLabel: '{name} löschen',
    enabledLabel: '{name} aktivieren',
    toggleFailed: 'Der Timer konnte nicht geändert werden.',
    deleteFailed: 'Der Timer konnte nicht gelöscht werden.',
    saved: 'Timer gespeichert',
    createTitle: 'Neuer Timer',
    editTitle: '{name} bearbeiten',
    dialogIntro: 'Ein Timer postet seine Nachricht nach Zeitplan oder auf Chataktivität hin.',
    nameLabel: 'Name',
    nameRequired: 'Vergib einen Namen für den Timer.',
    messageLabel: 'Nachricht',
    messageHint:
      'Was gepostet wird. Vorlagenvariablen ohne Chatter funktionieren, etwa count, random, pick, time und js.',
    messageRequired: 'Schreibe die Nachricht, die gepostet werden soll.',
    intervalLabel: 'Intervall (Minuten, optional)',
    intervalHint: 'Zwischen {min} und {max} Minuten. Leer lassen für kein Intervall.',
    messagesLabel: 'Mindestnachrichten (optional)',
    messagesHint: 'Zwischen {min} und {max} Nachrichten. Leer lassen für kein Minimum.',
    combinedRule:
      'Lege mindestens eines von beiden fest. Bei beidem wartet der Timer auf das Intervall und auf die Mindestzahl an Nachrichten seit seinem letzten Post.',
    triggerRequired: 'Lege ein Intervall, eine Mindestanzahl an Nachrichten oder beides fest.',
    intervalRange: 'Das Intervall muss eine ganze Zahl zwischen {min} und {max} Minuten sein.',
    messagesRange: 'Das Minimum muss eine ganze Zahl zwischen {min} und {max} Nachrichten sein.',
    triggerBoth: 'alle {minutes} Minuten, mindestens {messages} Nachrichten seit der letzten',
    triggerEvery: 'alle {minutes} Minuten',
    triggerMessages: 'nach {messages} neuen Nachrichten',
    cancel: 'Abbrechen',
    save: 'Speichern',
    saving: 'Wird gespeichert',
    saveFailed: 'Der Timer konnte nicht gespeichert werden.',
    deleteTitle: '{name} löschen?',
    deleteBody: 'Der Timer postet sofort nicht mehr.',
    deleteCancel: 'Behalten',
    deleteConfirm: 'Löschen',
  },
}
