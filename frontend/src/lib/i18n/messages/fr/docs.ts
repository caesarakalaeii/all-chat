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

export const docs: LocaleNamespace<'docs'> = {
  fieldTable: {
    columnField: 'Champ',
    columnType: 'Type',
    columnDescription: 'Description',
  },
  prose: {
    codeSample: 'Exemple de code',
  },
  // Wire field names, event type names and code samples are not copy.
  api: {
    eyebrow: 'API développeur',
    heading: "Référence de l'API développeur",
    intro:
      'Lisez le flux de chat unifié via un seul WebSocket pour créer des bots, des outils de modération, des compteurs de votes, des statistiques, des alertes, tout ce que vous voulez.',
    guidePrompt: 'Vous configurez simplement votre overlay ? {guide}',
    guideLinkText: 'Voir le guide du streamer →',
    tocHeading: 'Sur cette page',
    tocConnectATool: 'Connecter un outil',
    tocMessageFormat: 'Format des messages',
    tocChatMessages: 'Messages de chat',
    tocEvents: 'Événements',
    tocStatusMessages: 'Messages de statut et de contrôle',
    tocReconnecting: 'Reconnexion et heartbeats',
    tocTryItNow: 'Essayer maintenant (sans configuration)',
    connectHeading: 'Connecter un outil',
    connectBody:
      "All-Chat regroupe le chat en direct de {twitch}, {youtube}, {kick}, {tiktok} et {discord} en un seul flux normalisé : chaque message, plus les événements de plateforme comme les abonnements, les bits, les raids, les Super Chats et les cadeaux, dans un format unique, avec les emotes 7TV/BTTV/FFZ déjà résolues. Le flux qui alimente l'overlay navigateur est celui que lit votre outil.",
    connectAnonymous:
      "Ouvrez un WebSocket vers le point d'accès de l'overlay. La lecture du flux est {anonymous} : aucun token ni compte n'est requis (c'est le même « mode OBS » que celui de l'overlay navigateur) :",
    connectAnonymousEmphasis: 'anonyme',
    connectOverlayId:
      "Votre {field} est l'UUID qui figure dans l'URL de source navigateur de votre overlay. Envie d'essayer d'abord sans compte ? Utilisez l'{testOverlay}.",
    connectTestOverlayLinkText: 'overlay de test public',
    queryParamsHeading: 'Paramètres de requête',
    exampleJavascriptHeading: 'Exemple minimal (JavaScript)',
    examplePythonHeading: 'Exemple minimal (Python)',
    formatHeading: 'Format des messages',
    formatBody: 'Chaque trame est du JSON avec la même enveloppe :',
    formatTypeList: '{field} prend l’une de ces valeurs :',
    chatHeading: 'Messages de chat',
    chatBody: "Pour {chat} et {update}, {data} est l'objet message unifié :",
    userHeading: 'user',
    messageHeading: 'message, emotes et attachments',
    messageBody: '{field} a la forme {shape}. Chaque {emote} :',
    attachmentsBody:
      "{field} n'est présent que lorsqu'un message contient un média (aujourd'hui : images, GIF et vidéos envoyés sur Discord, et aperçus de liens Tenor/Giphy). Chaque {attachment} :",
    badgeBody: 'Chaque {badge} a la forme {shape}, par exemple {example}.',
    exampleHeading: 'Exemple',
    eventsHeading: 'Événements',
    eventsBody:
      "Les événements de plateforme (abonnements, bits, raids, dons, …) arrivent sous forme de trames {chat} dont {data} contient un objet {event}. Un message de chat normal n'a pas de champ {event}.",
    eventTypesHeading: "Types d'événements par plateforme",
    eventTypesLabel: '{platform} :',
    exampleSubscriptionHeading: 'Exemple (abonnement Twitch)',
    statusHeading: 'Messages de statut et de contrôle',
    statusBody:
      "À la connexion, le serveur envoie une trame {connected}, puis une trame {status} par source configurée, afin que votre interface puisse afficher les indicateurs immédiatement. {status} n'est envoyé que sur le WebSocket de l'overlay : le chemin anonyme des spectateurs ne le reçoit jamais. Données de {status} :",
    reconnectHeading: 'Reconnexion et heartbeats',
    reconnectPing:
      'Le serveur envoie régulièrement {ping}. Répondez par {pong}. (La plupart des bibliothèques WebSocket répondent aussi automatiquement aux trames ping de bas niveau.)',
    reconnectBackfill:
      'Si le socket se ferme, reconnectez-vous avec un délai croissant. Pour récupérer les messages manqués pendant une brève coupure, reconnectez-vous avec {since} réglé juste avant la perte de connexion : le serveur rejoue les messages en mémoire tampon plus récents que cet horodatage.',
    reconnectDedup:
      'Utilisez les ID comme clé de déduplication lorsque vous combinez le tampon de relecture avec les messages en direct.',
    tryItHeading: 'Essayer maintenant (sans configuration)',
    tryItBody:
      "Un overlay de test public diffuse un trafic factice réaliste (chat aléatoire, votes de sondage, soit littéralement les messages {one}, {two}, {three}, {four}, et événements de plateforme) pour que vous puissiez créer et valider une intégration sans compte ni vraie chaîne. Il suffit de se connecter : le trafic circule tant qu'un client est connecté et s'arrête quand le dernier se déconnecte :",
    tryItOutro:
      "Collez cette URL dans l'un des exemples ci-dessus : vous devriez voir aussitôt arriver des messages et des événements.",
    footerCopyright: '© {year} All-Chat',
    footerGuideLink: 'Guide du streamer',
    footerPrivacyLink: 'Politique de confidentialité',
    footerTermsLink: "Conditions d'utilisation",
  },
  apiFields: {
    queryParamsToken:
      'Token du propriétaire. Nécessaire uniquement pour un accès réservé au propriétaire ; à omettre pour une lecture seule.',
    queryParamsSince:
      "Millisecondes depuis l'époque Unix. À la connexion, le serveur rejoue les messages en mémoire tampon plus récents que cet horodatage, afin qu'un client qui se reconnecte puisse combler le trou. À omettre pour démarrer en direct.",
    envelopeType: 'Type de message (voir ci-dessous).',
    envelopeData: 'Charge utile ; sa forme dépend du type. Omise pour ping/pong.',
    envelopeTimestamp: 'Horodatage RFC 3339 du moment où la passerelle a envoyé la trame.',
    messageTypesChatMessage:
      "Un message de chat ou un événement de plateforme. data est l'objet message unifié.",
    messageTypesMessageUpdate:
      "Mise à jour d'un message déjà envoyé : un agrégat de likes TikTok (identifié par event.aggregation_id) ou un message de chat dont le média est arrivé en retard, par exemple l'aperçu d'un lien GIF Discord (identifié par id ; à ignorer si ce message n'est plus affiché). Même forme de data que chat_message.",
    messageTypesConnected: 'Envoyé une fois à la connexion : { overlay_id, message }.',
    messageTypesPlatformStatus: "État de connexion d'une plateforme source.",
    messageTypesPing: 'Heartbeat du serveur. Répondez par { "type": "pong" }.',
    messageTypesError: "Avis d'erreur : { code, message }.",
    chatMessageId: 'ID unique du message.',
    chatMessageOverlayId: 'Overlay auquel ce message a été remis.',
    chatMessagePlatform: '"twitch" | "youtube" | "kick" | "tiktok" | "discord".',
    chatMessageChannelId: 'Identifiant de la chaîne sur la plateforme.',
    chatMessageChannelName: 'Nom lisible de la chaîne.',
    chatMessageUser: "Informations sur l'auteur (voir ci-dessous).",
    chatMessageMessage: '{ text, emotes[], attachments[]? } (voir ci-dessous).',
    chatMessageTimestamp: 'Heure du message au format RFC 3339 (UTC).',
    chatMessageMetadata: 'Données supplémentaires libres, propres à la plateforme.',
    chatMessageEvent:
      'Présent uniquement lorsque le message est un événement de plateforme (voir Événements). Absent pour un message de chat normal.',
    userId: "ID de l'utilisateur sur la plateforme.",
    userUsername: 'Identifiant de connexion ou pseudo.',
    userDisplayName: "Nom d'affichage.",
    userColor: 'Couleur du nom, en hexadécimal (par exemple "#FF0000").',
    userBadges: "Badges de l'auteur (voir ci-dessous).",
    userAvatarUrl: "URL de l'image de profil, si disponible.",
    userPronouns: 'Par exemple "she/her", si connus.',
    userNameGradient: 'Descripteur de dégradé facultatif (chaîne JSON).',
    userSourceBadgesSourceUserId:
      "Identité dans la chaîne d'origine pour les messages de chat partagé.",
    userAvatarFrameUrlAvatarFlairUrl: 'Cadre ou flair cosmétique, si défini.',
    emoteCode: 'Code textuel de l\'emote, par exemple "Kappa".',
    emoteProvider: '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | plateforme.',
    emoteUrl: "URL de l'image sur le CDN.",
    emotePositions:
      "Tableau de paires d'index [start, end] dans text indiquant où l'emote apparaît.",
    attachmentType: '"image" ou "video". Les GIF sont des images animées.',
    attachmentUrl: 'URL du média.',
    attachmentContentType: 'Type MIME, par exemple "image/gif", "video/mp4".',
    attachmentWidthHeight: 'Dimensions intrinsèques en pixels, si connues.',
    attachmentThumbUrl: "Image d'aperçu pour les vidéos, si disponible.",
    attachmentSpoiler: "True lorsque l'expéditeur a marqué le média comme spoiler.",
    attachmentFilename: "Nom de fichier d'origine (utilisé pour le texte alternatif).",
    eventType: "Type d'événement (voir la liste ci-dessous).",
    eventTier: 'Importance relative : "high" | "medium" | "low".',
    eventValue:
      '{ amount: number, currency: string, display_text: string }, par exemple amount 250, currency "bits", display_text "250 bits".',
    eventDuration: "Durée d'affichage suggérée à l'écran, en secondes.",
    eventIsUpdate:
      'true lorsque cet événement en met à jour un précédent (par exemple les agrégats de likes TikTok, envoyés en message_update).',
    eventAggregationId: "Regroupe les mises à jour successives d'un même agrégat.",
    eventMetadata: "Champs bruts propres à l'événement.",
    platformStatusPlatform: 'Plateforme source.',
    platformStatusChannelId: 'Chaîne source.',
    platformStatusChannelName: 'Nom lisible de la chaîne.',
    platformStatusStatus: '"connected" | "reconnecting" | "offline" | "quota_exceeded".',
    platformStatusNextRetryAt:
      'Heure RFC 3339 de la prochaine tentative de reconnexion, le cas échéant.',
    platformStatusErrorMessage: 'Détail lisible en cas de fonctionnement dégradé.',
  },
  // Product-surface names follow the UI; selectors and chat commands are not copy.
  guide: {
    eyebrow: 'Docs All-Chat',
    heading: 'Guide du streamer',
    intro:
      'Installez-le dans OBS, gérez sondages, prédictions et modération depuis le moniteur de chat, et personnalisez-le à votre goût.',
    apiPrompt: "Vous créez un bot, une boîte d'alertes ou un outil de statistiques ? {api}",
    apiLinkText: "Voir l'API développeur →",
    tocHeading: 'Sur cette page',
    tocWhatIsAllChat: "Qu'est-ce qu'All-Chat",
    tocGettingStarted: 'Mettre votre overlay en ligne',
    tocIrl: 'Streams 24/7 et IRL',
    tocTiktok: 'TikTok : fonctionnement et limites',
    tocMonitor: 'Le moniteur de chat',
    tocModeration: 'Modérer votre chat',
    tocEngagement: 'Sondages, prédictions et points',
    tocEventsCredits: 'Événements et générique',
    tocAlerts: "Overlays d'alertes",
    tocCommands: 'Commandes de chat et minuteurs',
    tocSharing: 'Partager un overlay',
    tocThemes: 'Choisir un thème',
    tocCustomize: 'Le personnaliser',
    tocCustomCss: 'Aller plus loin avec le CSS',
    tocFonts: 'Polices personnalisées',
    tocPremium: 'Premium',
    whatIsHeading: "Qu'est-ce qu'All-Chat",
    whatIsBody:
      "All-Chat réunit votre chat en direct de {twitch}, {youtube}, {kick}, {tiktok} et {discord} dans un seul overlay à déposer dans OBS. Chaque message arrive dans un flux unique, avec les événements comme les abonnements, les bits, les raids, les Super Chats et les cadeaux, et les emotes 7TV, BTTV et FFZ s'affichent automatiquement. Aucun bot à inviter, aucun widget de chat à brancher.",
    startHeading: 'Mettre votre overlay en ligne',
    startSignIn: 'Connectez-vous sur {home} avec votre compte Twitch, YouTube ou Kick.',
    startSignInLinkText: 'allch.at',
    startCreate:
      'Dans le tableau de bord, créez un {overlay} et connectez les plateformes sur lesquelles vous streamez. Chaque plateforme connectée devient une {source} de cet overlay : combinez-en autant que vous voulez.',
    startCreateOverlayEmphasis: 'overlay',
    startCreateSourceEmphasis: 'source de chat',
    startObs:
      "Copiez l'URL de source navigateur de l'overlay et ajoutez-la à OBS comme {browserSource}. Le chat apparaît dès que l'overlay se connecte.",
    startObsEmphasis: 'Source navigateur',
    startDemandDriven:
      "Les sources fonctionnent à la demande : All-Chat commence à écouter quand l'overlay est ouvert et s'arrête quand plus rien n'est connecté. Rien ne tourne donc à vide.",
    startMultipleOverlays:
      'Un même compte peut gérer plusieurs overlays, chacun avec sa propre combinaison de sources de chat, son thème et son URL OBS. Par exemple, un overlay Twitch et TikTok pour un stream gaming, et un autre uniquement YouTube pour un podcast.',
    irlHeading: 'Streams 24/7 et IRL',
    irlIntro:
      "Faire tourner une instance OBS en continu, une configuration courante chez les streamers IRL qui veulent se protéger des déconnexions, demande une étape de plus pour que le chat YouTube se comporte bien. Ajoutez {passiveParam} à l'URL de source navigateur de votre overlay :",
    irlExplainer:
      "Un overlay {passive} affiche le chat exactement comme un overlay normal, mais il ne demande pas de lui-même à All-Chat de lancer la capture. C'est important, car la détection YouTube abandonne après une heure sans trouver de live (elle ne bombarde donc jamais YouTube pour une chaîne hors ligne). Un overlay 24/7 normal déclencherait ce délai pendant que vous êtes hors ligne et serait mis en attente au moment où vous passez en live ; un overlay passif ne lance jamais ce compte à rebours, donc rien ne reste bloqué.",
    irlExplainerEmphasis: 'passif',
    irlWhenLiveHeading: 'Quand vous passez en live',
    irlStepPassiveUrl: "Laissez votre source navigateur OBS 24/7 sur l'URL passive.",
    irlStepOpenMonitor: 'Ouvrez le {monitor} de votre overlay (la page moniteur / vue).',
    irlStepOpenMonitorEmphasis: 'moniteur de chat',
    irlStepRediscover:
      "Si le chat n'arrive pas déjà, cliquez sur {rediscover} dans le moniteur : la capture démarre en une minute environ.",
    irlStepRediscoverEmphasis: 'Redétecter',
    irlStepKeepOpen:
      "Gardez le moniteur de chat ouvert pendant votre stream : cela maintient la capture pour la session, et elle s'arrête quelques minutes après sa fermeture.",
    irlRefreshNote:
      "Une simple actualisation de la source navigateur ne relance {negation} une chaîne YouTube en attente : utilisez le bouton {rediscover} du moniteur. Tant qu'une chaîne est en attente, son point de plateforme affiche un état {paused} (elle attend que vous la déclenchiez), et non une erreur rouge.",
    irlRefreshNoteNegationEmphasis: 'pas',
    irlRefreshNotePausedEmphasis: 'indigo « en pause »',
    tiktokHeading: 'TikTok : fonctionnement et limites',
    tiktokNoApi:
      "TikTok n'a pas d'API publique pour le chat LIVE. Tous les outils qui affichent le chat TikTok LIVE, All-Chat compris, le lisent comme le fait le lecteur web de TikTok. Ajoutez une source TikTok par nom d'utilisateur, et le chat apparaît lorsque vous êtes en LIVE.",
    tiktokSigner:
      "Cette connexion doit être signée. All-Chat la signe avec son propre service de signature open source ; aucun serveur de signature tiers n'intervient. Lorsque TikTok est passé à des vérifications de session de niveau navigateur le 2026-09-09, All-Chat a suivi le changement dans la semaine. La conception et ses mises à jour sont décrites dans l'{adr}.",
    tiktokAdrLinkText: 'ADR-0052 sur GitHub',
    tiktokFallback:
      'Les salons premium disposent d’un second chemin de réception, utilisé lorsque TikTok refuse la connexion principale.',
    tiktokLimit:
      "{limit} TikTok est la plateforme la plus susceptible de connaître de courtes interruptions lorsque TikTok modifie son lecteur web. TikTok n'a pas non plus d'API d'envoi ni de modération : le moniteur ne peut donc ni y répondre ni y modérer.",
    tiktokLimitEmphasis: 'La limite :',
    monitorHeading: 'Le moniteur de chat',
    monitorIntro:
      "Ouvrez la {monitorView} depuis l'éditeur d'overlay pour disposer d'une régie en direct, distincte de l'overlay OBS. Elle affiche tous les messages dans un seul panneau, avec un fil d'activité à côté, et c'est là que vous envoyez des messages et modérez.",
    monitorIntroEmphasis: 'Vue moniteur',
    monitorSend:
      "{send} en votre nom sur Twitch, YouTube ou Kick directement depuis le moniteur (TikTok et Discord n'ont pas d'API d'envoi).",
    monitorSendEmphasis: 'Envoyez des messages',
    monitorRediscover:
      "{rediscover} force une nouvelle recherche de votre live : utilisez-le si le chat YouTube s'arrête après un plantage ou un redémarrage, ou pour lancer la capture d'un {passiveOverlay} quand vous passez en live.",
    monitorRediscoverEmphasis: 'Redétecter YouTube',
    monitorPassiveOverlayEmphasis: 'overlay passif 24/7',
    monitorDisplay:
      "Les paramètres {display} déterminent ce que vous voyez (avatars, badges, pronoms, horodatages, icônes de plateforme, contrôles de modération). Ce sont vos préférences personnelles pour le moniteur ; elles ne modifient pas l'overlay OBS.",
    monitorDisplayEmphasis: 'Affichage',
    monitorDockHeading: 'Ancrer le moniteur dans OBS',
    monitorDockIntro:
      "Le moniteur fonctionne aussi comme dock de navigateur personnalisé dans OBS : le chat se place à côté de votre mixeur plutôt que dans une seconde fenêtre. Dans l'éditeur d'overlay, cliquez sur {copyDock}. Le lien du moniteur est copié avec le mode dock activé :",
    monitorDockSteps:
      'Dans OBS, ouvrez {menu}, donnez un nom au dock, collez le lien et cliquez sur Appliquer. Le dock garde son propre profil de navigateur : il vous demande donc de vous connecter une fois dans le panneau. Streamlabs Desktop accepte le même lien comme dock de navigateur personnalisé.',
    monitorDockMenu: 'Affichage > Docks > Docks de navigateur personnalisés',
    moderationHeading: 'Modérer votre chat',
    moderationBody:
      "Avec les {controls} activés dans le moniteur, survolez un message pour afficher {del}, ou ouvrez un utilisateur du chat pour choisir {timeout}, {ban} ou {unban}. L'action s'applique sur la plateforme d'origine : Twitch, Kick et Discord gèrent la suppression, l'exclusion temporaire, le bannissement et le débannissement ; YouTube gère l'exclusion temporaire et le bannissement ; TikTok n'a pas d'API de modération.",
    moderationControlsEmphasis: 'Contrôles de modération',
    moderationDeleteEmphasis: 'Supprimer',
    moderationTimeoutEmphasis: 'Exclusion temporaire',
    moderationBanEmphasis: 'Bannir',
    moderationUnbanEmphasis: 'Débannir',
    moderationEnable:
      'La première fois, utilisez {enable} pour accorder les autorisations supplémentaires (pour Discord, vous réinvitez le bot). Modérer depuis votre overlay est une fonctionnalité {premium}.',
    moderationEnableEmphasis: "Activer la modération et l'envoi de messages",
    engagementHeading: 'Sondages, prédictions et points',
    engagementIntro:
      'All-Chat a ses propres {polls}, {predictions} et {points}, qui fonctionnent sur toutes les plateformes connectées, pas seulement sur Twitch.',
    engagementPollsEmphasis: 'sondages',
    engagementPredictionsEmphasis: 'prédictions',
    engagementPointsEmphasis: 'points de spectateur',
    engagementSetup:
      "Activez-les dans la section {section} de l'éditeur : {enablePoints}, définissez votre {pointsName} et choisissez comment les points se gagnent (par bit, par abonnement, par abonnement offert, etc.).",
    engagementSectionEmphasis: 'Engagement',
    engagementEnablePointsEmphasis: 'Activer les points des spectateurs',
    engagementPointsNameEmphasis: 'Nom des points',
    engagementRun:
      'Lancez les manches en direct depuis le panneau {section} du moniteur : {startPoll} / {closePoll}, et {startPrediction} / {lockWagers} / payer ou {cancelRefund}.',
    engagementStartPollEmphasis: 'Lancer le sondage',
    engagementClosePollEmphasis: 'Clôturer le sondage',
    engagementStartPredictionEmphasis: 'Lancer la prédiction',
    engagementLockWagersEmphasis: 'Verrouiller les mises',
    engagementCancelRefundEmphasis: 'Annuler et rembourser',
    engagementJoin:
      'Les spectateurs participent depuis le chat (par exemple {voteCommand}, {predictCommand}) ou sur la {participationPage}.',
    engagementParticipationPageEmphasis: 'Page de participation des spectateurs',
    engagementWidgets:
      "Ajoutez le {pollWidget} et le {predictionWidget} comme sources navigateur distinctes pour afficher les résultats à l'écran : copiez leurs URL depuis la section Engagement.",
    engagementPollWidgetEmphasis: 'Widget de sondage OBS',
    engagementPredictionWidgetEmphasis: 'Widget de prédiction OBS',
    eventsHeading: 'Événements et générique',
    eventsBody:
      "Votre overlay affiche aussi les {events} : abonnements, réabonnements, abonnements offerts, bits/cheers, raids, Super Chats, souscriptions et follows. Utilisez les {eventSettings} dans l'éditeur pour choisir précisément quels événements apparaissent, plateforme par plateforme.",
    eventsEmphasis: 'événements',
    eventSettingsEmphasis: 'Paramètres des événements',
    creditsBody:
      "Pour la fin d'un stream, ouvrez {credits} pour configurer un {creditRoll}, un remerciement défilant de vos meilleurs abonnés, donateurs, cheerers, raiders et nouveaux followers. C'est une source navigateur à part entière ({copyUrl}).",
    creditsEmphasis: 'Générique',
    creditRollEmphasis: 'Générique de fin',
    creditsCopyUrlEmphasis: "Copier l'URL OBS du générique",
    alertsHeading: "Overlays d'alertes",
    alertsBody:
      "Créez un overlay de type {kind} pour obtenir une boîte d'alertes dans sa propre source navigateur OBS. Les abonnements, Bits, raids et follows Twitch, les Super Chats et souscriptions YouTube et les cadeaux TikTok s'affichent avec votre texte, votre GIF et votre son, ou avec votre propre HTML, CSS et JavaScript. Les overlays d'alertes sont gratuits ; l'envoi de vos propres médias est premium. Le {guide} détaille la mise en place.",
    alertsKindEmphasis: 'Alertes',
    alertsGuideLinkText: 'guide des alertes de stream',
    commandsHeading: 'Commandes de chat et minuteurs',
    commandsBody:
      "Sous {nav} dans la barre supérieure, vous définissez une seule fois vos !commandes, déclencheurs par mot-clé et minuteurs, et ils répondent dans chaque chat que vos overlays regroupent. Les réponses viennent de l'All-Chat Bot ou de votre propre compte. Elles sont gratuites dans les chats Twitch et Kick et premium dans le chat YouTube. Le {guide} donne les détails.",
    commandsNavEmphasis: 'Commandes',
    commandsGuideLinkText: 'guide du bot de chat',
    sharingHeading: 'Partager un overlay',
    sharingBody:
      "{share} permet à un autre streamer d'intégrer le chat de votre overlay au sien, pratique pour les collabs et les raids. Envoyez une demande à son nom d'utilisateur Twitch ; une fois acceptée, votre overlay apparaît dans son éditeur sous {sharedOverlays} pour être ajouté comme source, et chacun de vous peut révoquer le partage plus tard. Le partage est une fonctionnalité {premium}.",
    sharingShareEmphasis: "Partager l'overlay",
    sharingSharedOverlaysEmphasis: 'Overlays partagés',
    themesHeading: 'Choisir un thème',
    themesIntro:
      "All-Chat est livré avec {count}, de Modern Dark et Minimal à Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk et bien d'autres. En appliquer un se fait en un clic et ne demande {noCss}.",
    themesCountEmphasis: '16 thèmes intégrés',
    themesNoCssEmphasis: 'aucun CSS',
    themesStepOpen: 'Ouvrez votre overlay dans le tableau de bord pour le modifier.',
    themesStepApply:
      "Dans la section {theme}, parcourez les thèmes et appliquez-en un. L'aperçu se met à jour instantanément.",
    themesStepApplyEmphasis: 'Thème',
    themesStepSave:
      'Enregistrez. Votre source navigateur OBS adopte le nouveau style à sa prochaine actualisation.',
    themesPreview:
      'Vous pouvez aussi prévisualiser chaque thème en direct sur la {home} avant de vous connecter.',
    themesPreviewLinkText: "page d'accueil",
    customizeHeading: 'Le personnaliser',
    customizeIntro:
      "Un thème est un point de départ : la plupart du temps, vous obtenez exactement le rendu voulu{noCode}. Dans le panneau {appearance} de l'éditeur d'overlay, vous pouvez régler, avec des curseurs, des interrupteurs et des sélecteurs de couleur :",
    customizeNoCodeEmphasis: ' sans écrire de code',
    customizeAppearanceEmphasis: 'Apparence',
    customizeFont:
      'La police, la taille du texte, la graisse, la hauteur de ligne et l’espacement des lettres',
    customizeSpacing: "L'espacement, les marges intérieures et les coins des bulles de message",
    customizeAvatar: 'La taille des avatars et des badges',
    customizeVisibility:
      "Ce qu'il faut afficher ou masquer : avatars, badges, horodatages, noms d'utilisateur",
    customizeColors: 'Les couleurs, dont une couleur de nom par plateforme',
    customizeEvents:
      "L'apparence des événements comme les abonnements, les raids et les Super Chats",
    customizeOutro:
      "Partez du thème le plus proche de ce que vous voulez, puis ajustez ces réglages jusqu'à ce qu'il vous ressemble. S'il vous faut quelque chose que le panneau ne couvre pas, passez au CSS ci-dessous.",
    cssHeading: 'Aller plus loin avec le CSS',
    cssIntro:
      "Pour un contrôle total, activez {customCss} dans la section {expert} de l'éditeur d'overlay et écrivez vos propres styles. Quelques points à connaître :",
    cssCustomCssEmphasis: 'CSS personnalisé',
    cssExpertEmphasis: 'Expert',
    cssScope: "Votre CSS n'affecte que votre overlay, rien d'autre.",
    cssOrder:
      "Il se charge {after} le thème : vous pouvez donc partir d'un thème intégré et ne remplacer que les parties voulues.",
    cssOrderEmphasis: 'après',
    cssPreview:
      "Un aperçu en direct se trouve juste à côté de l'éditeur : vous voyez les changements à mesure que vous tapez.",
    cssVarsHeading: 'Gains rapides : les variables de style',
    cssVarsIntro:
      "Le levier le plus simple ne demande aucune connaissance des noms de classes. Définissez l'une de ces variables sur {root} et l'overlay les prend en compte :",
    cssVarsForced:
      "Un groupe de variables est particulier : la graisse de la police, les coins des bulles, la hauteur de ligne, l'espacement des lettres, la couleur des messages et la police sont lus depuis ces variables par les styles de l'overlay lui-même. Une valeur définie ici l'emporte donc aussi sur le curseur du panneau d'apparence.",
    cssVarsColumnVariable: 'Variable',
    cssVarsColumnDefault: 'Par défaut',
    cssVarsColumnEffect: "Ce qu'elle modifie",
    cssHooksHeading: 'Contrôle plus fin : cibler les éléments du chat',
    cssHooksIntro:
      "Pour tout ce que les variables ne couvrent pas, stylez directement les éléments de l'overlay. Ces noms de classes et attributs data sont stables et peuvent être ciblés sans risque :",
    cssHooksColumnSelector: 'Sélecteur',
    cssHooksColumnKind: 'Type',
    cssHooksColumnTargets: 'Cible',
    cssFeedAnchor:
      "{feedAnchor} (paramètres Messages) détermine le {edge} sur lequel repose le flux : ancrez-le en bas et les nouveaux messages poussent les anciens vers le haut, l'espace vide se retrouvant en haut. {invertOrder} est un réglage distinct pour un axe distinct : il change seulement quelle {endOfList} accueille les messages les plus récents. Les quatre combinaisons fonctionnent.",
    cssFeedAnchorEmphasis: 'Ancrage du flux',
    cssFeedAnchorEdgeEmphasis: 'bord',
    cssInvertOrderEmphasis: "Inverser l'ordre des messages",
    cssEndOfListEmphasis: 'extrémité de la liste',
    cssExampleCaption: 'Exemple : donner à chaque plateforme sa propre bande de couleur :',
    cssCallout:
      "Envie d'exemples prêts à l'emploi ? Chaque thème intégré est un simple fichier CSS que vous pouvez lire et dont vous pouvez vous inspirer sur {github}. Vous avez déjà un thème venant d'un autre outil, ou vous voulez de l'aide pour en écrire un ? Demandez sur notre {discord}, nous vous aiderons volontiers.",
    cssCalloutGithubLinkText: 'GitHub',
    fontsHeading: 'Polices personnalisées',
    fontsIntro:
      "Vous pouvez charger une Google Font avec un {importRule} classique, puis l'utiliser partout dans votre CSS. Pour protéger la vie privée de vos spectateurs, les polices sont servies par All-Chat plutôt que directement par Google ; seules ces familles sont donc disponibles :",
    fontsFamilies:
      'Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito, Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech Mono, Source Code Pro, Space Grotesk, VT323.',
    fontsOutro:
      'Une famille qui ne figure pas dans la liste ne se chargera tout simplement pas : choisissez-en une autre ou gardez la police par défaut.',
    premiumHeading: 'Premium',
    premiumIntro:
      'All-Chat est gratuit et open source. Un abonnement Premium, via Patreon, connecté dans {settings}, débloque :',
    premiumSettingsEmphasis: 'Paramètres → Premium',
    premiumModeration:
      '{label} : supprimez, excluez temporairement et bannissez depuis le moniteur de chat.',
    premiumModerationEmphasis: 'Modérez depuis votre overlay',
    premiumTts:
      '{label} : des voix TTS premium (la synthèse vocale de base du navigateur est gratuite).',
    premiumTtsEmphasis: 'Synthèse vocale ElevenLabs',
    premiumYoutube:
      '{label} : choisissez quel stream suivre sur les chaînes à plusieurs streams (par défaut, la version gratuite suit le premier trouvé).',
    premiumYoutubeEmphasis: 'Sélection du stream YouTube',
    premiumSharedChat: "{label} : partagez votre overlay avec d'autres streamers.",
    premiumSharedChatEmphasis: 'Chat partagé',
    premiumFlairs: '{label} : des couleurs de nom en dégradé animé.',
    premiumFlairsEmphasis: 'Flairs de spectateur',
    footerCopyright: '© {year} All-Chat',
    footerApiLink: 'API développeur',
    footerPrivacyLink: 'Politique de confidentialité',
    footerTermsLink: "Conditions d'utilisation",
  },
  guideCssVars: {
    chatFontSize: 'Taille du texte des messages.',
    chatFontWeight: 'Graisse du texte des messages.',
    chatFontFamily: 'Police du texte des messages.',
    chatUsernameFontWeight: "Graisse du nom d'utilisateur.",
    chatMessageColor: 'Couleur du texte des messages.',
    chatMessageGap: 'Espace vertical entre les messages.',
    chatAvatarGap: "Espace entre l'avatar et le texte du message.",
    chatBubbleBorderRadius: 'Arrondi de la bulle de message.',
    chatBubblePadding: 'Marge intérieure de chaque message.',
    chatAvatarSize: "Largeur et hauteur de l'avatar.",
    chatUsernameFontSize: "Taille du nom d'utilisateur.",
    chatEmoteScale: 'Multiplicateur de taille des emotes.',
    chatShowAvatars: 'Mettre à none pour masquer les avatars.',
    chatShowBadges: 'Mettre à none pour masquer les badges.',
    chatShowTimestamps: 'Mettre à none pour masquer les horodatages.',
  },
  guideCssHooks: {
    overlayLiveBody: 'Le conteneur de toute la liste des messages.',
    chatMessage: 'Une bulle de message de chat.',
    chatUsername: "Le nom de l'auteur.",
    platformBadge: "L'étiquette ou l'icône de plateforme à côté du nom.",
    eventMessage: 'Une alerte d’abonnement, de raid, de Super Chat ou de cadeau.',
    dataPlatformTwitch: "Les messages d'une plateforme (twitch, youtube, kick, tiktok, discord).",
    dataUsername: "Les messages d'un utilisateur précis.",
    dataFeedAnchorTopBottom:
      "Le conteneur de l'overlay, qui porte le réglage Ancrage du flux. Lisez-le pour vous adapter ; ne remplacez pas le flex-direction du conteneur ni le margin-top de la liste, ce sont eux qui déplacent le flux.",
    dataFeedOrderNewestLastNewestFirst:
      "Le conteneur de l'overlay, qui porte le réglage Inverser l'ordre des messages. Il inverse aussi --msg-enter-dir et --msg-enter-origin, afin que les animations d'entrée arrivent du côté où atterrit le message le plus récent.",
  },
  themeContrast: {
    heading: 'Banc de test du contraste des thèmes',
    intro:
      'Développement uniquement. Affiche tous les thèmes fournis pour le contrôle WCAG du texte des messages. {count} thèmes.',
  },
}
