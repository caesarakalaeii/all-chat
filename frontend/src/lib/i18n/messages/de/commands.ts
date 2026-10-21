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
    'Beantworte den Chat automatisch und clippe auf Zuruf. Ein Befehl reagiert, wenn eine Nachricht mit seinem Auslöser beginnt, ein Stichwort, wenn der Auslöser irgendwo in der Nachricht steht. Antworten kommen von deinem eigenen Konto, in dem Chat, aus dem die Nachricht kam.',
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
  },
  list: {
    kindCommand: 'Befehl',
    kindKeyword: 'Stichwort',
    clips: 'Clippt',
    cooldown: '{seconds} s Abklingzeit',
    uses: '{count}× benutzt',
    noReply: 'Keine Antwort, nur Clip',
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
    responseHint: 'Leer lassen, um nur zu clippen. Klicke auf eine Variable, um sie einzufügen.',
    insertVariable: '{token} einfügen',
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
      'Schreibe Code in einen js-Block, um eine Antwort zu berechnen. Der Wert des letzten Ausdrucks wird eingefügt. Die Variablen user, touser, query, args, count, platform, channel, role und clip stehen bereit. Es gibt keinen Netzwerk- oder Dateizugriff, und jede Antwort bekommt 100 ms und 16 MB.',
    jsExample: 'Beispiel',
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
  },
  delete: {
    title: '{trigger} löschen?',
    body: 'Der Befehl antwortet sofort nicht mehr. Sein Zähler geht verloren.',
    cancel: 'Behalten',
    confirm: 'Löschen',
    failed: 'Der Befehl konnte nicht gelöscht werden.',
  },
}
