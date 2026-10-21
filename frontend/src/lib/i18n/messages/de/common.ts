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

export const common: LocaleNamespace<'common'> = {
  brand: {
    wordmark: 'all-chat',
  },
  appNav: {
    dashboard: 'Dashboard',
    flairs: 'Flairs',
    admin: 'Admin',
    settings: 'Einstellungen',
    docs: 'Docs',
    translate: 'Übersetzen',
    commands: 'Befehle',
    logOut: 'Abmelden',
    signIn: 'Anmelden',
  },
  soundPresets: {
    chime: 'Chime',
    pop: 'Pop',
    ping: 'Ping',
  },
  platforms: {
    twitch: 'Twitch',
    youtube: 'YouTube',
    kick: 'Kick',
    tiktok: 'TikTok',
    owncast: 'Owncast',
    goodgame: 'GoodGame',
    picarto: 'Picarto',
    facebook: 'Facebook',
    instagram: 'Instagram',
    discord: 'Discord',
  },
  overlayKinds: {
    chat: 'Chat',
    alerts: 'Alerts',
    goal: 'Goal',
    list: 'Liste',
  },
  patreon: {
    heading: 'Patreon',
    connect: 'Patreon verbinden',
    connecting: 'Weiterleitung…',
    notAPatron: 'Noch kein Patron?',
    subscribe: 'Auf Patreon abonnieren',
    subscriptionRow: 'Abonnement',
    renewsRow: 'Verlängert sich',
    active: 'Aktiv',
    inactive: 'Inaktiv',
    disconnect: 'Patreon trennen',
    disconnectTitle: 'Patreon trennen?',
    disconnectCancel: 'Abbrechen',
    disconnectConfirm: 'Ja, trennen',
    statusActive: 'Aktiv',
    statusDeclined: 'Zahlung abgelehnt',
    statusFormer: 'Beendet',
    statusNotSubscribed: 'Nicht abonniert',
    connectedToast: 'Patreon verbunden!',
    connectFailedToast: 'Patreon konnte nicht verbunden werden',
    connectStartFailedToast: 'Patreon-Verbindung konnte nicht gestartet werden',
    disconnectedToast: 'Patreon getrennt',
    disconnectFailedToast: 'Trennen fehlgeschlagen',
  },
  duration: {
    justNow: 'gerade eben',
    minutes: '{minutes}m',
    hours: '{hours}h',
    hoursAndMinutes: '{hours}h {minutes}m',
    days: '{days}T',
    daysAndHours: '{days}T {hours}h',
  },
  badges: {
    allChatLabel: 'All-Chat-Badge',
    premiumLabel: 'Premium-Badge',
  },
  dialog: {
    closeLabel: 'Dialog schließen',
  },
  toast: {
    closeLabel: 'Benachrichtigung schließen',
    tryAgain: 'Bitte versuche es erneut.',
    elevenLabsFallback: 'ElevenLabs nicht verfügbar – Browser-Stimme wird verwendet.',
  },
  splitPane: {
    resizeLabel: 'Größe der Bereiche anpassen',
    shrinkConfigLabel: 'Konfigurationsbereich verkleinern',
    growConfigLabel: 'Konfigurationsbereich vergrößern',
    previewTitle: 'Live-Vorschau des Overlays',
  },
  cssEditor: {
    regionLabel: 'Editor für benutzerdefiniertes CSS',
    keyboardHint:
      'Drücke Ctrl+M, um die Tab-Erfassung umzuschalten; mit Escape und dann Tab verlässt du den Editor.',
    loading: 'Editor wird geladen...',
  },
  impersonation: {
    bannerLabel: 'Admin-Modus:',
    viewingAs: 'Ansicht als {username}',
    exitButton: 'Beenden & zurück zum Admin',
  },
  moderatingElsewhere: {
    channelOne: '{count} Kanal',
    channelMany: '{count} Kanäle',
    sentence: 'Du moderierst {channels} für andere Streamer.',
    openLink: 'Öffnen',
  },
  maintenanceInfo: {
    buttonLabel: 'Service-Ankündigungen',
    popoverHeading: 'Service-Ankündigungen',
  },
  eventSubMigration: {
    body: 'Die alte IRC-Chatverbindung wird eingestellt und kann Nachrichten verlieren, wenn viele Streams live sind. Stelle die Verbindung neu her, um auf die neue Verbindung zu wechseln und deinen Chat zuverlässig zu halten.',
    reconnectButton: 'Jetzt neu verbinden',
    dismissLabel: 'Ausblenden',
    connectedToast: 'Twitch-Chat verbunden',
    failedToast: 'Upgrade konnte nicht gestartet werden',
  },
  forbidden: {
    heading: '403 Forbidden',
    body: 'Du hast keine Berechtigung, auf diese Seite zuzugreifen. Dafür sind Admin-Rechte erforderlich.',
    dashboardButton: 'Zum Dashboard',
  },
  betaWarning: {
    youtubeTitle: 'YouTube – OAuth-Verifizierung läuft',
    tiktokTitle: 'TikTok – Geschlossene Beta',
    youtubeBody:
      'Die YouTube-Integration wird derzeit im Rahmen der Google-OAuth-Verifizierung geprüft. In dieser Zeit können keine neuen Testnutzer hinzugefügt werden.',
    tiktokBody:
      'Die TikTok-Integration befindet sich derzeit in der geschlossenen Beta. Wenn du noch nicht in das Beta-Programm aufgenommen wurdest, schlägt die Authentifizierung fehl.',
    youtubeDiscordPrompt:
      'Tritt unserer Discord-Community bei, um auf dem Stand der Verifizierung zu bleiben und Support zu erhalten:',
    tiktokDiscordPrompt: 'Um an der Beta teilzunehmen, tritt unserer Discord-Community bei:',
    youtubeExistingUser:
      'Wenn du bereits als Testnutzer hinzugefügt wurdest, kannst du die YouTube-Integration weiterhin nutzen.',
    tiktokExistingUser:
      'Wenn du bereits in der Beta bist, kannst du die Authentifizierung durchführen.',
    discordLink: 'Discord-Server beitreten',
    cancelButton: 'Abbrechen',
    continueButton: 'Ich habe verstanden, weiter',
  },
  breadcrumb: {
    label: 'Navigationspfad',
    home: 'Startseite',
  },
}
