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

export const auth: LocaleNamespace<'auth'> = {
  link: {
    title: 'Associer une surface de contrôle',
    intro:
      "Approuvez un Stream Deck, un StreamController ou une autre surface de contrôle de bureau. Rien n'est copié ni collé : l'identifiant d'accès est transmis directement au plugin, et vous pouvez le révoquer à tout moment depuis {devices}.",
    introDevices: 'vos appareils associés',
    loading: "Chargement de la demande d'association",
    codeTitle: "Saisissez le code d'association",
    codeBody:
      "Votre plugin affiche un code de huit caractères. Saisissez-le ici. C'est la méthode à suivre pour une surface de contrôle située sur une autre machine que ce navigateur.",
    codeLabel: "Code d'association",
    codePlaceholder: 'ABCD-EFGH',
    codeHint:
      "Les majuscules et minuscules n'ont pas d'importance, et le tiret est facultatif. Les codes expirent au bout de dix minutes.",
    codeChecking: 'Vérification…',
    codeContinue: 'Continuer',
    codeInvalid:
      "Ce code n'est pas valide. Vérifiez le code affiché par votre plugin, ou relancez l'association depuis le plugin.",
    approveTitle: 'Un appareil souhaite contrôler votre chat',
    approveBody:
      "Il indique s'appeler « {name} ». Ce nom est {selfReported} : nous n'avons aucun moyen de le vérifier, considérez-le donc comme une indication, pas comme une preuve. Si vous ne venez pas de lancer l'association d'une surface de contrôle, refusez.",
    approveSelfReported: 'déclaré par le plugin lui-même',
    overlayLegend: 'Quel overlay peut-il contrôler ?',
    overlayBody:
      "L'appareil est verrouillé sur cet overlay tant qu'il reste associé. Pour le déplacer, révoquez-le et associez-le à nouveau.",
    overlayNone:
      "Vous n'avez pas encore d'overlay. Créez-en un d'abord, puis associez votre appareil.",
    scopeLegend: 'Que peut-il faire ?',
    scopeBody:
      "Le plugin a demandé les éléments ci-dessous. Désactivez tout ce que vous ne voulez pas lui accorder : vous pouvez accorder moins que ce qu'il a demandé, jamais plus.",
    scopeNone:
      "Choisissez-en au moins un : un appareil qui n'en a aucun peut se connecter mais ne peut rien faire.",
    chatWriteCaveat:
      "Bon à savoir : l'envoi de messages dans le chat ne dépend pas de l'overlay. Les messages partent sur toutes les plateformes connectées à votre compte, cette autorisation n'est donc pas limitée par l'overlay choisi ci-dessus. Seules les actions de sondage et de prédiction le sont.",
    scopeChatWriteTitle: 'Envoyer des messages dans le chat',
    scopeChatWriteBody: 'Permet à cet appareil de publier des messages dans vos chats connectés.',
    scopeEngagementWriteTitle: 'Gérer les sondages et les prédictions',
    scopeEngagementWriteBody:
      "Permet à cet appareil d'ouvrir, de clôturer, de résoudre et d'annuler des sondages et des prédictions.",
    scopeUnknownBody: 'Demandé par cet appareil.',
    nameLabel: 'Nommez cet appareil',
    namePlaceholder: 'Stream Deck (PC du studio)',
    nameHint:
      "Affiché dans la liste de vos appareils associés, pour que vous sachiez plus tard ce que vous révoquez. Reprend d'abord la suggestion du plugin ; remplacez-la par ce que vous voulez.",
    approving: 'Approbation…',
    approve: 'Approuver cet appareil',
    deny: 'Refuser',
    approvedTitle: 'Appareil approuvé',
    approvedBody: "Retournez dans votre plugin, il terminera l'association tout seul.",
    deniedTitle: 'Appareil refusé',
    approveFailed:
      "Impossible d'approuver cet appareil. La demande a peut-être expiré : relancez l'association depuis le plugin.",
    denyFailed:
      "Impossible de refuser cette demande. Elle expirera d'elle-même dans les dix minutes.",
    overlaysFailed: 'Impossible de charger vos overlays. Actualisez la page pour réessayer.',
    requestExpired:
      "Cette demande d'association a expiré ou a déjà été utilisée. Relancez l'association depuis le plugin.",
  },
  callback: {
    authenticating: 'Authentification...',
    noCode: "Aucun code d'authentification reçu",
    exchangeFailed: "Échec de l'authentification. Le code a peut-être expiré, veuillez réessayer.",
    failed: "Échec de l'authentification. Veuillez réessayer.",
    duplicateAccount:
      "Votre chaîne {platform} est déjà connectée au compte All-Chat « {username} ». Connectez-vous plutôt avec ce compte. Vous n'avez pas besoin d'une connexion {platform} séparée.",
    returnHome: "Retour à l'accueil",
  },
  viewerSuccess: {
    completing: "Finalisation de l'authentification...",
    pleaseWait: 'Veuillez patienter',
    succeeded: 'Authentification réussie !',
    closeWindow: 'Vous pouvez fermer cette fenêtre',
    noCode: "Aucun code d'authentification reçu",
    exchangeFailed: "Échec de l'authentification. Le code a peut-être expiré, veuillez réessayer.",
    viewerInfoFailed: "Impossible de finaliser l'authentification. Veuillez réessayer.",
  },
  viewerError: {
    title: "Échec de l'authentification",
    defaultError: "Échec de l'authentification",
    body: "Une erreur s'est produite lors de l'authentification avec votre {account}. Veuillez réessayer ou contacter le support si le problème persiste.",
    accountNamed: 'compte {platform}',
    accountNeutral: 'compte de streaming',
    returnHome: "Retour à l'accueil",
    goBack: 'Retour',
  },
}
