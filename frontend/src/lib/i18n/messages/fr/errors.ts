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

export const errors: LocaleNamespace<'errors'> = {
  unauthorized: {
    title: 'Authentification requise',
    message: 'Vous devez vous connecter pour envoyer des messages.',
    step1: 'Cliquez sur le bouton « Se connecter avec {platform} »',
    step2: "Autorisez l'application à envoyer des messages en votre nom",
    step3: "Essayez d'envoyer votre message à nouveau",
  },
  tokenExpired: {
    title: 'Session expirée',
    message: "Votre session d'authentification a expiré.",
    step1: 'Connectez-vous à nouveau pour actualiser votre session',
    step2: "Assurez-vous d'autoriser l'application",
    step3: "Essayez d'envoyer votre message à nouveau",
  },
  rateLimited: {
    title: "Limite d'envoi atteinte",
    message: 'Vous envoyez des messages trop rapidement. Veuillez ralentir.',
    step1: "Attendez un instant avant d'envoyer un autre message",
    step2: "Évitez d'envoyer des messages à la suite trop rapidement",
    step3: "La limite d'envoi sera réinitialisée automatiquement",
  },
  banned: {
    title: "Impossible d'envoyer des messages",
    message: "Vous êtes actuellement banni de l'envoi de messages.",
    step1: 'Consultez la raison du bannissement ci-dessous',
    step2: "Contactez le streamer ou les modérateurs si vous pensez qu'il s'agit d'une erreur",
    step3: "Attendez la fin du bannissement s'il est temporaire",
  },
  streamerOffline: {
    title: 'Stream hors ligne',
    message: "Ce streamer n'est actuellement pas en live.",
    step1: "Vérifiez si le stream est terminé ou s'il n'a pas encore commencé",
    step2: "Essayez d'actualiser la page pour mettre à jour le statut du stream",
    step3: 'Vous pouvez uniquement envoyer des messages lorsque le stream est en direct',
  },
  platformApiError: {
    title: 'Erreur de la plateforme',
    message: 'La plateforme de streaming a rencontré une erreur.',
    step1: "Il s'agit probablement d'un problème temporaire avec la plateforme",
    step2: 'Attendez un instant, puis réessayez',
    step3: 'Vérifiez si la plateforme rencontre actuellement des problèmes',
    step4: "Essayez d'envoyer votre message à nouveau dans quelques instants",
  },
  networkError: {
    title: 'Erreur de connexion',
    message: 'Échec de la connexion au serveur.',
    step1: 'Vérifiez votre connexion Internet',
    step2: "Essayez d'actualiser la page",
    step3: 'Si le problème persiste, le serveur rencontre peut-être des problèmes',
  },
  validationError: {
    title: 'Message invalide',
    message: 'Votre message ne respecte pas les conditions requises.',
    step1: "Vérifiez que votre message n'est pas vide",
    step2: "Assurez-vous que votre message n'est pas trop long",
    step3: "Évitez d'utiliser des caractères ou du contenu interdits",
  },
  unknownError: {
    title: 'Erreur inattendue',
    message: "Une erreur inattendue s'est produite lors de l'envoi de votre message.",
    step1: "Essayez d'envoyer votre message à nouveau",
    step2: 'Actualisez la page si le problème persiste',
    step3: 'Contactez le support si cette erreur persiste',
  },

  display: {
    iconLabel: "Icône d'erreur",
    dismissLabel: "Fermer l'erreur",

    rateLimitCountdown: 'Vous pourrez envoyer un autre message dans {countdown}',
    reasonLabel: 'Raison :',
    expiresLabel: 'Expire le :',
    platformMessage: 'Message de la plateforme : {message}',

    whatYouCanDo: 'Ce que vous pouvez faire :',
    tryAgain: 'Réessayer',
    signInWith: 'Se connecter avec {platform}',
    showDetails: 'Afficher les détails',
    hideDetails: 'Masquer les détails',
  },
}
