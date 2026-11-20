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

export const viewerOverlay: LocaleNamespace<'viewerOverlay'> = {
  engagement: {
    pollHeading: 'Sondage',
    predictionHeading: 'Prédiction',
    twitchSourceBadge: 'Twitch',

    labelListEntry: '{noun} {index}',
    labelListRemove: 'Retirer',
    labelListAdd: 'Ajouter : {noun}',

    pollQuestionPlaceholder: 'Question',
    pollQuestionLabel: 'Question du sondage',
    pollOptionNoun: 'Option',
    pollAllowChange: 'Autoriser à changer de vote',
    pollAutoCloseAfter: 'Clôture automatique après',
    secondsSuffix: 's',
    pollStart: 'Lancer le sondage',

    pollVotes: '{total} votes',
    // Appended to pollVotes, so it opens with its own separator.
    pollAutoCloses: ' · clôture automatique {time}',
    pollNew: 'Nouveau sondage',
    pollClose: 'Clôturer le sondage',
    pollMirroredNote:
      "Repris depuis Twitch : les spectateurs votent dans l'interface ou le chat Twitch",
    pollParticipateHint:
      'Les spectateurs votent sur la {link} ou depuis le chat ({voteCommand} ou simplement {shortCommand})',
    participateLink: 'page de participation',

    predictionTitlePlaceholder: 'Titre (par exemple : Allons-nous gagner cette manche ?)',
    predictionTitleLabel: 'Titre de la prédiction',
    predictionOutcomeNoun: 'Résultat',
    predictionAutoLockAfter: 'Verrouillage automatique des mises après',
    predictionStart: 'Lancer la prédiction',
    predictionParticipateHint:
      'Les spectateurs misent sur la {link} (ils y voient leur solde) ou depuis le chat : {predictCommand}',

    predictionPointsWagered: '{total} points misés',
    predictionAutoLocks: ' · verrouillage automatique {time}',
    predictionOutcomeTally: '{points} pts · {entrants} participants',
    winningOutcome: 'Résultat gagnant',
    winningOutcomeChoice: 'Résultat gagnant : {label}',
    predictionLock: 'Verrouiller les mises',
    predictionNew: 'Nouvelle prédiction',
    predictionLockedNote:
      'Choisissez le résultat gagnant, puis payez. Les paiements sont définitifs.',
    predictionMirroredNote: 'Reprise depuis Twitch : fonctionne avec les points de chaîne Twitch',

    predictionResolve: 'Résoudre',
    predictionPayOut: 'Payer « {label} »',
    predictionPayOutConfirm: 'Payer « {label} » : définitif ?',
    predictionResolveDisabledTitle: "Sélectionnez d'abord le résultat gagnant",

    predictionCancel: 'Annuler et rembourser',
    predictionCancelTitle: 'Annuler et rembourser toutes les mises',
    predictionCancelConfirm: 'Vraiment rembourser toutes les mises ?',

    mirrorNote:
      'Reprenez les sondages et prédictions natifs de Twitch sur vos overlays (lecture seule). Sur activation uniquement ; prend effet après la prochaine synchronisation de la chaîne (un redémarrage du stream ou un nouvel ajout de la source).',
    mirrorEnable: 'Activer la reprise depuis Twitch',

    pollIncompleteToast: 'Un sondage a besoin d’une question et d’au moins 2 options',
    pollStartedToast: 'Sondage lancé',
    pollStartFailed: 'Impossible de lancer le sondage',
    pollClosedToast: 'Sondage clôturé',
    pollCloseFailed: 'Impossible de clôturer le sondage',
    predictionIncompleteToast: 'Une prédiction a besoin d’un titre et d’au moins 2 résultats',
    predictionStartedToast: 'Prédiction lancée',
    predictionStartFailed: 'Impossible de lancer la prédiction',
    predictionLockedToast: 'Prédiction verrouillée : les mises sont gelées',
    predictionLockFailed: 'Impossible de verrouiller la prédiction',
    pickWinnerToast: "Choisissez d'abord le résultat gagnant",
    predictionNoLongerLockedToast: "La prédiction n'est plus verrouillée : actualisez et réessayez",
    predictionResolvedToast: 'Prédiction résolue : les gagnants ont été payés',
    predictionResolveFailed: 'Impossible de résoudre la prédiction',
    predictionCanceledToast: 'Prédiction annulée : toutes les mises ont été remboursées',
    // {state} is the engagement service's own lowercase value.
    nothingToCancelToast: 'Rien à annuler : la prédiction est déjà {state}',
    predictionCancelFailed: "Impossible d'annuler la prédiction",
    twitchConsentFailedToast: "Impossible de lancer l'autorisation Twitch. Veuillez réessayer.",
  },

  participate: {
    loading: 'Chargement…',
    loginHeading: 'Rejoignez la partie',
    loginBlurb: 'Connectez-vous avec votre compte de plateforme pour voter et miser.',
    loginWith: 'Continuer avec {platform}',
    noWebLoginNote:
      "Vous regardez sur TikTok ou Discord ? Participez avec les commandes de chat affichées à l'écran : la connexion web n'est pas encore disponible pour ces plateformes.",

    heading: 'Participer',
    balanceLabel: 'Solde : {balance} {pointsName}',
    balance: '{balance} {pointsName}',
    defaultPointsName: 'Points',

    settledBanner:
      'Votre prédiction sur « {outcome} » est réglée : vous aviez misé {amount} {pointsName}. Consultez votre solde ci-dessus.',

    pollNativeNote:
      "Ce sondage se déroule sur Twitch : votez dans le chat Twitch ou l'application Twitch.",
    pollVoteNativeTitle: 'Votez dans le chat Twitch',
    working: 'En cours…',
    yourVote: '(votre vote)',
    pollOptionTally: '{pct} % ({votes})',

    predictionLocked: 'verrouillée',
    predictionNativeNote: 'Cette prédiction fonctionne avec les points de chaîne Twitch.',
    youHave: 'Vous avez {balance} {pointsName}',
    maxWager: 'Max',
    noPointsYet:
      "Vous n'avez pas encore de {pointsName}. Gagnez-en en gardant cette page ouverte et en soutenant le stream (abonnements, bits, dons, cadeaux), puis revenez miser.",
    wagerAmountLabel: 'Montant à miser en {pointsName}',
    wagerAmountPlaceholder: 'Montant à miser ({pointsName})',
    // Appended to the outcome label, so it opens with its own separator.
    yourWager: ' · votre mise : {amount}',
    outcomeTally: '{points} · {pct} %',
    alreadyWagered: 'Votre mise est verrouillée pour cette manche.',
    nothingActive: "Aucun sondage ni prédiction en cours pour l'instant. Patience !",

    wagerNativeTitle: 'Fonctionne avec les points de chaîne Twitch',
    wagerAlreadyTitle: 'Vous avez déjà misé pour cette manche',
    wagerClosedTitle: 'Les mises sont closes',

    lockedAnnouncement: 'Prédiction verrouillée : les mises sont closes.',

    rejectNotFound: "Cette prédiction n'est plus disponible.",
    rejectNotActive: 'Les mises sont closes pour cette manche.',
    rejectBadOutcome: "Ce résultat n'est pas valide.",
    rejectAlreadyWagered: 'Vous avez déjà placé une mise pour cette manche.',
    rejectInsufficient: 'Pas assez de {pointsName}. Vous en avez {balance}.',
    rejectNative: 'Cette prédiction fonctionne avec les points de chaîne Twitch.',
    insufficientLocal: 'Pas assez de {pointsName} : vous en avez {balance}.',

    loginFailed: 'Impossible de lancer la connexion. Veuillez réessayer.',
    voteFailed: 'Échec du vote',
    wagerFailed: 'Échec de la mise',
    wagerNeedsAmount: 'Saisissez un montant positif à miser.',
  },

  pollWidget: {
    finalBadge: 'Final',
    finalResults: 'Résultats finaux',
    winnerPill: 'Gagnant',
    tiePill: 'Égalité',
    optionTally: '{pct} % ({votes})',
    remaining: '{clock} restantes',
  },

  predictionWidget: {
    stateLocked: '🔒 VERROUILLÉE',
    stateResolved: '🏆 RÉSOLUE',
    stateOpen: 'OUVERTE',
    winnerPill: 'Gagnant',
    outcomeTally: '{points} pts · {pct} %',
    pool: '{points} pts misés · {players} joueurs',
  },

  credits: {
    loading: 'Chargement des crédits...',
    loadFailed: 'Échec du chargement du générique',
    errorHeading: 'Impossible de charger le générique',
    errorHint: 'Vérifiez que vous avez une session de stream active',
    empty: 'Aucune donnée de générique disponible',

    heading: '🎬 Crédits du stream',
    subheading: 'Merci à toutes les personnes qui ont soutenu le stream !',
    session: 'Session : {date}',
    duration: 'Durée : {duration}',

    durationHourOne: '{hours} heure',
    durationHourMany: '{hours} heures',
    durationMinuteOne: '{minutes} minute',
    durationMinuteMany: '{minutes} minutes',
    durationHoursAndMinutes: '{hours} {minutes}',
    durationJustStarted: 'vient de commencer',

    topSubscribers: 'Top abonnés',
    topGifters: 'Top donateurs',
    topCheerers: 'Top cheerers',
    topChannelPoints: 'Top points de chaîne',
    topRaiders: 'Top raiders',
    topSuperChats: 'Top Super Chats',
    newFollowers: 'Nouveaux followers',
    topMembers: 'Top membres',
    topCoinChests: 'Top coffres de pièces',
    topSharers: 'Top partages',
    topLikers: 'Top likes',

    valueBitsOne: '{count} bit',
    valueBitsMany: '{count} bits',
    valueMonthsOne: '{count} mois',
    valueMonthsMany: '{count} mois',
    valueViewersOne: '{count} spectateur',
    valueViewersMany: '{count} spectateurs',
    valuePointsOne: '{count} point',
    valuePointsMany: '{count} points',
    valuePointsNamed: '{count} {name}',
    valueGiftsOne: '{count} cadeau',
    valueGiftsMany: '{count} cadeaux',
    valueDiamondsOne: '{count} diamant',
    valueDiamondsMany: '{count} diamants',
    valueCoinsOne: '{count} pièce',
    valueCoinsMany: '{count} pièces',
    valueLikesOne: '{count} like',
    valueLikesMany: '{count} likes',
    valueSharesOne: '{count} partage',
    valueSharesMany: '{count} partages',

    nowPlaying: 'En cours de lecture',
    clipViews: '{views} vues',
    clipCounter: 'Clip {index}/{total}',

    thanks: "Merci d'avoir regardé ! ❤️",
    seeYou: 'À bientôt pour le prochain stream !',
  },

  activity: {
    heading: 'Activité et événements',
    empty: "Aucun événement pour l'instant.",
    modBadge: 'mod',
    someUser: 'un utilisateur',

    // Noun phrases, so nothing has to agree with the user's gender.
    deleted: 'Message supprimé',
    deletedBy: 'Message supprimé par {moderator}',
    cleared: 'Chat vidé',
    clearedBy: 'Chat vidé par {moderator}',
    banned: 'Bannissement de {user}',
    bannedBy: 'Bannissement de {user} par {moderator}',
    timedOut: 'Exclusion temporaire de {user}',
    timedOutFor: 'Exclusion temporaire de {user} pendant {seconds} s',
    timedOutBy: 'Exclusion temporaire de {user} par {moderator}',
    timedOutForBy: 'Exclusion temporaire de {user} pendant {seconds} s par {moderator}',
    automodHeld: 'AutoMod a retenu un message de {user}',
    automodHeldCategory: 'AutoMod a retenu un message de {user} ({category})',
    automodResolved: 'Message retenu par AutoMod : {resolution}',
    automodResolvedBy: 'Message retenu par AutoMod : {resolution} par {moderator}',
    automodHeldBadge: 'retenu',
  },

  chatPanel: {
    heading: 'Chat',
    empty: "Aucun message de chat pour l'instant.",
    filteredBy: 'Seuls les messages de {user} sont affichés',
    showAll: 'Afficher tout le chat',
    filteredEmpty: "Aucun message de {user} pour l'instant.",
    filteredCount: '{shown} sur {total}',
    sharedBadge: 'partagé',
  },

  observability: {
    sources: 'Sources ({count})',
    configuredEvents: 'Événements configurés',
    emotes: 'Emotes',
    filters: 'Filtres',

    noSources: 'Aucune source configurée.',
    sourceLive: 'en live',
    sourceIdle: 'inactive',

    eventsUnavailable:
      "Configuration des événements indisponible ; les événements s'affichent ici à mesure qu'ils arrivent.",
    sevenTvSet: 'Set 7TV',
    sevenTvDefault: 'par défaut de chaque source',

    bannedWords: 'Mots bannis',
    bannedUsers: 'Utilisateurs bannis',
    minLength: 'Longueur min.',
    hideCommands: 'Masquer les commandes',
    sayHiFilter: 'Filtre « Say hi »',
    yes: 'oui',
    no: 'non',
    filtersNote: 'Les filtres sont indiqués pour référence ; cette vue affiche tous les messages.',

    twitchOptIns: 'Options Twitch à activer',
    optInAdBreaks: 'Avis de coupure pub',
    optInModLog: 'Événements de modération et AutoMod',
    optInEnable: 'Activer',

    eventTwitchSubs: 'Abonnements Twitch',
    eventTwitchResubs: 'Réabonnements Twitch',
    eventTwitchGiftSubs: 'Abonnements offerts Twitch',
    eventTwitchBits: 'Bits Twitch',
    eventTwitchRaids: 'Raids Twitch',
    eventTwitchChannelPoints: 'Points de chaîne',
    eventTwitchFollows: 'Follows Twitch',
    eventTwitchWatchStreaks: 'Séries de visionnage',
    eventYoutubeSuperChat: 'Super Chat YouTube',
    eventYoutubeSuperSticker: 'Super Sticker',
    eventYoutubeMembers: 'Membres YouTube',
    eventYoutubeMemberMilestones: 'Paliers de membres',
    eventYoutubeMemberGifts: 'Souscriptions offertes',
    eventYoutubeSubscribers: 'Abonnés YouTube',
    eventKickSubs: 'Abonnements Kick',
    eventKickGifts: 'Cadeaux Kick',
    eventTiktokLikes: 'Likes TikTok',
    eventTiktokGifts: 'Cadeaux TikTok',
    eventTiktokFollows: 'Follows TikTok',
    eventTiktokShares: 'Partages TikTok',
    eventTiktokTreasureChests: 'Coffres de pièces TikTok',
    eventTiktokSubscriptions: 'Abonnements TikTok',
    eventTiktokSuperFans: 'Super Fans TikTok',
    eventTiktokQuestions: 'Questions TikTok',
    eventTokenWarnings: 'Avertissements de jeton',
  },

  viewSettings: {
    buttonLabel: "Paramètres d'affichage",
    buttonText: 'Affichage',
    heading: 'Paramètres de la vue',
    chatOrderHeading: 'Ordre du chat',
    activitySoundHeading: "Son d'activité",

    showAvatars: 'Avatars',
    showBadges: 'Badges',
    showPronouns: 'Pronoms',
    showTimestamps: 'Horodatages',
    showPlatformGlyph: 'Icône de plateforme',
    showModeration: 'Contrôles de modération',

    newestFirst: "Messages les plus récents d'abord",
    newestFirstNote:
      'Place le message le plus récent en haut du panneau Chat, pour lire le chat sans baisser les yeux. Ne concerne que ce navigateur.',

    activitySoundEnabled: 'Son à chaque nouvelle activité',
    soundPresetLabel: 'Son',
    volume: 'Volume',
    testSound: 'Tester le son',
    activitySoundNote:
      "Joué uniquement ici, dans ce navigateur, pour que vous remarquiez les activités faciles à manquer, comme une récompense de points de chaîne ou une Rose TikTok. C'est indépendant des sons de notification de votre overlay diffusés à l'antenne.",
  },

  chatSend: {
    formLabel: 'Envoyer un message de chat',
    targetGroupLabel: 'Envoyer vers',
    sendToPlatform: 'Envoyer vers {platform}',
    enableSendingFor: "Activer l'envoi pour {platform}",
    allLabel: 'Envoyer vers toutes les plateformes',
    allText: 'Toutes',
    messageLabel: 'Message de chat',
    placeholderAll: 'Message à toutes les plateformes…',
    placeholderOne: 'Envoyer un message…',
    send: 'Envoyer',

    enableSending: "Activer l'envoi",
    reconnect: 'Reconnecter',
    enablePlatform: 'Activer {platform}',
    reconnectPlatform: 'Reconnecter {platform}',

    sendFailed: "Échec de l'envoi. Veuillez réessayer.",
    missingScope: "L'envoi n'est pas encore activé.",
    missingScopeFor: "L'envoi n'est pas encore activé pour {platform}.",
    reauthRequired: 'Votre connexion {platform} a expiré. Veuillez vous reconnecter.',
    reauthRequiredGeneric: 'Votre connexion à la plateforme a expiré. Veuillez vous reconnecter.',
    rateLimitedRetry: "Limite d'envoi atteinte : réessayez dans {seconds} s.",
    rateLimited: "Limite d'envoi atteinte : veuillez ralentir.",
    streamOffline: "Cette chaîne n'est pas en live actuellement.",

    sent: 'Envoyé ✓',
    resultOk: '{platform} ✓',
    resultFailed: '{platform} ✗',
    resultFailedWhy: '{platform} ✗ {why}',

    reasonReauthRequired: 'reconnexion',
    reasonMissingScope: 'verrouillé',
    reasonStreamOffline: 'hors ligne',
    reasonQuotaExhausted: 'quota',
    reasonSendFailed: 'échec',
    reasonMessageDropped: 'bloqué par les paramètres du chat',
  },

  moderationControls: {
    menuLabel: "Modérer l'utilisateur",
    timeout: 'Exclure temporairement',
    ban: "Bannir l'utilisateur",
    unban: "Débannir l'utilisateur",
    deleteMessage: 'Supprimer le message',

    noModerationApi: "{platform} n'a pas d'API de modération",
    unavailable: 'La modération est indisponible pour cette source',
    missingScope: 'Accordez les autorisations de modération pour activer les actions de modération',
    needsDiscordLink: 'Associez votre compte Discord pour modérer ici',
    ownerChannelUnverified:
      "Le compte Discord de ce streamer n'est pas connecté : rien ne peut donc être modéré ici",
    botMissingPermission:
      "Le bot All-Chat n'a pas reçu cette autorisation Discord : demandez au streamer de le réinviter",
  },

  layoutPicker: {
    groupLabel: 'Disposition des panneaux',
    chatLeft: 'Chat à gauche, événements à droite',
    chatRight: 'Chat à droite, événements à gauche',
    chatTop: 'Chat en haut, événements en dessous',
    eventsTop: 'Événements en haut, chat en dessous',
  },

  platformGlyph: {
    groupLabel: 'Plateformes : {platforms}',
  },

  monitor: {
    details: 'Détails',
    engagement: 'Engagement',
    engagementTitle: 'Gérer les sondages et prédictions de cet overlay',
    rediscoverYouTube: 'Redétecter YouTube',
    rediscoverYouTubeTitle:
      "Forcer YouTube à redétecter le live : à utiliser si le chat s'est arrêté après un plantage ou un redémarrage du stream",
    obsOverlay: 'Overlay OBS',

    stillReconnecting:
      "Reconnexion toujours en cours : elle se rétablit d'elle-même, et les messages envoyés entre-temps sont rejoués au retour de la connexion. C'est en fermant cette page que vous les perdriez.",
    replayTruncated:
      "Certains messages antérieurs peuvent manquer : la déconnexion a duré plus longtemps que le tampon de relecture, la partie la plus ancienne n'a donc pas pu être récupérée.",

    noRole:
      'Vous pouvez consulter ce moniteur, mais vous ne modérez pas ici : la modération est désactivée.',

    featureGatedOwner: 'La modération du chat est une fonctionnalité premium.',
    featureGatedUpgrade: 'Passez à premium pour modérer depuis votre overlay',
    featureGatedModerator:
      "L'offre de ce streamer n'inclut pas la modération pour le moment : vos actions sont indisponibles jusqu'à son renouvellement.",

    // {platform} is the raw lowercase wire value.
    needsConsent: 'Connectez votre propre compte {platform} pour modérer.',
    needsConsentChannel: 'Connectez votre propre compte {platform} pour modérer {channel}.',
    connectPlatform: 'Connecter {platform}',

    needsDiscordLink:
      "Associez votre compte Discord pour modérer Discord ici : All-Chat vérifie vos propres autorisations sur le serveur avant d'agir.",
    linkDiscord: 'Associer Discord',

    missingScope:
      'Accordez les autorisations de modération pour activer les actions de modération sur {platform}.',
    missingScopeChannel:
      'Accordez les autorisations de modération pour activer les actions de modération sur {platform} ({channel}).',
    missingScopeDiscord:
      'Réinvitez le bot avec les autorisations de modération pour activer les actions de modération sur {platform}.',
    missingScopeDiscordChannel:
      'Réinvitez le bot avec les autorisations de modération pour activer les actions de modération sur {platform} ({channel}).',
    reinviteBot: 'Réinviter le bot',
    enableModeration: "Activer la modération et l'envoi de messages",
    comingSoonFor: '(bientôt disponible pour {platform})',

    modLogOptIn:
      "Affichez les actions de modération Twitch et les messages retenus par AutoMod dans ce fil d'activité. Twitch exige une autorisation AutoMod de « gestion » pour simplement nous transmettre les messages retenus : All-Chat ne fait que les lire ; il n'y a pas encore de boutons pour approuver ou refuser.",
    enableModLog: 'Afficher les événements de modération et AutoMod',

    adBreaksOptIn:
      "Recevez un avis dans ce fil d'activité lorsqu'une coupure pub Twitch commence, qu'elle soit programmée ou lancée à la main, pour mettre en pause ce que vous diffusez.",
    enableAdBreaks: 'Afficher les coupures pub',

    reauthOwner:
      "Votre autorisation de modération {platform} a expiré ou n'a jamais été accordée : autorisez de nouveau pour continuer à modérer depuis votre overlay.",
    reauthModerator:
      "Votre autorisation de modération {platform} a expiré ou n'a jamais été accordée : autorisez de nouveau pour continuer à modérer ici.",
    reconnectPlatform: 'Reconnecter {platform}',
    reauthorizeModeration: "Autoriser de nouveau la modération et l'envoi de messages",

    consentStartFailed:
      'Impossible de lancer la configuration de la modération. Veuillez réessayer.',
    twitchConsentStartFailed: "Impossible de lancer l'autorisation Twitch. Veuillez réessayer.",
    modConnectUnavailable:
      "La connexion de {platform} n'est pas encore disponible. Demandez au streamer d'y modérer pour l'instant.",
    discordLinkUnavailable:
      "L'association de Discord n'est pas disponible actuellement. Demandez au streamer d'y modérer pour l'instant.",
    reloginStartFailed: 'Impossible de lancer la reconnexion. Veuillez réessayer.',

    rediscoverStarted: 'Redétection du stream YouTube…',
    rediscoverRateLimited: 'Veuillez patienter un instant avant de réessayer',
    rediscoverForbidden: 'Non autorisé pour cet overlay',
    rediscoverFailed: 'Impossible de lancer la redétection',

    reauthNeededToast: '{platform} vous demande d’autoriser de nouveau la modération',
    actionFailed: "Échec de l'action de modération",
    messageDeleted: 'Message supprimé',
    timedOut: 'Exclusion temporaire de {name}',
    banned: 'Bannissement de {name}',
    unbanned: 'Débannissement de {name}',
    unbanFailed: 'Échec du débannissement',
    unnamedTarget: 'utilisateur',

    connectRequired: 'Connectez votre propre compte {platform} pour modérer ici',
    ownerChannelUnverifiedModerator:
      "Le compte {platform} de ce streamer n'est pas connecté : rien ne peut donc être modéré ici",
    ownerChannelUnverifiedOwner:
      "Votre compte {platform} n'est pas connecté pour cette chaîne : reconnectez-le pour modérer ici",
    delegationUnsupported:
      "Les modérateurs ne peuvent pas encore agir sur {platform} : demandez au streamer de s'en charger",
    targetNotActionable:
      "{platform} ne permet à personne de modérer cette personne : c'est le propriétaire de la chaîne ou un autre modérateur",
    discordLinkRequired: 'Associez votre compte Discord pour modérer ici',
    modNotInGuild: "Vous n'êtes pas sur ce serveur Discord : demandez au streamer de vous inviter",
    modLacksPermission:
      'Vos rôles Discord ne le permettent pas : demandez au streamer un rôle qui le permet',
    modBelowTarget:
      'La hiérarchie des rôles de Discord bloque cette action : votre rôle le plus élevé doit être au-dessus du sien',
    botMissingPermission:
      "Le bot All-Chat n'a pas reçu cette autorisation Discord : demandez au streamer de le réinviter",
  },

  dock: {
    productName: 'All-Chat',
    signInExplanation:
      'Ce dock a sa propre connexion, distincte de celle de votre navigateur. Connectez-vous une fois et il reste connecté.',
    signInWith: 'Se connecter avec {platform}',
    signInFailed: 'Impossible de lancer la connexion',
    signInFailedBody: 'La connexion avec {platform} a échoué. Veuillez réessayer.',

    chatTab: 'Chat',
    activityTab: 'Activité',
    menuLabel: 'Contrôles du moniteur',
    noticesLabel: 'Avis du moniteur',
    noticesSummary: '{count} avis',
  },

  eventNotice: {
    tokenExpired: 'Le jeton OAuth a expiré',
    tokenRefreshFailed: "Échec de l'actualisation du jeton OAuth",
    tokenExpiredFor: 'Le jeton OAuth de {username} a expiré',
    tokenRefreshFailedFor: "Échec de l'actualisation du jeton OAuth de {username}",
    tokenRemedy: '→ Veuillez reconnecter votre compte dans Paramètres → Connexions',

    channelInaccessible: "Le salon {channel} n'est pas accessible",
    channelRemedy:
      '→ Accordez au bot l’autorisation « Voir le salon » dans les paramètres de votre serveur Discord',

    listenerDeprecated: "L'ancienne connexion au chat Twitch est en cours d'abandon.",
    listenerRemedy:
      '→ Ajoutez de nouveau votre source Twitch pour passer à la nouvelle connexion EventSub',
  },
  customAlertFrameTitle: 'Alerte personnalisée',
  goalTitles: {
    followers: 'Objectif de followers',
    subscribers: "Objectif d'abonnés",
    bits: 'Objectif de Bits',
    super_chat: 'Objectif Super Chat',
  },
  goal: {
    progress: '{current} / {target}',
  },
  listTitles: {
    bits: 'Top cheerers',
    gifts: 'Top donateurs',
    super_chat: 'Top Super Chats',
  },
  list: {
    empty: "Aucun soutien pour l'instant",
  },
  statusIndicator: {
    active: '{platform} - {channel} (Actif)',
    inactive: '{platform} - {channel} (Inactif)',
    connected: '{platform} - {channel} (Connecté)',
    reconnecting: '{platform} - {channel} - Reconnexion dans {seconds} s',
    reconnectingWithError: '{platform} - {channel} - {error} (nouvel essai dans {seconds} s)',
    quotaExceeded: '{platform} - {channel} - Quota dépassé',
    error: '{platform} - {channel} - Erreur',
    discoveryPaused:
      '{platform} - {channel} - Détection en pause (utilisez le moniteur de chat pour réessayer)',
    authRequired: '{platform} - {channel} - Authentification requise',
    offline: '{platform} - {channel} - Hors ligne',
    withErrorMessage: '{platform} - {channel} - {error}',
    countdownSeconds: '{seconds} s',
  },
}
