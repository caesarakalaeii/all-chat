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

export const onboarding: LocaleNamespace<'onboarding'> = {
  steps: {
    createOverlay: 'Créez votre overlay',
    connectSource: 'Connectez une source de chat',
    chooseTheme: 'Choisissez un thème',
    copyObs: 'Ajoutez-le à OBS',
  },
  checklist: {
    title: 'Guide de configuration',
    allDone: 'Toutes les étapes sont terminées !',
    progress: '{done} étapes sur {total} terminées',
    expand: 'Développer le guide de configuration',
    minimize: 'Réduire le guide de configuration',
    dismiss: 'Fermer le guide de configuration',
    // Leading space separates it from the step label for screen readers.
    stepDone: ' (terminé)',
    create: 'Créer',
    showMe: 'Montrez-moi',
    openEditor: "Ouvrir l'éditeur",
    copyLink: 'Copier le lien',
    copied: 'Copié !',
  },
  obs: {
    heading: 'Dans OBS :',
    step1: 'Dans OBS, cliquez sur {plus} sous Sources et choisissez {browser}.',
    step1Plus: '+',
    step1Browser: 'Navigateur',
    step2: "Collez le lien de l'overlay copié dans le champ URL.",
    step3:
      "Dimensionnez la source selon la zone que le chat doit occuper (un cadre haut et étroit comme 450×800 convient bien, pas tout votre canevas), puis faites-la glisser à sa place. Le chat apparaît dès que l'overlay se connecte.",
  },
  obsDock: {
    step1: 'Dans OBS, ouvrez {menu}.',
    step1Menu: 'Affichage → Docks → Docks de navigateur personnalisés',
    step2:
      'Donnez un nom au dock, collez le lien du dock copié dans le champ URL, puis cliquez sur Appliquer et fermez la fenêtre.',
    step3:
      'Le dock a sa propre connexion : connectez-vous une fois dans le panneau et il reste connecté. Faites glisser le panneau où vous le souhaitez ; une colonne étroite à côté de Scènes et Sources convient bien.',
  },
  extras: {
    heading: 'Facultatif : aller plus loin',
    ttsTitle: 'Synthèse vocale',
    ttsBody:
      'Faites lire le chat à voix haute. Les voix du navigateur sont gratuites ; les voix ElevenLabs sont Premium.',
    moderationTitle: 'Modérez depuis votre overlay',
    moderationBody:
      "Supprimez, excluez temporairement, bannissez et débannissez depuis le bouton Vue moniteur en haut de l'éditeur. Contrôles complets sur Twitch, Kick et Discord ; exclusion temporaire et bannissement sur YouTube. (Premium)",
    moderatorsTitle: 'Faites-vous aider par vos modos',
    moderatorsBody:
      "Confiez la Vue moniteur à vos modérateurs existants dans Modérateurs. Ils agissent avec leurs propres comptes de plateforme et n'ont jamais besoin de Premium eux-mêmes. (Premium)",
    sharedChatTitle: 'Chat partagé',
    sharedChatBody:
      "Réunissez plusieurs chaînes en une seule conversation via le bouton Partager l'overlay. (Premium)",
    streamSelectionTitle: 'Sélection du stream YouTube',
    streamSelectionBody:
      "Choisissez précisément la diffusion qu'un overlay écoute, pour chaque source YouTube dans Sources. (Premium)",
    youtubeOfficialApiTitle: 'Streams YouTube non répertoriés et réservés aux membres',
    youtubeOfficialApiBody:
      'Activez API YouTube officielle sur une source YouTube dans Sources, et votre propre chaîne est lue via votre connexion YouTube : les streams non répertoriés et réservés aux membres sont détectés automatiquement. (Premium)',
    bubbleColorsTitle: 'Des bulles de couleurs différentes',
    bubbleColorsBody:
      'Donnez à chaque plateforme sa propre couleur de bulle, ou faites défiler une palette au fil du flux, dans Couleurs des bulles sous Apparence. Gratuit.',
    adBreaksTitle: 'Avis de coupure pub Twitch',
    adBreaksBody:
      'La section Activité et événements du moniteur indique quand une coupure pub Twitch commence, programmée ou manuelle, pour que vous puissiez faire une pause. Activez-la depuis la bannière du moniteur. Gratuit.',
    alertsOverlayTitle: "Overlays d'alertes",
    alertsOverlayBody:
      "Affichez à l'écran les abonnements, les Bits, les raids, les Super Chats et les cadeaux TikTok dans leur propre source OBS, avec un bouton de test pour les positionner. Gratuit.",
    alertsEditorTitle: "Éditeur d'alertes personnalisées",
    alertsEditorBody:
      "Donnez à chaque alerte son propre texte, son GIF et son son, cherchez des GIF directement dans l'éditeur, ou écrivez votre propre HTML, CSS et JavaScript. Gratuit.",
    goalOverlayTitle: "Overlays d'objectif",
    goalOverlayBody:
      "Une barre de progression vers un objectif de followers, d'abonnés, de Bits ou de Super Chats qui se remplit en direct à l'écran.",
    listOverlayTitle: 'Overlays de classement',
    listOverlayBody:
      "Classez à l'écran ceux qui ont envoyé le plus de Bits, de cadeaux ou de Super Chats ces 30 derniers jours, avec une mise à jour à chaque nouvel envoi.",
    mediaUploadsTitle: 'Importez vos propres médias',
    mediaUploadsBody:
      'Premium : importez vos propres images, GIF et sons pour les alertes et les alertes de commande. Coller un lien et chercher des GIF restent gratuits.',
    chatCommandsTitle: 'Commandes de chat',
    chatCommandsBody:
      "Des !commandes personnalisées et des déclencheurs par mot-clé pour tous les chats : répondez en votre nom, créez un clip de votre stream Twitch sur demande, affichez votre propre alerte (image, son, texte) sur votre overlay d'alertes, et utilisez des variables, des requêtes web ou du JavaScript dans les réponses. Les réponses dans le chat YouTube sont premium.",
    youtubeCommandRepliesTitle: 'Réponses des commandes dans le chat YouTube',
    youtubeCommandRepliesBody:
      'Premium : vos commandes de chat et vos minuteurs répondent aussi dans le chat YouTube, pas seulement sur Twitch et Kick.',
    customChatBotTitle: 'Votre propre bot de chat',
    customChatBotBody:
      'Premium : connectez votre propre compte de bot sur Twitch, Kick ou YouTube, et les réponses des commandes et les minuteurs partent sous le nom de votre bot.',
    flairsTitle: 'Flairs de spectateur',
    flairsBody:
      'Des cosmétiques premium pour les chatteurs, comme des dégradés de nom animés, dans Flairs dans la navigation.',
    streamDeckTitle: 'Boutons Stream Deck',
    streamDeckBody:
      'Pilotez les sondages, les prédictions et les messages prédéfinis depuis un bouton physique. Associez un Stream Deck ou StreamController dans Appareils associés, sans rien copier ni coller. Lancer un sondage ou une prédiction reste Premium.',
    languageTitle: 'Utilisez All-Chat dans votre langue',
    languageBody:
      "Changez la langue de l'interface dans Paramètres. L'allemand, le français et l'espagnol sont disponibles, et le choix est mémorisé par navigateur.",
    translateTitle: 'Aidez à traduire All-Chat',
    translateBody:
      "Les bêta-testeurs peuvent traduire l'interface dans d'autres langues dans Traduire dans la navigation. Tout est relu avant la mise en ligne.",
    seePremium: 'Voir tout ce que Premium inclut',
    done: 'Terminé',
    skip: 'Passer',
  },
  finale: {
    title: 'Vous êtes en live ! 🎉',
    body: 'Des questions, des retours ou des demandes de thèmes ? Notre communauté se fera un plaisir de vous aider.',
    joinDiscord: 'Rejoindre le Discord',
    finish: 'Terminer',
  },
  dismissDialog: {
    title: 'Masquer le guide de configuration ?',
    body: 'Vous pouvez le relancer à tout moment depuis Paramètres → Guide de configuration.',
    keep: 'Le garder',
    hide: 'Masquer le guide',
  },
  createDialog: {
    title: 'Créez votre overlay',
    body: 'Donnez-lui un nom, vous connecterez votre chat juste après.',
    nameLabel: "Nom de l'overlay",
    cancel: 'Annuler',
    submit: "Créer l'overlay",
    submitting: 'Création…',
    failedToast: "Impossible de créer l'overlay",
  },
}
