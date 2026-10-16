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

export const metadata: LocaleNamespace<'metadata'> = {
  socialCard: {
    alt: 'All-Chat — Jeder Chat. Eine URL.',
    emoteProviders: '7TV + BTTV + FFZ EMOTES',
  },
  site: {
    titleDefault: 'All-Chat — Jeder Chat. Ein Overlay.',
    titleTemplate: '%s | All-Chat',
    description:
      'Sieh den Chat von Twitch, YouTube, Kick, TikTok und Discord in einem OBS-Chat-Overlay. Einfach in OBS einfügen und loslegen. 7TV-, BTTV- und FFZ-Emotes eingebaut. Kostenlos und Open Source.',
    socialTitle: 'All-Chat — Jeder Chat. Ein Overlay.',
    socialDescription:
      'Den gesamten Chat von Twitch, YouTube, Kick, TikTok und Discord in einem OBS-Overlay. 7TV-, BTTV- und FFZ-Emotes eingebaut.',
  },
  home: {
    title: 'Multi-Plattform-Chat-Overlay für Twitch, YouTube, Kick & TikTok | All-Chat',
    description:
      'Kostenloses Multi-Plattform-Chat-Overlay für OBS. Führe den Chat von Twitch, YouTube, Kick, TikTok und Discord in einem Overlay zusammen, mit 7TV-, BTTV- und FFZ-Emotes eingebaut. Open Source, keine Installation.',
  },
  docs: {
    title: 'Dokumentation',
    description:
      'Bekomme dein All-Chat-Overlay in OBS zum Laufen, wähle aus 16 eingebauten Themes und mache es zu deinem eigenen — ohne CSS zum Start, mit voller CSS-Kontrolle, wenn du willst.',
  },
  docsApi: {
    title: 'Entwickler-API',
    description:
      'Verbinde Drittanbieter-Tools mit dem vereinheitlichten Chat-WebSocket-Stream von All-Chat: Nachrichtenformat, Plattform-Events, Statusmeldungen und Wiederverbindung (Twitch, YouTube, Kick, TikTok, Discord).',
  },
  upgrade: {
    title: 'Upgrade auf Premium',
    description:
      'Unterstütze All-Chat auf Patreon und schalte Premium-Funktionen frei: moderieren aus deinem Overlay, ElevenLabs-TTS, YouTube-Stream-Auswahl, geteilter Chat und Zuschauer-Flairs.',
  },
  overlay: {
    title: 'Overlay',
    description: 'Chat-Overlay für die OBS-Browser-Quelle',
  },
  overlayMonitor: {
    title: 'Chat-Monitor',
    description: 'Lesbarer Chat- und Aktivitätsmonitor für Streamer',
  },
  impressum: {
    title: 'Impressum',
    description: 'Rechtliche Hinweise (Impressum) gemäß § 5 DDG.',
  },
  privacy: {
    title: 'Datenschutzerklärung',
    description: 'Erfahre, wie All-Chat deine Informationen erhebt, verarbeitet und schützt.',
  },
  terms: {
    title: 'Nutzungsbedingungen',
    description: 'Verstehe die Regeln und Pflichten bei der Nutzung von All-Chat.',
  },
}
