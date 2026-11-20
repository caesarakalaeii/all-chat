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

export const moderation: LocaleNamespace<'moderation'> = {
  actions: {
    delete: 'Supprimer des messages',
    timeout: 'Exclusion temporaire',
    ban: 'Bannir',
    unban: 'Débannir',
  },
  readiness: {
    notAModerator:
      'Pas encore modérateur sur {platform} : ajoutez cette personne dans les outils de {platform}.',
    needsConsent: 'En attente de la connexion de son compte {platform}.',
    needsDiscordLink: "En attente de l'association de son compte Discord.",
    unavailable: 'Impossible de vérifier {platform} pour le moment.',
  },
  status: {
    pending: 'Invitation en attente',
    suspended: "Suspendu après 90 jours d'inactivité",
    revoked: 'Retiré',
    active: 'Actif',
  },
  unnamedInvite: 'Invitation sans nom',
  roster: {
    loadFailed: 'Impossible de charger les modérateurs de cet overlay.',
    tryAgain: 'Réessayer',
    loading: 'Chargement des modérateurs…',
    // `emphasis` is rendered as an element; the sentence stays whole.
    explainer:
      "Les modérateurs agissent avec {emphasis} comptes de plateforme : Twitch, YouTube et Kick vérifient donc leur rôle de modérateur à chaque action. Retirer quelqu'un ici prend effet immédiatement.",
    explainerEmphasis: 'leurs propres',
    inviteButton: 'Inviter un modérateur',
    usage: '{used} sur {cap} utilisés',
    removeAll: 'Tout retirer',
    atCap:
      "Cet overlay a atteint sa limite de {cap} modérateurs. Retirez quelqu'un pour en inviter un autre.",
    empty:
      "Personne ne modère encore cet overlay. Invitez quelqu'un : cette personne pourra supprimer des messages et exclure temporairement des spectateurs depuis la vue moniteur, avec ses propres comptes de plateforme.",
  },
  row: {
    legChangeFailed: 'Impossible de modifier {platform} pour {name}.',
    needsOneAction: '{name} doit conserver au moins une action. Retirez plutôt cette personne.',
    updateFailed: 'Impossible de mettre à jour {name}.',
    removed: '{name} a été retiré.',
    removeFailed: 'Impossible de retirer {name}.',
    removeAllFailed: 'Impossible de retirer tout le monde. Réessayez.',
    removeButton: 'Retirer',
    removeLabel: 'Retirer {name}',
    forAccount: 'pour {account}',
    platformToggleLabel: 'Modération {platform} pour {name}',
    removedCountOne: '{count} modérateur retiré.',
    removedCountMany: '{count} modérateurs retirés.',
  },
  revoke: {
    title: 'Retirer {name} ?',
    description:
      "Cette personne perd l'accès à la modération de cet overlay dès sa prochaine action. Ses actions passées restent dans le journal.",
    cancel: 'Annuler',
    confirm: 'Retirer',
  },
  revokeAll: {
    title: 'Retirer tous les modérateurs ?',
    description:
      "Cela retire tout le monde de cet overlay, y compris les invitations que personne n'a encore acceptées. Vous pourrez réinviter des personnes ensuite.",
    confirm: 'Retirer tout le monde',
  },
  invite: {
    title: 'Inviter un modérateur',
    sendLink:
      "Envoyez ce lien à la personne que vous souhaitez voir modérer. Il fonctionne une seule fois, expire dans 7 jours et {emphasis} : s'il se perd, créez une nouvelle invitation.",
    sendLinkEmphasis: 'ne sera plus affiché',
    copied: 'Copié',
    copyLink: 'Copier le lien',
    done: 'Terminé',
    acceptExplainer:
      "La personne accepte avec son propre compte All-Chat, puis connecte chaque plateforme la première fois qu'elle y modère.",
    labelPrompt: 'Pour qui est-ce ? (facultatif)',
    labelPlaceholder: 'Sarah, ma modératrice Twitch',
    actionsLegend: 'Que peut faire cette personne ?',
    platformsLegend: "Sur quelles plateformes ? Désactivé signifie qu'elle ne peut pas y agir.",
    cancel: 'Annuler',
    creating: 'Création…',
    create: "Créer l'invitation",
    capReached: 'Cet overlay a déjà atteint le nombre maximal de modérateurs.',
    createFailed: "Impossible de créer l'invitation. Réessayez.",
    boundToAccount: 'Cette invitation appartient à {account}.',
    copyFailed: 'Copie impossible. Sélectionnez le lien et copiez-le manuellement.',
    premiumGate:
      "Déléguer la modération fait partie d'All-Chat premium. Vos modérateurs ne paient jamais : seule votre propre formule compte.",
    upgradeLink: 'Passez à premium pour inviter des modérateurs',
  },
  accept: {
    heading: 'Invitation à modérer',
    loadingInvite: "Chargement de l'invitation",
    goToChannels: 'Accéder à vos chaînes',
    accept: 'Accepter et commencer à modérer',
    accepting: 'Acceptation…',
    notNow: 'Pas maintenant',
    actionDelete: 'Supprimer des messages',
    actionTimeout: 'Exclure temporairement des spectateurs',
    actionBan: 'Bannir des spectateurs',
    actionUnban: 'Lever les bannissements et les exclusions temporaires',
    signInHeading: 'Connectez-vous pour accepter cette invitation',
    signInBody:
      "La modération est liée à un compte All-Chat : nous devons donc savoir auquel la confier. Connectez-vous, puis rouvrez le lien d'invitation, il reste valide.",
    signIn: 'Se connecter',
    askingToHelp: "{owner} vous demande de l'aider à modérer",
    addressedTo: 'Cette invitation est adressée à « {label} ».',
    actionsHeading: 'Ce que vous pourriez faire',
    platformsHeading: 'Sur ces plateformes',
    noPlatforms: "Aucune pour l'instant : {owner} doit encore activer une plateforme.",
    ownAccountNote:
      'Vous agirez avec votre propre compte de plateforme : chaque plateforme vérifie donc toujours que {owner} vous y a nommé modérateur. Rien ne vous est demandé maintenant, vous connectez une plateforme la première fois que vous y modérez.',
    expectedAccount: 'Cette invitation est destinée à {account}.',
    expectedAccountOnPlatform: 'Cette invitation est destinée à {account} sur {platform}.',
    errorMissingToken:
      "Il manque le code d'invitation dans ce lien. Demandez au streamer de vous le renvoyer.",
    errorNotFound:
      "Cette invitation n'est plus valide : elle a peut-être déjà été utilisée, ou le streamer l'a retirée. Demandez-lui-en une nouvelle.",
    errorExpired: 'Cette invitation a expiré. Demandez-en une nouvelle au streamer.',
    errorAlreadyModerator: 'Vous modérez déjà cette chaîne. Elle figure sur votre page de chaînes.',
    errorOwnerCannotAccept:
      "C'est votre propre overlay : vous y disposez déjà de tous les droits de modération.",
    errorBoundToOther:
      "Cette invitation est réservée à un compte précis. Connectez-vous avec ce compte, ou demandez au streamer d'envoyer une nouvelle invitation pour celui-ci.",
    errorBoundToOtherAccount:
      "Cette invitation est réservée à un compte précis ({account}). Connectez-vous avec ce compte, ou demandez au streamer d'envoyer une nouvelle invitation pour celui-ci.",
    errorBoundToOtherPlatform:
      "Cette invitation est réservée à un compte {platform} précis. Connectez-vous avec ce compte, ou demandez au streamer d'envoyer une nouvelle invitation pour celui-ci.",
    errorBoundToOtherBoth:
      "Cette invitation est réservée à un compte {platform} précis ({account}). Connectez-vous avec ce compte, ou demandez au streamer d'envoyer une nouvelle invitation pour celui-ci.",
    errorUnknown: "Impossible d'ouvrir cette invitation. Vérifiez le lien et réessayez.",
  },
  channels: {
    heading: 'Chaînes que vous modérez',
    subheading:
      "Les overlays que d'autres streamers vous ont confiés. Vous agissez avec votre propre compte de plateforme : chaque plateforme vérifie donc toujours que vous faites partie de leurs modérateurs.",
    loading: 'Chargement des chaînes',
    loadFailed: 'Impossible de charger vos chaînes.',
    tryAgain: 'Réessayer',
    emptyHeading: 'Aucune chaîne pour le moment',
    emptyBody:
      "Lorsqu'un streamer vous invite à modérer son overlay, il vous envoie un lien privé. Ouvrez-le en étant connecté à ce compte et sa chaîne apparaîtra ici.",
    forOwner: 'pour {owner}',
    noPlatforms: "Aucune plateforme activée pour l'instant : demandez à {owner} d'en activer une.",
    suspendedNote:
      'Suspendu après 90 jours sans aucune action. Demandez à {owner} de le réactiver.',
    unavailableNote:
      "La formule de {owner} n'inclut pas la modération pour le moment : les actions sont indisponibles jusqu'à son renouvellement.",
    openMonitor: 'Ouvrir le moniteur de chat',
    discordPromptBody:
      "Associez votre compte Discord pour modérer Discord. All-Chat vérifie vos propres permissions sur le serveur avant d'agir, il doit donc savoir quel compte Discord est le vôtre.",
    linkDiscord: 'Associer Discord',
    noticeDiscordAlreadyLinked:
      "Ce compte Discord est déjà associé à un autre compte All-Chat. Associez-en un autre, ou dissociez-le d'abord de l'autre compte.",
    noticeConnectFailed:
      "Cette connexion n'a pas abouti. Ouvrez une chaîne ci-dessous et réessayez depuis celle-ci.",
    noticeDiscordLinked:
      'Compte Discord associé. All-Chat peut désormais vérifier vos propres permissions sur le serveur lorsque vous modérez Discord.',
    noticeConnected:
      '{platform} connecté. Cela couvre désormais toutes les chaînes qui vous ont délégué {platform}.',
  },
}
