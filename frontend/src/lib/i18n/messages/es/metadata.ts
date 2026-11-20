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
    alt: 'All-Chat — Todos los chats. Una URL.',
    emoteProviders: 'EMOTES DE 7TV + BTTV + FFZ',
  },
  site: {
    titleDefault: 'All-Chat — Todos los chats. Un overlay.',
    titleTemplate: '%s | All-Chat',
    description:
      'Ve todos tus chats de Twitch, YouTube, Kick, TikTok y Discord en un solo overlay de chat para OBS. Añádelo a OBS y listo. Emotes de 7TV, BTTV y FFZ integrados. Gratis y de código abierto.',
    socialTitle: 'All-Chat — Todos los chats. Una URL.',
    socialDescription:
      'Overlay de chat multistream y dock de OBS gratuitos para Twitch, YouTube, Kick, TikTok y Discord. Emotes de 7TV, BTTV y FFZ integrados.',
  },
  home: {
    title: 'Overlay de chat multiplataforma para Twitch, YouTube, Kick y TikTok | All-Chat',
    description:
      'Overlay de chat multiplataforma gratuito para OBS. Une tus chats de Twitch, YouTube, Kick, TikTok y Discord en un solo overlay, con emotes de 7TV, BTTV y FFZ integrados. Código abierto, sin instalación.',
  },
  docs: {
    title: 'Documentación',
    description:
      'Pon tu overlay de All-Chat en directo en OBS, elige entre 16 temas integrados y hazlo tuyo: no hace falta CSS para empezar y tienes control total con CSS cuando lo quieras.',
  },
  docsApi: {
    title: 'API para desarrolladores',
    description:
      'Conecta herramientas de terceros al stream WebSocket de chat unificado de All-Chat: formato de mensajes, eventos de plataforma, mensajes de estado y reconexión (Twitch, YouTube, Kick, TikTok, Discord).',
  },
  // Route titles stay bare: the layout's titleTemplate appends the brand.
  upgrade: {
    title: 'Mejorar a Premium',
    description:
      'Apoya a All-Chat en Patreon para desbloquear las funciones premium: moderar desde tu overlay, TTS de ElevenLabs, selección de directo de YouTube, chat compartido y flairs de espectador.',
  },
  overlay: {
    title: 'Overlay',
    description: 'Overlay de chat para la fuente de navegador de OBS',
  },
  overlayMonitor: {
    title: 'Monitor de chat',
    description: 'Monitor de chat y actividad legible para streamers',
  },
  impressum: {
    title: 'Impressum',
    description: 'Aviso legal (Impressum) conforme al § 5 DDG.',
  },
  privacy: {
    title: 'Política de privacidad',
    description: 'Descubre cómo All-Chat recoge, trata y protege tu información.',
  },
  terms: {
    title: 'Términos del servicio',
    description: 'Conoce las normas y responsabilidades del uso de All-Chat.',
  },
}
