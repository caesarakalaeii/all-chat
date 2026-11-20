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
    dashboard: 'Tableau de bord',
    flairs: 'Flairs',
    admin: 'Admin',
    settings: 'Paramètres',
    docs: 'Docs',
    translate: 'Traduire',
    commands: 'Commandes',
    logOut: 'Se déconnecter',
    signIn: 'Se connecter',
  },
  theme: {
    lightMode: 'Mode clair',
  },
  soundPresets: {
    chime: 'Carillon',
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
    altstream: 'Altstream',
    discord: 'Discord',
  },
  overlayKinds: {
    chat: 'Chat',
    alerts: 'Alertes',
    goal: 'Objectif',
    list: 'Liste',
  },
  patreon: {
    heading: 'Patreon',
    connect: 'Connecter Patreon',
    connecting: 'Redirection…',
    notAPatron: 'Pas encore mécène ?',
    subscribe: "S'abonner sur Patreon",
    subscriptionRow: 'Abonnement',
    renewsRow: 'Renouvellement',
    active: 'Actif',
    inactive: 'Inactif',
    disconnect: 'Déconnecter Patreon',
    disconnectTitle: 'Déconnecter Patreon ?',
    disconnectCancel: 'Annuler',
    disconnectConfirm: 'Oui, déconnecter',
    statusActive: 'Actif',
    statusDeclined: 'Paiement refusé',
    statusFormer: 'Terminé',
    statusNotSubscribed: 'Non abonné',
    connectedToast: 'Patreon connecté !',
    connectFailedToast: 'Impossible de connecter Patreon',
    connectStartFailedToast: 'Impossible de lancer la connexion à Patreon',
    disconnectedToast: 'Patreon déconnecté',
    disconnectFailedToast: 'Échec de la déconnexion',
  },
  // Compact units: min, h, j (jours).
  duration: {
    justNow: "à l'instant",
    minutes: '{minutes} min',
    hours: '{hours} h',
    hoursAndMinutes: '{hours} h {minutes} min',
    days: '{days} j',
    daysAndHours: '{days} j {hours} h',
  },
  badges: {
    allChatLabel: 'Badge All-Chat',
    premiumLabel: 'Badge premium',
  },
  dialog: {
    closeLabel: 'Fermer la boîte de dialogue',
  },
  toast: {
    closeLabel: 'Fermer la notification',
    tryAgain: 'Veuillez réessayer.',
    elevenLabsFallback: 'ElevenLabs indisponible, utilisation de la voix du navigateur.',
  },
  splitPane: {
    resizeLabel: 'Redimensionner les panneaux',
    shrinkConfigLabel: 'Réduire le panneau de configuration',
    growConfigLabel: 'Agrandir le panneau de configuration',
    previewTitle: "Aperçu en direct de l'overlay",
  },
  cssEditor: {
    regionLabel: 'Éditeur de CSS personnalisé',
    keyboardHint:
      "Appuyez sur Ctrl+M pour activer ou désactiver la capture de la touche Tab ; Escape puis Tab permet de quitter l'éditeur.",
    loading: "Chargement de l'éditeur...",
  },
  impersonation: {
    bannerLabel: 'Mode admin :',
    viewingAs: 'Affichage en tant que {username}',
    exitButton: "Quitter et revenir à l'admin",
  },
  moderatingElsewhere: {
    channelOne: '{count} chaîne',
    channelMany: '{count} chaînes',
    sentence: "Vous modérez {channels} pour d'autres streamers.",
    openLink: 'Ouvrir',
  },
  maintenanceInfo: {
    buttonLabel: 'Annonces de service',
    popoverHeading: 'Annonces de service',
  },
  eventSubMigration: {
    body: "L'ancienne connexion au chat par IRC est en cours d'abandon et peut perdre des messages lorsque de nombreux streams sont en live. Reconnectez-vous pour passer à la nouvelle connexion et garder un chat fiable.",
    reconnectButton: 'Se reconnecter maintenant',
    dismissLabel: 'Fermer',
    connectedToast: 'Chat Twitch connecté',
    failedToast: 'Impossible de lancer la mise à niveau',
  },
  forbidden: {
    heading: '403 Forbidden',
    body: "Vous n'avez pas l'autorisation d'accéder à cette page. Des droits d'administrateur sont requis.",
    dashboardButton: 'Aller au tableau de bord',
  },
  betaWarning: {
    youtubeTitle: 'YouTube : vérification OAuth en cours',
    tiktokTitle: 'TikTok : bêta fermée',
    youtubeBody:
      "L'intégration YouTube est actuellement en cours de vérification OAuth par Google. Nous ne pouvons pas ajouter de nouveaux utilisateurs de test pendant cette période.",
    tiktokBody:
      "L'intégration TikTok est actuellement en bêta fermée. Si vous n'avez pas encore été ajouté au programme bêta, l'authentification échouera.",
    youtubeDiscordPrompt:
      "Rejoignez notre communauté Discord pour suivre l'avancement de la vérification et obtenir de l'aide :",
    tiktokDiscordPrompt: 'Pour rejoindre la bêta, rejoignez notre communauté Discord :',
    youtubeExistingUser:
      "Si vous avez déjà été ajouté comme utilisateur de test, vous pouvez continuer à utiliser l'intégration YouTube.",
    tiktokExistingUser:
      "Si vous faites déjà partie de la bêta, vous pouvez poursuivre l'authentification.",
    discordLink: 'Rejoindre le serveur Discord',
    cancelButton: 'Annuler',
    continueButton: "J'ai compris, continuer",
  },
  breadcrumb: {
    label: "Fil d'Ariane",
    home: 'Accueil',
  },
}
