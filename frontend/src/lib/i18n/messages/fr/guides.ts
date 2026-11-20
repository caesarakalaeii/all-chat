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

export const guides: LocaleNamespace<'guides'> = {
  shared: {
    eyebrow: 'Guide',
    ctaHeading: 'Lancez-vous',
    ctaBody:
      'All-Chat est gratuit. Connectez-vous avec Twitch, YouTube ou Kick, créez un overlay et collez son URL dans OBS.',
    ctaSignIn: 'Se connecter',
    ctaDocs: 'Lire la documentation',
    relatedHeading: 'Autres guides',
    limitsHeading: 'Limites',
    premiumLinkText: 'Premium',
    tiktokGuideLinkText: 'guide TikTok',
    linkObsOverlay: 'Overlay de chat OBS',
    linkObsDock: 'Dock de chat OBS',
    linkTiktok: 'Chat TikTok LIVE dans OBS',
    linkMultistream: 'Chat multistream',
    linkCompare: "Comparaison avec d'autres outils de chat",
  },
  index: {
    metaTitle: 'Guides : overlay de chat OBS, dock de chat, chat TikTok LIVE et chat multistream',
    metaDescription:
      "Guides pas à pas pour All-Chat : afficher le chat fusionné dans OBS, l'ancrer dans OBS, afficher le chat TikTok LIVE, fusionner le chat multistream et comparer les outils de chat.",
    breadcrumb: 'Guides',
    heading: 'Guides',
    intro:
      'Comment afficher dans OBS le chat fusionné de Twitch, YouTube, Kick, TikTok et Discord, et comment All-Chat se situe par rapport aux autres outils de chat.',
  },
  obsOverlay: {
    metaTitle: 'Overlay de chat OBS pour Twitch, YouTube, Kick et TikTok',
    metaDescription:
      "Une seule source navigateur OBS qui affiche ensemble les chats Twitch, YouTube, Kick, TikTok et Discord. L'URL, la taille, le fond transparent et les réglages de la source, étape par étape.",
    breadcrumb: 'Overlay de chat OBS',
    heading: 'Overlay de chat OBS pour Twitch, YouTube, Kick, TikTok et Discord',
    intro:
      "All-Chat fusionne le chat de toutes les plateformes sur lesquelles vous streamez en un seul flux et le sert sous forme de page web. OBS affiche cette page comme source navigateur, il n'y a donc rien à installer.",
    stepsHeading: "Ajouter l'overlay à OBS",
    stepSignIn: 'Connectez-vous sur {home} avec votre compte Twitch, YouTube ou Kick.',
    stepSignInLinkText: 'allch.at',
    stepCreate:
      'Dans le tableau de bord, créez un overlay et connectez les plateformes sur lesquelles vous streamez. Chaque plateforme devient une source de chat de cet overlay.',
    stepCopy: "Copiez l'URL de l'overlay. Elle a la forme {url}.",
    stepAdd:
      "Dans OBS, cliquez sur {plus} sous Sources, choisissez {browser} et collez l'URL dans le champ URL.",
    stepAddPlus: '+',
    stepAddBrowser: 'Navigateur',
    stepSize:
      'Réglez la largeur et la hauteur sur la zone que le chat doit occuper, par exemple 400 × 800. Ne lui donnez pas la taille de tout votre canevas.',
    stepShutdown: 'Décidez si vous cochez {shutdown}. La section ci-dessous explique le compromis.',
    stepShutdownEmphasis: "Désactiver la source quand elle n'est pas visible",
    stepDone: "Cliquez sur OK. Le chat apparaît dès que l'overlay se connecte.",
    backgroundHeading: 'Fond transparent',
    backgroundBody:
      "La page de l'overlay a un fond transparent, seuls les messages s'affichent donc par-dessus votre scène. Vous n'avez besoin ni d'incrustation chroma ni de CSS supplémentaire dans OBS.",
    shutdownHeading: "Désactiver la source quand elle n'est pas visible",
    shutdownBody:
      "All-Chat ne lit le chat de vos plateformes que lorsqu'un overlay est connecté. Si {shutdown} est coché, OBS ferme la page dès que la source n'est plus dans la scène active. Après un court délai de grâce, All-Chat arrête de lire votre chat, puis reprend quand la source redevient visible.",
    shutdownTradeoff:
      "Cochez la case si vous voulez que la capture du chat s'arrête quand l'overlay est masqué. Laissez-la décochée si vous changez souvent de scène et voulez retrouver le flux déjà actif à votre retour.",
    shutdown247:
      "Vous faites tourner OBS en continu pour un stream IRL ou 24h/24 ? Lisez d'abord la section {section} de la documentation. Elle décrit une URL d'overlay passive qui évite que la détection YouTube se mette en pause pendant que vous êtes hors ligne.",
    shutdown247LinkText: '24h/24 et IRL',
    themesHeading: 'Thèmes et CSS personnalisé',
    themesBody:
      "Choisissez l'un des {themes} dans l'éditeur d'overlay. La police, l'espacement, les couleurs et les avatars se règlent dans le panneau Apparence, sans code. Pour un contrôle total, écrivez vos propres styles avec du {css}.",
    themesLinkText: '16 thèmes intégrés',
    cssLinkText: 'CSS personnalisé',
    platformBadges:
      "Chaque message peut afficher un badge indiquant sa plateforme d'origine. Activez-le dans l'éditeur d'overlay, ou stylisez vous-même chaque plateforme avec du CSS personnalisé.",
    emotesHeading: 'Emotes',
    emotesBody:
      "Les emotes 7TV, BTTV et FFZ s'affichent dans l'overlay à côté des emotes natives de Twitch et YouTube. Il n'y a rien à configurer.",
    limitYoutube:
      "Le chat YouTube fonctionne tout seul pour les streams publics. Pour un stream non répertorié, collez son lien dans les paramètres de la source. Le chat réservé aux membres de votre propre chaîne nécessite le mode API officielle premium. Les streams programmés sont pris en compte dès qu'ils passent en direct.",
    limitTiktokReadOnly:
      "Vous ne pouvez ni répondre au chat TikTok ni le modérer depuis All-Chat. TikTok ne fournit d'API pour aucun des deux.",
    limitRetention:
      "Les messages du chat sont supprimés d'All-Chat après environ une heure. L'overlay est un flux en direct, pas une archive.",
    limitTiktok:
      'TikTok est la plateforme la plus susceptible de décrocher brièvement. Le {guide} explique pourquoi.',
  },
  obsDock: {
    metaTitle: 'Dock de chat OBS pour Twitch, YouTube, Kick et TikTok',
    metaDescription:
      'Ancrez le chat fusionné de Twitch, YouTube, Kick, TikTok et Discord dans OBS via Affichage > Docks > Docks de navigateur personnalisés. Lisez, répondez, modérez et lancez des sondages depuis un seul panneau.',
    breadcrumb: 'Dock de chat OBS',
    heading: 'Dock de chat OBS pour Twitch, YouTube, Kick, TikTok et Discord',
    intro:
      'Chaque overlay All-Chat est fourni avec un moniteur de chat : les mêmes messages, mis en page pour que vous les lisiez plutôt que pour vos spectateurs. OBS peut garder le moniteur sous forme de dock à côté de Scènes et Sources, le chat reste ainsi dans la fenêtre depuis laquelle vous streamez.',
    urlHeading: "L'URL du dock",
    urlBody:
      "Le lien de dock d'un overlay est {url} : le moniteur mis en page pour un panneau étroit. Dans l'éditeur d'overlay, cliquez sur Copier l'URL du dock pour le copier. Sans ?dock=1, la même adresse ouvre le moniteur en pleine largeur dans un onglet du navigateur.",
    stepsHeading: 'Ajouter le moniteur comme dock OBS',
    stepMenu: 'Dans OBS, ouvrez {menu}.',
    stepMenuPath: 'Affichage > Docks > Docks de navigateur personnalisés',
    stepPaste:
      "Saisissez un nom pour le dock, collez l'URL du dock dans le champ URL, puis cliquez sur Appliquer et fermez la fenêtre.",
    stepSignIn:
      "Connectez-vous dans le dock. Les docks OBS utilisent leur propre profil de navigateur, une connexion dans votre navigateur habituel n'y est donc pas reprise. Vous ne le faites qu'une seule fois.",
    stepPlace:
      'Faites glisser le panneau où vous le souhaitez. Une colonne étroite à côté de Scènes et Sources convient bien.',
    streamlabsNote:
      "Streamlabs Desktop propose lui aussi des docks de navigateur personnalisés, et l'URL du dock y fonctionne de la même façon.",
    featuresHeading: 'Ce que vous pouvez faire depuis le dock',
    featureRead: "Lire toutes les plateformes dans une seule liste, avec un fil d'activité à côté.",
    featureSend:
      "Envoyer des messages en votre nom sur Twitch, YouTube ou Kick. TikTok et Discord n'ont pas d'API d'envoi, ils sont donc en lecture seule ici.",
    featureModerate:
      "Supprimer, exclure temporairement, bannir et débannir sur la plateforme d'où vient le message : les quatre actions sur Twitch, Kick et Discord, l'exclusion temporaire et le bannissement sur YouTube. TikTok n'a pas d'API de modération. Modérer depuis All-Chat est une fonctionnalité {premium}.",
    featureEngage:
      'Lancer et clôturer des sondages, et lancer, verrouiller et régler des prédictions. Ils fonctionnent sur toutes les plateformes connectées, et les spectateurs participent depuis le chat.',
    featureMods:
      "Confier le moniteur à vos modérateurs. Ils agissent avec leurs propres comptes de plateforme et n'ont pas besoin de Premium eux-mêmes. L'ajout de modérateurs nécessite {premium} sur votre compte.",
    sameFeedHeading: "Le même flux que l'overlay",
    sameFeedBody:
      "Le dock et l'overlay OBS affichent le même flux. Les réglages d'affichage du moniteur, comme les avatars, les horodatages et les icônes de plateforme, ne changent que votre vue, jamais ce que les spectateurs voient à l'écran.",
    limitPermissions:
      "La première fois que vous envoyez un message ou modérez, le moniteur vous demande d'accorder des autorisations supplémentaires avec {enable}. Pour Discord, cela implique d'inviter à nouveau le bot.",
    limitPermissionsEmphasis: "Activer la modération et l'envoi de messages",
    limitTiktok:
      'Le chat TikTok dans le dock est en lecture seule, et TikTok est la plateforme la plus susceptible de décrocher brièvement. Le {guide} donne les détails.',
    moreDocs:
      'Les sections {monitor}, {moderation} et {engagement} de la documentation vont plus loin.',
    moreDocsMonitor: 'moniteur de chat',
    moreDocsModeration: 'modération',
    moreDocsEngagement: 'sondages et prédictions',
  },
  tiktok: {
    metaTitle: 'Overlay de chat TikTok LIVE pour OBS',
    metaDescription:
      'Affichez le chat TikTok LIVE dans OBS à côté des chats Twitch, YouTube, Kick et Discord, avec un objectif de disponibilité de 99,9 % sans API TikTok officielle.',
    breadcrumb: 'Overlay de chat TikTok LIVE',
    heading: 'Overlay de chat TikTok LIVE pour OBS',
    intro:
      'All-Chat affiche le chat TikTok LIVE dans le même overlay OBS que vos chats Twitch, YouTube, Kick et Discord. TikTok est la plateforme la plus difficile à prendre en charge et la moins stable de celles auxquelles All-Chat se connecte, et le chat TikTok vise pourtant le même objectif de disponibilité de 99,9 % que toutes les autres plateformes. Cette page explique comment.',
    stepsHeading: 'Ajouter TikTok à un overlay',
    stepOpen: 'Ouvrez votre overlay dans le tableau de bord All-Chat.',
    stepConnect:
      "Cliquez sur {connect} et saisissez le nom d'utilisateur TikTok du créateur. Il n'y a pas d'étape de connexion à TikTok.",
    stepConnectEmphasis: 'Connecter TikTok',
    stepLive:
      "Passez en live sur TikTok. Le chat apparaît dans l'overlay dès qu'All-Chat voit que le compte est en live.",
    stepObs: "Si l'overlay n'est pas encore dans OBS, suivez le {guide}.",
    stepObsLinkText: "guide de l'overlay de chat OBS",
    apiHeading: "Aucun outil ne dispose d'une API officielle pour le chat TikTok LIVE",
    apiBody:
      "TikTok n'a pas d'API publique pour le chat LIVE. Tous les outils qui affichent le chat TikTok LIVE, All-Chat compris, le lisent comme le fait le lecteur web de TikTok lui-même.",
    approachHeading: 'Ce que fait All-Chat',
    approachSigner:
      "All-Chat signe les connexions TikTok avec son propre {signer} ; aucun serveur de signature tiers n'intervient. La décision et son historique figurent dans {adr}.",
    approachSignerLinkText: 'service de signature open source',
    approachAdrLinkText: 'ADR-0052',
    approachSessions:
      'Le 9 septembre 2026, TikTok a commencé à exiger des sessions de niveau navigateur pour les données du chat en direct. All-Chat a suivi le changement la même semaine ; les mises à jour sont consignées dans le même ADR.',
    approachFallback:
      "Les salons des comptes {premium} bénéficient d'une seconde voie de livraison lorsque TikTok refuse la connexion principale.",
    approachStory: "L'histoire complète est racontée dans {post}.",
    limitInterruptions:
      "TikTok reste la plateforme la moins stable prise en charge par All-Chat. Quand TikTok modifie son lecteur web, tous les outils qui lisent le chat TikTok LIVE doivent s'adapter, All-Chat compris, de courtes interruptions sont donc possibles. Les chats Twitch, YouTube, Kick et Discord du même overlay continuent de fonctionner pendant ce temps.",
    limitReadOnly:
      "Vous ne pouvez ni répondre au chat TikTok ni le modérer depuis All-Chat. TikTok ne fournit d'API pour aucun des deux.",
    verticalNote:
      'Vous streamez en vertical sur TikTok et en horizontal ailleurs ? Le {guide} décrit un second overlay pour le canevas vertical.',
    verticalNoteLinkText: 'guide du chat multistream',
  },
  multistream: {
    metaTitle: 'Chat multistream : Twitch, YouTube, Kick et TikTok dans un seul flux',
    metaDescription:
      'Fusionnez le chat de toutes les plateformes de votre multistream dans un seul overlay OBS et un seul panneau de modération. Fonctionne avec SE.Live, Restream, Streamlabs Multistream ou plusieurs sorties RTMP.',
    breadcrumb: 'Chat multistream',
    heading: 'Chat multistream : un seul flux pour Twitch, YouTube, Kick, TikTok et Discord',
    intro:
      'Streamez sur quatre plateformes à la fois et le chat arrive à quatre endroits. All-Chat le fusionne, Discord compris, en un seul flux pour votre overlay OBS et un seul panneau pour lire et modérer.',
    methodHeading: "Fonctionne avec n'importe quelle configuration multistream",
    methodBody:
      "All-Chat se connecte directement au chat de chaque plateforme et ne touche jamais à votre vidéo. La façon dont la vidéo est diffusée n'a pas d'importance : SE.Live, Restream, Streamlabs Multistream, plusieurs sorties RTMP depuis OBS ou un second PC fonctionnent tous de la même manière.",
    methodSources:
      "Le chat ne dépend pas de l'envoi de votre stream par un service particulier. Connectez chaque plateforme comme source de chat de votre overlay et le flux suit.",
    platformsHeading: 'Distinguer les plateformes',
    platformsBody:
      "Chaque message peut afficher un badge indiquant sa plateforme d'origine, vous et vos spectateurs voyez ainsi d'un coup d'œil s'il vient de Kick ou de YouTube. Activez les badges dans l'éditeur d'overlay. Avec du {css}, vous pouvez donner à chaque plateforme sa propre couleur, car chaque message est étiqueté avec sa plateforme.",
    platformsCssLinkText: 'CSS personnalisé',
    moderationHeading: 'Un seul endroit pour modérer',
    moderationBody:
      "Le moniteur de chat liste toutes les plateformes ensemble. Depuis celui-ci, vous pouvez supprimer, exclure temporairement, bannir et débannir sur Twitch, Kick et Discord, et exclure temporairement et bannir sur YouTube, chaque action s'appliquant sur la plateforme d'où vient le message. TikTok n'a pas d'API de modération. Modérer depuis All-Chat est une fonctionnalité {premium}, et vous pouvez confier le moniteur à vos modérateurs.",
    moderationDock: 'Gardez le moniteur dans OBS grâce au {guide}.',
    moderationDockLinkText: 'guide du dock de chat OBS',
    engagementHeading: 'Sondages et prédictions sur toutes les plateformes',
    engagementBody:
      "All-Chat gère ses propres sondages, prédictions et points de spectateur. Les spectateurs de toutes les plateformes connectées participent depuis le chat, un sondage n'est donc pas limité à votre audience Twitch.",
    overlaysHeading: 'Des overlays distincts pour des canevas distincts',
    overlaysBody:
      "Un compte peut avoir plusieurs overlays, chacun avec ses propres sources de chat et son thème. Une configuration courante : un overlay avec toutes les plateformes pour le canevas horizontal, et un second overlay plus étroit pour un canevas vertical, par exemple un overlay qui n'affiche que le chat TikTok.",
    limitTiktok:
      'TikTok est la plateforme la plus susceptible de décrocher brièvement. Le {guide} explique pourquoi.',
    limitYoutube:
      "Le chat YouTube fonctionne tout seul pour les streams publics. Pour un stream non répertorié, collez son lien dans les paramètres de la source. Le chat réservé aux membres de votre propre chaîne nécessite le mode API officielle premium. Les streams programmés sont pris en compte dès qu'ils passent en direct.",
    compareNote:
      "Vous hésitez entre plusieurs outils ? La {compare} liste ce que proposent Streamlabs, SE.Live, Social Stream Ninja, SleepyChat et Restream Chat, sources à l'appui.",
    compareNoteLinkText: 'page de comparaison',
  },
  compare: {
    metaTitle: 'Comparatif des outils de chat multistream',
    metaDescription:
      'All-Chat face à Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat et Restream Chat : plateformes, installation, overlay et dock OBS, emotes, prix, open source, alertes et TTS, avec une source pour chaque cellule.',
    breadcrumb: 'Comparaison',
    heading: 'All-Chat comparé aux autres outils de chat multistream',
    intro:
      "Ce tableau place All-Chat à côté de cinq autres outils qui fusionnent le chat de plusieurs plateformes. Nous développons All-Chat, gardez-le à l'esprit en le lisant. Chaque affirmation sur un autre outil renvoie aux pages de cet outil, et là où nous n'avons trouvé aucune indication, nous avons écrit « non documenté » au lieu de deviner.",
    lastChecked: 'Dernière vérification : {date}',
    caption:
      'Outils de chat multistream par fonctionnalité, dernière vérification le {date}. Les liens numérotés renvoient aux sources listées sous le tableau.',
    colTool: 'Outil',
    colPlatforms: 'Chat Twitch, YouTube, Kick, TikTok',
    colInstall: 'Installation requise',
    colOverlay: 'Overlay OBS',
    colDock: 'Dock OBS',
    colEmotes: '7TV, BTTV, FFZ',
    colPrice: 'Prix du chat fusionné',
    colOpenSource: 'Open source',
    colSelfHost: 'Auto-hébergeable',
    colAlerts: 'Alertes',
    colTts: 'TTS',
    notDocumented: 'Non documenté',
    yes: 'Oui',
    allThree: 'Oui, les trois',
    sourceRefLabel: 'Source {number}',
    toolAllChat: 'All-Chat',
    allChatPlatforms: 'Les quatre, plus Discord',
    allChatInstall: 'Non. Collez une URL dans une source navigateur',
    allChatDock: 'Oui, le moniteur de chat',
    allChatPrice: 'Gratuit. Extras Premium via Patreon',
    allChatOpenSource: 'Oui, AGPL-3.0',
    allChatSelfHost: 'Oui, avec Docker Compose',
    allChatAlerts: 'En développement',
    allChatTts: 'Oui. Voix du navigateur gratuites, ElevenLabs avec Premium et votre propre clé',
    toolStreamlabs: 'Streamlabs Chat Box et Multichat',
    streamlabsPlatforms:
      "Twitch, YouTube, Kick. TikTok ne figure pas dans la liste des plateformes de la Chat Box ; un article d'aide indique que le chat TikTok s'ouvre dans une fenêtre de navigateur séparée",
    streamlabsInstall:
      'Non pour la Chat Box, qui est une URL de source navigateur. Multichat est fourni avec Streamlabs Multistream',
    streamlabsOverlay: 'Oui, la Chat Box',
    streamlabsDock: "Multichat dans Streamlabs Desktop. Un dock OBS n'est pas documenté",
    streamlabsPrice:
      'Chat Box gratuite. Multistream est une fonctionnalité Ultra ; Dual Output, gratuit, couvre une destination horizontale et une verticale',
    streamlabsOpenSource: 'Streamlabs Desktop est sous GPL-3.0. Widgets : non documenté',
    streamlabsAlerts: 'Oui, Alert Box',
    streamlabsTts: "Oui, lecture à voix haute des dons et des Bits dans l'Alert Box",
    toolSeLive: 'StreamElements SE.Live',
    seLivePlatforms:
      'Multi-Chat affiche toutes les plateformes connectées, et Twitch, YouTube et Kick peuvent être connectés. Le chat TikTok a son propre dock, en dehors de Multi-Chat',
    seLiveInstall: 'Oui, un plugin OBS, Windows uniquement',
    seLiveOverlay:
      "Un overlay de chat fusionné n'est pas documenté. En février 2025, l'équipe a indiqué que le widget de chat n'affiche que la plateforme principale",
    seLiveDock: 'Oui, dock de chat et fenêtre Multi-Chat',
    seLivePrice: 'Plugin gratuit',
    seLiveAlerts: "Oui, alertes d'overlay",
    seLiveTts: 'Oui, via les récompenses du Stream Store',
    toolSsn: 'Social Stream Ninja',
    ssnPlatforms: 'Les quatre, plus 100+ autres sites',
    ssnInstall:
      'Oui, une extension de navigateur ou une application de bureau. Une application web Lite a moins de fonctionnalités',
    ssnDock: 'Oui, la page de dock de chat',
    ssnPrice: 'Gratuit. Certaines intégrations optionnelles nécessitent des comptes payants',
    ssnOpenSource: 'Oui, GPL-3.0',
    ssnAlerts: "Oui, overlays d'alertes d'événements",
    ssnTts: 'Oui. Voix du système, plus des fournisseurs payants en option',
    toolSleepyChat: 'SleepyChat',
    sleepyChatPlatforms: 'Les quatre',
    sleepyChatInstall: 'Non. Copiez un lien depuis le tableau de bord web',
    sleepyChatPrice: "Offre gratuite avec limites d'utilisation. Premium les relève",
    sleepyChatOpenSource: 'Non documenté. Le dépôt GitHub public est un suivi de tickets',
    sleepyChatAlerts: 'Oui, alertes animées personnalisées',
    sleepyChatTts: "Oui, une seule file d'attente pour les quatre plateformes",
    toolRestream: 'Restream Chat',
    restreamPlatforms: "Twitch, YouTube, Kick. Le chat TikTok n'est pas lu",
    restreamInstall:
      'Non. Fonctionne dans le navigateur ; une application de bureau est optionnelle',
    restreamOverlay: "Oui, une URL d'intégration comme source navigateur",
    restreamPrice:
      "L'offre gratuite inclut le chat. Les messages n'arrivent que lorsque vous streamez via Restream",
    restreamTts: "Alertes en synthèse vocale dans l'application de chat de bureau",
    sourcesHeading: 'Sources',
    sourceAllChatReadme: 'README All-Chat sur GitHub',
    sourceStreamlabsChatBox: 'Streamlabs : widget Chat Box',
    sourceStreamlabsMultistream: 'Streamlabs : page Multistream et FAQ Multichat',
    sourceStreamlabsWidgets: 'Streamlabs : page des widgets et FAQ',
    sourceStreamlabsTiktok: 'Aide Streamlabs : How to Go Live on TikTok from a Desktop',
    sourceStreamlabsDesktopRepo: 'Code source de Streamlabs Desktop sur GitHub',
    sourceStreamlabsTts: 'Streamlabs : How to Add Text-to-Speech for Tips (and Bits)',
    sourceSeLiveGuide: 'Aide StreamElements : SE.Live, The Complete Guide',
    sourceSeMultichatOverlay:
      "Communauté StreamElements : demande Multichat Overlay et réponse de l'équipe",
    sourceSeAlerts: 'Aide StreamElements : Setting Up Twitch Alerts with StreamElements Overlays',
    sourceSeTts: 'Aide StreamElements : Stream Store, The Complete Guide',
    sourceSsnReadme: 'README Social Stream Ninja sur GitHub',
    sourceSsnFeatures: 'Social Stream Ninja : Features',
    sourceSsnParameters: "Social Stream Ninja : paramètres d'overlay",
    sourceSleepyChatHome: "Page d'accueil de SleepyChat",
    sourceSleepyChatObs: "SleepyChat : guide du dock navigateur et de l'overlay OBS",
    sourceSleepyChatPricing: 'SleepyChat : Pricing',
    sourceSleepyChatRepo: 'Dépôt SleepyChat sur GitHub',
    sourceRestreamRead: 'Aide Restream : Read incoming chat messages',
    sourceRestreamChat: 'Restream : page produit Chat',
    sourceRestreamEncoders: 'Aide Restream : How to use Restream Chat with encoders',
    sourceRestreamEmbed: 'Aide Restream : Embed chat on your streaming software',
    sourceRestreamFree: 'Aide Restream : Can I use Restream for free?',
    betterHeading: 'Quand un autre outil est le meilleur choix',
    betterIntro:
      "Aucun outil de cette liste n'est le meilleur en tout. Choisissez-en un autre si :",
    betterAlerts:
      "Vous avez besoin d'alertes à l'écran dès aujourd'hui. Les alertes All-Chat sont encore en développement, alors que SleepyChat, Streamlabs et StreamElements les proposent déjà.",
    betterTts:
      "Vous voulez une synthèse vocale liée aux dons et aux alertes. Streamlabs lit les dons et les Bits à voix haute dans son Alert Box, et SleepyChat regroupe le TTS des quatre plateformes dans une seule file d'attente à côté de ses alertes.",
    betterPlatforms:
      "Vous streamez sur d'autres plateformes que ces quatre-là. Social Stream Ninja couvre plus de 100 sites, dont Facebook, Instagram, Rumble et Zoom.",
    betterRestream:
      'Vous faites déjà du multistream via Restream et voulez répondre et relayer les messages entre plateformes depuis une seule fenêtre. Restream Chat est intégré à ce fonctionnement.',
    fitHeading: 'Là où All-Chat a sa place',
    fitBody:
      'All-Chat ne nécessite aucune installation, couvre TikTok en plus de Twitch, YouTube, Kick et Discord, affiche les emotes 7TV, BTTV et FFZ, et est open source sous AGPL-3.0. Le chat fusionné est gratuit ; la modération depuis le moniteur de chat et quelques extras sont Premium.',
    corrections:
      "Les produits évoluent. Si une cellule n'est plus à jour, {issue} et nous corrigerons.",
    correctionsLinkText: 'ouvrez un ticket sur GitHub',
  },
}
