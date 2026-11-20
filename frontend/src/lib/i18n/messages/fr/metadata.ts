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
    alt: 'All-Chat — Tous les chats. Une seule URL.',
    emoteProviders: 'EMOTES 7TV + BTTV + FFZ',
  },
  site: {
    titleDefault: 'All-Chat — Tous les chats. Un seul overlay.',
    titleTemplate: '%s | All-Chat',
    description:
      "Regroupez vos chats Twitch, YouTube, Kick, TikTok et Discord dans un seul overlay de chat OBS. Ajoutez-le à OBS et c'est parti. Emotes 7TV, BTTV et FFZ intégrées. Gratuit et open source.",
    socialTitle: 'All-Chat — Tous les chats. Une seule URL.',
    socialDescription:
      'Overlay de chat multistream et dock OBS gratuits pour Twitch, YouTube, Kick, TikTok et Discord. Emotes 7TV, BTTV et FFZ intégrées.',
  },
  home: {
    title: 'Overlay de chat multiplateforme pour Twitch, YouTube, Kick et TikTok | All-Chat',
    description:
      'Overlay de chat multiplateforme gratuit pour OBS. Fusionnez vos chats Twitch, YouTube, Kick, TikTok et Discord dans un seul overlay, avec les emotes 7TV, BTTV et FFZ intégrées. Open source, sans installation.',
  },
  docs: {
    title: 'Documentation',
    description:
      'Mettez votre overlay All-Chat en service dans OBS, choisissez parmi 16 thèmes intégrés et personnalisez-le : aucun CSS requis pour démarrer, contrôle CSS complet quand vous le souhaitez.',
  },
  docsApi: {
    title: 'API développeur',
    description:
      "Connectez des outils tiers au flux WebSocket de chat unifié d'All-Chat : format des messages, événements de plateforme, messages de statut et reconnexion (Twitch, YouTube, Kick, TikTok, Discord).",
  },
  upgrade: {
    title: 'Passer à Premium',
    description:
      'Soutenez All-Chat sur Patreon pour débloquer les fonctionnalités premium : modération depuis votre overlay, TTS ElevenLabs, sélection du stream YouTube, chat partagé et flairs de spectateur.',
  },
  overlay: {
    title: 'Overlay',
    description: 'Overlay de chat pour la source navigateur OBS',
  },
  overlayMonitor: {
    title: 'Moniteur de chat',
    description: "Moniteur de chat et d'activité lisible pour les streamers",
  },
  impressum: {
    title: 'Impressum',
    description: 'Mentions légales (Impressum) conformément au § 5 DDG.',
  },
  privacy: {
    title: 'Politique de confidentialité',
    description: 'Découvrez comment All-Chat collecte, traite et protège vos informations.',
  },
  terms: {
    title: "Conditions d'utilisation",
    description: "Comprenez les règles et les responsabilités liées à l'utilisation d'All-Chat.",
  },
}
